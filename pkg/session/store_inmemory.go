package session

import (
	"context"
	"sync"
	"time"
)

type inMemoryEntry struct {
	data      map[string]string
	expiresAt time.Time
}

// InMemoryStore is an in-process session backend for local development and tests.
type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]inMemoryEntry
}

// NewInMemoryStore creates a new in-memory session store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sessions: map[string]inMemoryEntry{},
	}
}

// Load fetches a session by id.
func (s *InMemoryStore) Load(_ context.Context, id string) (map[string]string, error) {
	s.mu.RLock()
	entry, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	return cloneMap(entry.data), nil
}

// Save persists a session.
func (s *InMemoryStore) Save(_ context.Context, id string, data map[string]string, ttl time.Duration) error {
	s.mu.Lock()
	s.sessions[id] = inMemoryEntry{
		data:      cloneMap(data),
		expiresAt: time.Now().Add(ttl),
	}
	s.mu.Unlock()
	return nil
}

// Delete removes a session.
func (s *InMemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
	return nil
}

// Touch extends a session expiration.
func (s *InMemoryStore) Touch(_ context.Context, id string, ttl time.Duration) error {
	s.mu.Lock()
	entry, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	entry.expiresAt = time.Now().Add(ttl)
	s.sessions[id] = entry
	s.mu.Unlock()
	return nil
}

// Close is a no-op for the in-memory store.
func (s *InMemoryStore) Close() error {
	return nil
}
