package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	storememcached "github.com/nimburion/nimburion/pkg/cache/memcached"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// MemcachedClient abstracts memcached operations used by cache middleware.
type MemcachedClient interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

type memcachedAdapterClient struct {
	adapter *storememcached.Adapter
	timeout time.Duration
}

// Get retrieves a value from the context by key.
func (c *memcachedAdapterClient) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Get(ctx, key)
}

// Set stores a value in the context with the given key.
func (c *memcachedAdapterClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ctx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Set(ctx, key, value, ttl)
}

// Delete removes a key from the cache.
func (c *memcachedAdapterClient) Delete(ctx context.Context, key string) error {
	ctx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Delete(ctx, key)
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (c *memcachedAdapterClient) Close() error {
	return c.adapter.Close()
}

func (c *memcachedAdapterClient) opContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	// #nosec G118 -- the cancel function is returned to the caller, which defers it immediately.
	return context.WithTimeout(ctx, c.timeout)
}

// MemcachedConfig configures a memcached cache backend.
type MemcachedConfig struct {
	Addresses []string
	Timeout   time.Duration
	Prefix    string
}

// MemcachedStore persists cache entries in memcached.
type MemcachedStore struct {
	client MemcachedClient
	prefix string
}

// NewMemcachedStore creates a Memcached store from a generic memcached client.
func NewMemcachedStore(client MemcachedClient, prefix string) (*MemcachedStore, error) {
	if client == nil {
		return nil, coreerrors.NewValidationWithCode("validation.cache.memcached.client.required", "memcached cache client is required", nil, nil)
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = "http-cache"
	}
	return &MemcachedStore{
		client: client,
		prefix: prefix,
	}, nil
}

// NewMemcachedStoreFromConfig builds a Memcached store using framework adapter.
func NewMemcachedStoreFromConfig(cfg MemcachedConfig) (*MemcachedStore, error) {
	adapter, err := storememcached.NewMemcachedAdapter(cfg.Addresses, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	return NewMemcachedStore(&memcachedAdapterClient{adapter: adapter, timeout: cfg.Timeout}, cfg.Prefix)
}

// Get loads an entry.
func (s *MemcachedStore) Get(ctx context.Context, key string) ([]byte, error) {
	raw, err := s.client.Get(ctx, s.key(key))
	if err != nil {
		if isMemcachedNotFound(err) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	if len(raw) == 0 {
		return nil, ErrCacheMiss
	}
	return raw, nil
}

// Set stores an entry with TTL.
func (s *MemcachedStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, s.key(key), value, ttl)
}

// Delete removes an entry.
func (s *MemcachedStore) Delete(ctx context.Context, key string) error {
	err := s.client.Delete(ctx, s.key(key))
	if isMemcachedNotFound(err) {
		return nil
	}
	return err
}

// Close closes memcached client.
func (s *MemcachedStore) Close() error {
	return s.client.Close()
}

func (s *MemcachedStore) key(key string) string {
	return fmt.Sprintf("%s:%s", s.prefix, key)
}

func isMemcachedNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(err.Error()), "not found")
}
