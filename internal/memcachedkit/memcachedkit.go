// Package memcachedkit provides low-level helpers shared by memcached adapters.
package memcachedkit

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

const absoluteTTLThreshold = 30 * 24 * time.Hour

// Client is a shared Memcached text-protocol client for role-specific adapters.
type Client struct {
	addresses []string
	timeout   time.Duration
	dial      func(ctx context.Context, network, address string) (net.Conn, error)
}

// NewClient creates a Memcached client with normalized addresses and timeout.
func NewClient(addresses []string, timeout time.Duration) (*Client, error) {
	return NewClientWithDial(addresses, timeout, nil)
}

// NewClientWithDial creates a Memcached client with an optional custom dialer.
func NewClientWithDial(addresses []string, timeout time.Duration, dial func(ctx context.Context, network, address string) (net.Conn, error)) (*Client, error) {
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
	if dial == nil {
		dial = (&net.Dialer{Timeout: timeout}).DialContext
	}
	return &Client{
		addresses: normalized,
		timeout:   timeout,
		dial:      dial,
	}, nil
}

// Get fetches a value by key.
func (c *Client) Get(ctx context.Context, key string) (data []byte, err error) {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := conn.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

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
	parts := strings.Fields(line)
	if len(parts) != 4 || parts[0] != "VALUE" {
		return nil, fmt.Errorf("unexpected memcached response: %s", line)
	}
	size, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid memcached size: %w", err)
	}
	payload := make([]byte, size+2)
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
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) (err error) {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	exptime := TTLToSeconds(ttl)
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
func (c *Client) Delete(ctx context.Context, key string) (err error) {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

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
func (c *Client) Touch(ctx context.Context, key string, ttl time.Duration) (err error) {
	conn, err := c.connect(ctx, key)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := conn.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	exptime := TTLToSeconds(ttl)
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

// Close closes the client. Operations use short-lived TCP connections, so this is a no-op.
func (c *Client) Close() error {
	return nil
}

func (c *Client) connect(ctx context.Context, key string) (net.Conn, error) {
	target := c.pickAddress(key)
	conn, err := c.dial(ctx, "tcp", target)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(c.timeout)
	if deadlineFromCtx, ok := ctx.Deadline(); ok && deadlineFromCtx.Before(deadline) {
		deadline = deadlineFromCtx
	}
	if err := conn.SetDeadline(deadline); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return conn, nil
}

func (c *Client) pickAddress(key string) string {
	if len(c.addresses) == 1 {
		return c.addresses[0]
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	target := uint64(hash.Sum32()) % uint64(len(c.addresses))
	for index := range c.addresses {
		if uint64(index) == target {
			return c.addresses[index]
		}
	}
	return c.addresses[0]
}

// TTLToSeconds converts a Go duration into Memcached exptime semantics.
func TTLToSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	if ttl > absoluteTTLThreshold {
		return int(time.Now().Add(ttl).Unix())
	}
	seconds := int(math.Ceil(ttl.Seconds()))
	if seconds <= 0 {
		return 1
	}
	return seconds
}
