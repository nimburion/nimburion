package dedup

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu    sync.Mutex
	items map[string]map[string]time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: map[string]map[string]time.Time{}}
}

func (s *memoryStore) IsDuplicate(_ context.Context, scope, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeItems, ok := s.items[scope]
	if !ok {
		return false, nil
	}
	expiresAt, ok := scopeItems[key]
	return ok && expiresAt.After(time.Now().UTC()), nil
}

func (s *memoryStore) MarkSeen(_ context.Context, scope, key string, _, retainUntil time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[scope]; !ok {
		s.items[scope] = map[string]time.Time{}
	}
	s.items[scope][key] = retainUntil
	return nil
}

func (s *memoryStore) CleanupExpiredBefore(_ context.Context, before time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for scope, items := range s.items {
		for key, expiresAt := range items {
			if deleted >= limit {
				return deleted, nil
			}
			if !expiresAt.After(before) {
				delete(items, key)
				deleted++
			}
		}
		if len(items) == 0 {
			delete(s.items, scope)
		}
	}
	return deleted, nil
}

func TestDeduplicator_CheckAndMark(t *testing.T) {
	store := newMemoryStore()
	d, err := New(store)
	if err != nil {
		t.Fatalf("new deduplicator: %v", err)
	}

	duplicate, err := d.CheckAndMark(context.Background(), "consumer-a", "evt-1", time.Minute)
	if err != nil {
		t.Fatalf("first check: %v", err)
	}
	if duplicate {
		t.Fatal("first key should not be duplicate")
	}

	duplicate, err = d.CheckAndMark(context.Background(), "consumer-a", "evt-1", time.Minute)
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if !duplicate {
		t.Fatal("second key should be duplicate")
	}
}

func TestCleaner_RemovesExpiredMarkers(t *testing.T) {
	store := newMemoryStore()
	_ = store.MarkSeen(context.Background(), "consumer-a", "evt-1", time.Now().UTC(), time.Now().UTC().Add(-time.Minute))

	cleaner, err := NewCleaner(store, CleanerConfig{
		CleanupEvery: 5 * time.Millisecond,
		BatchSize:    100,
	})
	if err != nil {
		t.Fatalf("new cleaner: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_ = cleaner.Run(ctx)

	duplicate, err := store.IsDuplicate(context.Background(), "consumer-a", "evt-1")
	if err != nil {
		t.Fatalf("is duplicate: %v", err)
	}
	if duplicate {
		t.Fatal("expected marker to be cleaned")
	}
}
