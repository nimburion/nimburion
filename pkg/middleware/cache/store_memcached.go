package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	storememcached "github.com/nimburion/nimburion/pkg/store/memcached"
)

// MemcachedClient abstracts memcached operations used by cache middleware.
type MemcachedClient interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Close() error
}

type memcachedAdapterClient struct {
	adapter *storememcached.Adapter
	timeout time.Duration
}

// Get retrieves a value from the context by key.
func (c *memcachedAdapterClient) Get(key string) ([]byte, error) {
	ctx, cancel := c.opContext()
	defer cancel()
	return c.adapter.Get(ctx, key)
}

// Set stores a value in the context with the given key.
func (c *memcachedAdapterClient) Set(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := c.opContext()
	defer cancel()
	return c.adapter.Set(ctx, key, value, ttl)
}

// Delete TODO: add description
func (c *memcachedAdapterClient) Delete(key string) error {
	ctx, cancel := c.opContext()
	defer cancel()
	return c.adapter.Delete(ctx, key)
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (c *memcachedAdapterClient) Close() error {
	return c.adapter.Close()
}

func (c *memcachedAdapterClient) opContext() (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), c.timeout)
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
		return nil, errors.New("memcached cache client is required")
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
func (s *MemcachedStore) Get(key string) ([]byte, error) {
	raw, err := s.client.Get(s.key(key))
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
func (s *MemcachedStore) Set(key string, value []byte, ttl time.Duration) error {
	return s.client.Set(s.key(key), value, ttl)
}

// Delete removes an entry.
func (s *MemcachedStore) Delete(key string) error {
	err := s.client.Delete(s.key(key))
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
