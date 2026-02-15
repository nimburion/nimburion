package sse

import (
	"context"
	"testing"
)

func TestInMemoryStore_AppendTrimsToMaxSize(t *testing.T) {
	s := NewInMemoryStore(2)
	ctx := context.Background()

	_ = s.Append(ctx, Event{ID: "1", Channel: "ch"})
	_ = s.Append(ctx, Event{ID: "2", Channel: "ch"})
	_ = s.Append(ctx, Event{ID: "3", Channel: "ch"})

	events, err := s.GetSince(ctx, "ch", "", 10)
	if err != nil {
		t.Fatalf("get since: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].ID != "2" || events[1].ID != "3" {
		t.Fatalf("unexpected trimmed ids: %s %s", events[0].ID, events[1].ID)
	}
}

func TestInMemoryStore_GetSinceRespectsLastIDAndLimit(t *testing.T) {
	s := NewInMemoryStore(10)
	ctx := context.Background()

	for _, id := range []string{"1", "2", "3", "4"} {
		_ = s.Append(ctx, Event{ID: id, Channel: "ch"})
	}

	events, err := s.GetSince(ctx, "ch", "1", 2)
	if err != nil {
		t.Fatalf("get since: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].ID != "3" || events[1].ID != "4" {
		t.Fatalf("unexpected filtered ids: %s %s", events[0].ID, events[1].ID)
	}
}
