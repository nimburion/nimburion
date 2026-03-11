package memcached

import (
	"context"
	"time"

	"github.com/nimburion/nimburion/internal/memcachedkit"
)

// Adapter is a lightweight Memcached text-protocol adapter.
type Adapter struct {
	client *memcachedkit.Client
}

// NewMemcachedAdapter creates a concrete memcached adapter using TCP text protocol.
func NewMemcachedAdapter(addresses []string, timeout time.Duration) (*Adapter, error) {
	client, err := memcachedkit.NewClient(addresses, timeout)
	if err != nil {
		return nil, err
	}
	return &Adapter{client: client}, nil
}

// Get fetches a value by key.
func (c *Adapter) Get(ctx context.Context, key string) ([]byte, error) {
	return c.client.Get(ctx, key)
}

// Set stores a value with TTL.
func (c *Adapter) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl)
}

// Delete removes a value by key.
func (c *Adapter) Delete(ctx context.Context, key string) error {
	return c.client.Delete(ctx, key)
}

// Touch refreshes TTL for an existing key.
func (c *Adapter) Touch(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Touch(ctx, key, ttl)
}

// Close closes the client (no-op: each operation uses short-lived TCP connection).
func (c *Adapter) Close() error {
	return c.client.Close()
}
