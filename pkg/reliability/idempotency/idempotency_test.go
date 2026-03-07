package idempotency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu        sync.Mutex
	processed map[string]map[string]time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{processed: map[string]map[string]time.Time{}}
}

func (s *memoryStore) IsProcessed(_ context.Context, scope, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scopeItems, ok := s.processed[scope]
	if !ok {
		return false, nil
	}
	_, found := scopeItems[key]
	return found, nil
}

func (s *memoryStore) MarkProcessed(_ context.Context, scope, key string, processedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.processed[scope]; !ok {
		s.processed[scope] = map[string]time.Time{}
	}
	s.processed[scope][key] = processedAt
	return nil
}

func (s *memoryStore) CleanupProcessedBefore(_ context.Context, before time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for scope, keys := range s.processed {
		for key, processedAt := range keys {
			if deleted >= limit {
				return deleted, nil
			}
			if processedAt.Before(before) {
				delete(keys, key)
				deleted++
			}
		}
		if len(keys) == 0 {
			delete(s.processed, scope)
		}
	}
	return deleted, nil
}

func TestGuard_ExecuteOnce(t *testing.T) {
	store := newMemoryStore()
	guard, err := NewGuard(store)
	if err != nil {
		t.Fatalf("new guard failed: %v", err)
	}

	handlerCalls := 0
	handler := func(context.Context) error {
		handlerCalls++
		return nil
	}

	executed, err := guard.ExecuteOnce(context.Background(), "consumer-a", "evt-1", handler)
	if err != nil {
		t.Fatalf("first process failed: %v", err)
	}
	if !executed {
		t.Fatal("expected first execution")
	}

	executed, err = guard.ExecuteOnce(context.Background(), "consumer-a", "evt-1", handler)
	if err != nil {
		t.Fatalf("second process failed: %v", err)
	}
	if executed {
		t.Fatal("expected duplicate key to be skipped")
	}
	if handlerCalls != 1 {
		t.Fatalf("expected one handler call, got %d", handlerCalls)
	}
}

func TestGuard_DoesNotMarkOnHandlerFailure(t *testing.T) {
	store := newMemoryStore()
	guard, err := NewGuard(store)
	if err != nil {
		t.Fatalf("new guard failed: %v", err)
	}

	executed, err := guard.ExecuteOnce(context.Background(), "consumer-a", "evt-1", func(context.Context) error {
		return errors.New("handler failure")
	})
	if err == nil {
		t.Fatal("expected handler failure")
	}
	if executed {
		t.Fatal("should not report executed when handler fails")
	}
}
