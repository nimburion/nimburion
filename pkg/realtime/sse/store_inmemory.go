package sse

import (
	"context"
	"sync"
)

// InMemoryStore keeps replay buffers in memory (single-node/dev usage).
type InMemoryStore struct {
	mu      sync.RWMutex
	maxSize int
	events  map[string][]Event
}

// NewInMemoryStore creates an in-memory replay store.
func NewInMemoryStore(maxSize int) *InMemoryStore {
	if maxSize <= 0 {
		maxSize = 256
	}
	return &InMemoryStore{
		maxSize: maxSize,
		events:  make(map[string][]Event),
	}
}

// Append stores one event in channel replay buffer.
func (s *InMemoryStore) Append(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	events := append(s.events[event.Channel], event)
	if len(events) > s.maxSize {
		events = events[len(events)-s.maxSize:]
	}
	s.events[event.Channel] = events
	return nil
}

// GetSince returns events strictly newer than lastEventID, in chronological order.
func (s *InMemoryStore) GetSince(_ context.Context, channel, lastEventID string, limit int) ([]Event, error) {
	s.mu.RLock()
	events := append([]Event(nil), s.events[channel]...)
	s.mu.RUnlock()

	if len(events) == 0 {
		return []Event{}, nil
	}
	if limit <= 0 {
		limit = len(events)
	}

	out := make([]Event, 0, limit)
	for _, evt := range events {
		if lastEventID == "" || evt.ID > lastEventID {
			out = append(out, evt)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// Close is a no-op for in-memory store.
func (s *InMemoryStore) Close() error {
	return nil
}
