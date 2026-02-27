package memcached

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

const memcachedAbsoluteTTLThreshold = 30 * 24 * time.Hour

// Adapter is a lightweight Memcached text-protocol adapter.
type Adapter struct {
	addresses []string
	timeout   time.Duration
	dial      func(ctx context.Context, network, address string) (net.Conn, error)
}

// NewMemcachedAdapter creates a concrete memcached adapter using TCP text protocol.
func NewMemcachedAdapter(addresses []string, timeout time.Duration) (*Adapter, error) {
	if len(addresses) == 0 {
		return nil, errors.New("at least one memcached address is required")
	}
	normalized := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		trimmed := strings.TrimSpace(addr)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, errors.New("at least one non-empty memcached address is required")
	}
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &Adapter{
		addresses: normalized,
		timeout:   timeout,
		dial:      (&net.Dialer{Timeout: timeout}).DialContext,
	}, nil
}

// Get fetches a value by key.
func (c *Adapter) Get(ctx context.Context, key string) ([]byte, error) {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if _, writeErr := io.WriteString(conn, fmt.Sprintf("get %s\r\n", key)); writeErr != nil {
		return nil, writeErr
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "END" {
		return nil, errors.New("not found")
	}
	// VALUE <key> <flags> <bytes>
	parts := strings.Fields(line)
	if len(parts) != 4 || parts[0] != "VALUE" {
		return nil, fmt.Errorf("unexpected memcached response: %s", line)
	}
	size, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid memcached size: %w", err)
	}
	payload := make([]byte, size+2) // include trailing CRLF
	if _, readErr := io.ReadFull(reader, payload); readErr != nil {
		return nil, readErr
	}
	endLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(endLine) != "END" {
		return nil, fmt.Errorf("unexpected memcached terminator: %s", strings.TrimSpace(endLine))
	}
	return payload[:size], nil
}

// Set stores a value with TTL.
func (c *Adapter) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return err
	}
	defer conn.Close()

	exptime := ttlToSeconds(ttl)
	cmd := fmt.Sprintf("set %s 0 %d %d\r\n", key, exptime, len(value))
	if _, writeErr := io.WriteString(conn, cmd); writeErr != nil {
		return writeErr
	}
	if _, writeErr := conn.Write(value); writeErr != nil {
		return writeErr
	}
	if _, writeErr := io.WriteString(conn, "\r\n"); writeErr != nil {
		return writeErr
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "STORED" {
		return fmt.Errorf("memcached set failed: %s", strings.TrimSpace(line))
	}
	return nil
}

// Delete removes a value by key.
func (c *Adapter) Delete(ctx context.Context, key string) error {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, writeErr := io.WriteString(conn, fmt.Sprintf("delete %s\r\n", key)); writeErr != nil {
		return writeErr
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	switch strings.TrimSpace(line) {
	case "DELETED":
		return nil
	case "NOT_FOUND":
		return errors.New("not found")
	default:
		return fmt.Errorf("unexpected memcached delete response: %s", strings.TrimSpace(line))
	}
}

// Touch refreshes TTL for an existing key.
func (c *Adapter) Touch(ctx context.Context, key string, ttl time.Duration) error {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return err
	}
	defer conn.Close()

	exptime := ttlToSeconds(ttl)
	if _, writeErr := io.WriteString(conn, fmt.Sprintf("touch %s %d\r\n", key, exptime)); writeErr != nil {
		return writeErr
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	switch strings.TrimSpace(line) {
	case "TOUCHED":
		return nil
	case "NOT_FOUND":
		return errors.New("not found")
	default:
		return fmt.Errorf("unexpected memcached touch response: %s", strings.TrimSpace(line))
	}
}

// Close closes the client (no-op: each operation uses short-lived TCP connection).
func (c *Adapter) Close() error {
	return nil
}

func (c *Adapter) connect(ctx context.Context, key string) (net.Conn, error) {
	target := c.pickAddress(key)
	conn, err := c.dial(ctx, "tcp", target)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(c.timeout)
	if deadlineFromCtx, ok := ctx.Deadline(); ok && deadlineFromCtx.Before(deadline) {
		deadline = deadlineFromCtx
	}
	_ = conn.SetDeadline(deadline)
	return conn, nil
}

func (c *Adapter) pickAddress(key string) string {
	if len(c.addresses) == 1 {
		return c.addresses[0]
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	index := int(hash.Sum32() % uint32(len(c.addresses)))
	return c.addresses[index]
}

func ttlToSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}

	if ttl > memcachedAbsoluteTTLThreshold {
		return int(time.Now().Add(ttl).Unix())
	}

	seconds := int(math.Ceil(ttl.Seconds()))
	if seconds <= 0 {
		return 1
	}
	return seconds
}
