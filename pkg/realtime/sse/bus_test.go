package sse

import (
	"context"
	"testing"
)

func TestInMemoryBus_PublishAndUnsubscribe(t *testing.T) {
	bus := NewInMemoryBus()

	var received []string
	sub, err := bus.Subscribe(context.Background(), "orders", func(e Event) {
		received = append(received, e.ID)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), Event{ID: "1", Channel: "orders"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(received) != 1 || received[0] != "1" {
		t.Fatalf("unexpected received events: %+v", received)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("close subscription: %v", err)
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("close subscription idempotency: %v", err)
	}

	if err := bus.Publish(context.Background(), Event{ID: "2", Channel: "orders"}); err != nil {
		t.Fatalf("publish after unsubscribe: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("expected no new events after unsubscribe, got %+v", received)
	}
}

func TestNewRedisBus_ValidationAndDefaults(t *testing.T) {
	if _, err := NewRedisBus(RedisBusConfig{}); err == nil {
		t.Fatal("expected error for empty redis url")
	}

	bus, err := NewRedisBus(RedisBusConfig{URL: "redis://localhost:6379/0"})
	if err != nil {
		t.Fatalf("new redis bus: %v", err)
	}
	defer bus.Close()

	if bus.prefix != "sse:bus" {
		t.Fatalf("expected default prefix sse:bus, got %q", bus.prefix)
	}
}
