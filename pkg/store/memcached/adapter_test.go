package memcached

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeMemcached struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newFakeMemcached() *fakeMemcached {
	return &fakeMemcached{
		data: map[string][]byte{},
	}
}

func (f *fakeMemcached) dial(_ context.Context, _ string, _ string) (net.Conn, error) {
	clientConn, serverConn := net.Pipe()
	go f.serve(serverConn)
	return clientConn, nil
}

func (f *fakeMemcached) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "set":
		if len(parts) != 5 {
			_, _ = io.WriteString(conn, "CLIENT_ERROR\r\n")
			return
		}
		key := parts[1]
		size, _ := strconv.Atoi(parts[4])
		payload := make([]byte, size+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return
		}
		f.mu.Lock()
		f.data[key] = append([]byte(nil), payload[:size]...)
		f.mu.Unlock()
		_, _ = io.WriteString(conn, "STORED\r\n")
	case "get":
		if len(parts) != 2 {
			_, _ = io.WriteString(conn, "END\r\n")
			return
		}
		key := parts[1]
		f.mu.Lock()
		value, ok := f.data[key]
		f.mu.Unlock()
		if !ok {
			_, _ = io.WriteString(conn, "END\r\n")
			return
		}
		_, _ = io.WriteString(conn, fmt.Sprintf("VALUE %s 0 %d\r\n", key, len(value)))
		_, _ = conn.Write(value)
		_, _ = io.WriteString(conn, "\r\nEND\r\n")
	case "delete":
		if len(parts) != 2 {
			_, _ = io.WriteString(conn, "NOT_FOUND\r\n")
			return
		}
		key := parts[1]
		f.mu.Lock()
		_, ok := f.data[key]
		delete(f.data, key)
		f.mu.Unlock()
		if ok {
			_, _ = io.WriteString(conn, "DELETED\r\n")
		} else {
			_, _ = io.WriteString(conn, "NOT_FOUND\r\n")
		}
	case "touch":
		if len(parts) != 3 {
			_, _ = io.WriteString(conn, "NOT_FOUND\r\n")
			return
		}
		key := parts[1]
		f.mu.Lock()
		_, ok := f.data[key]
		f.mu.Unlock()
		if ok {
			_, _ = io.WriteString(conn, "TOUCHED\r\n")
		} else {
			_, _ = io.WriteString(conn, "NOT_FOUND\r\n")
		}
	default:
		_, _ = io.WriteString(conn, "ERROR\r\n")
	}
}

func TestAdapter_CRUD(t *testing.T) {
	client, err := NewMemcachedAdapter([]string{"fake:11211"}, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	fake := newFakeMemcached()
	client.dial = fake.dial

	ctx := context.Background()
	if err := client.Set(ctx, "k1", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := client.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("expected v1, got %q", string(got))
	}

	if err := client.Touch(ctx, "k1", time.Minute); err != nil {
		t.Fatalf("touch: %v", err)
	}

	if err := client.Delete(ctx, "k1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = client.Get(ctx, "k1")
	if err == nil || err.Error() != "not found" {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestTTLToSeconds(t *testing.T) {
	t.Run("non positive ttl means no expiry", func(t *testing.T) {
		if got := ttlToSeconds(0); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
		if got := ttlToSeconds(-1 * time.Second); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})

	t.Run("short ttl uses relative seconds", func(t *testing.T) {
		got := ttlToSeconds(1500 * time.Millisecond)
		if got != 2 {
			t.Fatalf("expected ceil to 2 seconds, got %d", got)
		}
	})

	t.Run("ttl above 30 days uses absolute unix timestamp", func(t *testing.T) {
		ttl := 31 * 24 * time.Hour
		before := time.Now().Add(ttl).Unix()
		got := ttlToSeconds(ttl)
		after := time.Now().Add(ttl).Unix()

		if int64(got) < before-1 || int64(got) > after+1 {
			t.Fatalf("expected absolute unix timestamp in [%d,%d], got %d", before-1, after+1, got)
		}
	})
}
