package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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

// NewMemcachedStore creates a memcached-backed session store.
func NewMemcachedStore(client MemcachedClient, prefix string) (*MemcachedStore, error) {
	if client == nil {
		return nil, errors.New("memcached client is required")
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = "session"
	}
	return &MemcachedStore{
		client: client,
		prefix: prefix,
	}, nil
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

func isMemcachedNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(err.Error()), "not found")
}
