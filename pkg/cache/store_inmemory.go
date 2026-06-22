package cache

import (
	"context"
	"sync"
	"time"
)

type inMemoryItem struct {
	value     []byte
	expiresAt time.Time
}

// InMemoryStore is a simple in-process cache backend.
type InMemoryStore struct {
	mu    sync.RWMutex
	items map[string]inMemoryItem
}

// NewInMemoryStore creates an in-memory cache store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		items: make(map[string]inMemoryItem),
	}
}

// Get loads a key from memory.
func (s *InMemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrCacheMiss
	}
	if time.Now().After(item.expiresAt) {
		s.mu.Lock()
		delete(s.items, key)
		s.mu.Unlock()
		return nil, ErrCacheMiss
	}
	return append([]byte{}, item.value...), nil
}

// Set stores a key with TTL.
func (s *InMemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = time.Second
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = inMemoryItem{
		value:     append([]byte{}, value...),
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Delete removes a key.
func (s *InMemoryStore) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

// Close is a no-op for in-memory store.
func (s *InMemoryStore) Close() error {
	return nil
}
