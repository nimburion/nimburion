package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	storememcached "github.com/nimburion/nimburion/pkg/session/memcached"
)

// MemcachedClient abstracts a memcached client so applications can plug any SDK.
type MemcachedClient interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Touch(ctx context.Context, key string, ttl time.Duration) error
	Close() error
}

// MemcachedStore persists sessions via a user-provided memcached client adapter.
type MemcachedStore struct {
	client MemcachedClient
	prefix string
}

type memcachedAdapterClient struct {
	adapter *storememcached.Adapter
	timeout time.Duration
}

// MemcachedConfig configures a Memcached-backed session store.
type MemcachedConfig struct {
	Addresses []string
	Timeout   time.Duration
	Prefix    string
}

// NewMemcachedStore creates a memcached-backed session store.
func NewMemcachedStore(client MemcachedClient, prefix string) (*MemcachedStore, error) {
	if client == nil {
		return nil, coreerrors.NewValidationWithCode("validation.session.memcached.client.required", "memcached client is required", nil, nil)
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = "session"
	}
	return &MemcachedStore{
		client: client,
		prefix: prefix,
	}, nil
}

// NewMemcachedStoreFromConfig builds a Memcached store using the framework adapter.
func NewMemcachedStoreFromConfig(cfg MemcachedConfig) (*MemcachedStore, error) {
	adapter, err := storememcached.NewMemcachedAdapter(cfg.Addresses, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	return NewMemcachedStore(&memcachedAdapterClient{adapter: adapter, timeout: cfg.Timeout}, cfg.Prefix)
}

// Load fetches a session by id.
func (s *MemcachedStore) Load(ctx context.Context, id string) (map[string]string, error) {
	raw, err := s.client.Get(ctx, s.key(id))
	if err != nil {
		if isMemcachedNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(raw) == 0 {
		return nil, ErrNotFound
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode memcached session payload: %w", err)
	}
	return payload, nil
}

// Save persists session payload.
func (s *MemcachedStore) Save(ctx context.Context, id string, data map[string]string, ttl time.Duration) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode memcached session payload: %w", err)
	}
	return s.client.Set(ctx, s.key(id), raw, ttl)
}

// Delete removes a session.
func (s *MemcachedStore) Delete(ctx context.Context, id string) error {
	err := s.client.Delete(ctx, s.key(id))
	if isMemcachedNotFound(err) {
		return nil
	}
	return err
}

// Touch extends session TTL.
func (s *MemcachedStore) Touch(ctx context.Context, id string, ttl time.Duration) error {
	err := s.client.Touch(ctx, s.key(id), ttl)
	if isMemcachedNotFound(err) {
		return ErrNotFound
	}
	return err
}

// Close releases the underlying memcached client.
func (s *MemcachedStore) Close() error {
	return s.client.Close()
}

func (s *MemcachedStore) key(id string) string {
	return fmt.Sprintf("%s:%s", s.prefix, id)
}

func (c *memcachedAdapterClient) Get(ctx context.Context, key string) ([]byte, error) {
	innerCtx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Get(innerCtx, key)
}

func (c *memcachedAdapterClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	innerCtx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Set(innerCtx, key, value, ttl)
}

func (c *memcachedAdapterClient) Delete(ctx context.Context, key string) error {
	innerCtx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Delete(innerCtx, key)
}

func (c *memcachedAdapterClient) Touch(ctx context.Context, key string, ttl time.Duration) error {
	innerCtx, cancel := c.opContext(ctx)
	defer cancel()
	return c.adapter.Touch(innerCtx, key, ttl)
}

func (c *memcachedAdapterClient) Close() error {
	return c.adapter.Close()
}

func (c *memcachedAdapterClient) opContext(parent context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return parent, func() {}
	}
	if parent == nil {
		parent = context.Background()
	}
	// #nosec G118 -- the cancel function is returned to the caller, which defers it immediately.
	return context.WithTimeout(parent, c.timeout)
}

func isMemcachedNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(err.Error()), "not found")
}
