package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryProcessedStore struct {
	mu        sync.Mutex
	processed map[string]map[string]time.Time
}

func newMemoryProcessedStore() *memoryProcessedStore {
	return &memoryProcessedStore{processed: map[string]map[string]time.Time{}}
}

func (s *memoryProcessedStore) IsProcessed(_ context.Context, consumerName, eventID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	consumerEvents, ok := s.processed[consumerName]
	if !ok {
		return false, nil
	}
	_, found := consumerEvents[eventID]
	return found, nil
}

func (s *memoryProcessedStore) MarkProcessed(_ context.Context, consumerName, eventID string, processedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.processed[consumerName]; !ok {
		s.processed[consumerName] = map[string]time.Time{}
	}
	s.processed[consumerName][eventID] = processedAt
	return nil
}

func (s *memoryProcessedStore) CleanupProcessedBefore(_ context.Context, before time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for consumer, events := range s.processed {
		for eventID, processedAt := range events {
			if deleted >= limit {
				return deleted, nil
			}
			if processedAt.Before(before) {
				delete(events, eventID)
				deleted++
			}
		}
		if len(events) == 0 {
			delete(s.processed, consumer)
		}
	}
	return deleted, nil
}

func TestIdempotencyGuard_ProcessOnce(t *testing.T) {
	store := newMemoryProcessedStore()
	guard, err := NewIdempotencyGuard(store)
	if err != nil {
		t.Fatalf("new guard failed: %v", err)
	}

	handlerCalls := 0
	handler := func(ctx context.Context) error {
		handlerCalls++
		_ = ctx
		return nil
	}

	executed, err := guard.ProcessOnce(context.Background(), "consumer-a", "evt-1", handler)
	if err != nil {
		t.Fatalf("first process failed: %v", err)
	}
	if !executed {
		t.Fatalf("expected first event to execute")
	}

	executed, err = guard.ProcessOnce(context.Background(), "consumer-a", "evt-1", handler)
	if err != nil {
		t.Fatalf("second process failed: %v", err)
	}
	if executed {
		t.Fatalf("expected duplicate event to be skipped")
	}

	if handlerCalls != 1 {
		t.Fatalf("expected handler calls to be 1, got %d", handlerCalls)
	}
}

func TestIdempotencyGuard_DoesNotMarkOnHandlerFailure(t *testing.T) {
	store := newMemoryProcessedStore()
	guard, err := NewIdempotencyGuard(store)
	if err != nil {
		t.Fatalf("new guard failed: %v", err)
	}

	executed, err := guard.ProcessOnce(context.Background(), "consumer-a", "evt-1", func(context.Context) error {
		return errors.New("handler failure")
	})
	if err == nil {
		t.Fatalf("expected handler failure")
	}
	if executed {
		t.Fatalf("should not report executed when handler fails")
	}

	processed, err := store.IsProcessed(context.Background(), "consumer-a", "evt-1")
	if err != nil {
		t.Fatalf("is processed failed: %v", err)
	}
	if processed {
		t.Fatalf("event should not be marked on handler failure")
	}
}
