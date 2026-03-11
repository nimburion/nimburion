package idempotency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu         sync.Mutex
	processed  map[string]map[string]time.Time
	processing map[string]map[string]struct{}
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		processed:  map[string]map[string]time.Time{},
		processing: map[string]map[string]struct{}{},
	}
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

func (s *memoryStore) ExecuteAtomically(ctx context.Context, scope, key string, handler func(context.Context) error) (bool, error) {
	s.mu.Lock()
	if _, ok := s.processed[scope]; ok {
		if _, found := s.processed[scope][key]; found {
			s.mu.Unlock()
			return false, nil
		}
	}
	if _, ok := s.processing[scope]; !ok {
		s.processing[scope] = map[string]struct{}{}
	}
	if _, found := s.processing[scope][key]; found {
		s.mu.Unlock()
		return false, nil
	}
	s.processing[scope][key] = struct{}{}
	s.mu.Unlock()

	if err := handler(ctx); err != nil {
		s.mu.Lock()
		delete(s.processing[scope], key)
		if len(s.processing[scope]) == 0 {
			delete(s.processing, scope)
		}
		s.mu.Unlock()
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.processing[scope], key)
	if len(s.processing[scope]) == 0 {
		delete(s.processing, scope)
	}
	if _, ok := s.processed[scope]; !ok {
		s.processed[scope] = map[string]time.Time{}
	}
	s.processed[scope][key] = time.Now().UTC()
	return true, nil
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

type nonAtomicStore struct{}

func (nonAtomicStore) IsProcessed(context.Context, string, string) (bool, error)      { return false, nil }
func (nonAtomicStore) MarkProcessed(context.Context, string, string, time.Time) error { return nil }
func (nonAtomicStore) CleanupProcessedBefore(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func TestGuard_RejectsNonAtomicStore(t *testing.T) {
	guard, err := NewGuard(nonAtomicStore{})
	if err != nil {
		t.Fatalf("new guard failed: %v", err)
	}

	executed, err := guard.ExecuteOnce(context.Background(), "consumer-a", "evt-1", func(context.Context) error {
		t.Fatal("handler should not run")
		return nil
	})
	if err == nil || !errors.Is(err, ErrAtomicExecutionRequired) {
		t.Fatalf("expected ErrAtomicExecutionRequired, got %v", err)
	}
	if executed {
		t.Fatal("expected execution to be rejected")
	}
}
