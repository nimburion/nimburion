package pubsub

import (
	"context"
	"errors"
	"testing"
)

type storeStub struct {
	appended  []Message
	appendErr error
	closed    bool
}

func (s *storeStub) Append(_ context.Context, msg Message) error {
	if s.appendErr != nil {
		return s.appendErr
	}
	s.appended = append(s.appended, msg)
	return nil
}

func (s *storeStub) Recent(context.Context, Topic, int) ([]Message, error) {
	return nil, nil
}

func (s *storeStub) Close() error {
	s.closed = true
	return nil
}

func TestInMemoryBus_PublishAppendsToStore(t *testing.T) {
	store := &storeStub{}
	bus := NewInMemoryBus(InMemoryConfig{Store: store})

	err := bus.Publish(context.Background(), Message{Topic: "orders", Payload: []byte("ok")})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(store.appended) != 1 {
		t.Fatalf("expected 1 appended message, got %d", len(store.appended))
	}
}

func TestInMemoryBus_PublishReturnsStoreError(t *testing.T) {
	store := &storeStub{appendErr: errors.New("store failed")}
	bus := NewInMemoryBus(InMemoryConfig{Store: store})

	err := bus.Publish(context.Background(), Message{Topic: "orders", Payload: []byte("ok")})
	if err == nil {
		t.Fatal("expected publish error")
	}
}

func TestInMemoryBus_CloseClosesStore(t *testing.T) {
	store := &storeStub{}
	bus := NewInMemoryBus(InMemoryConfig{Store: store})

	if err := bus.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !store.closed {
		t.Fatal("expected store to be closed")
	}
}
