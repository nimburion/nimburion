package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"
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

	sub, err := bus.Subscribe("orders")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer bus.Close()

	err = bus.Publish(context.Background(), Message{Topic: "orders", Payload: []byte("ok")})
	if err == nil {
		t.Fatal("expected publish error")
	}

	select {
	case msg := <-sub.Receive():
		t.Fatalf("subscriber should not have received message when store failed, got: %v", msg)
	default:
	}
}

func TestInMemoryBus_CloseClosesAllSubscribers(t *testing.T) {
	bus := NewInMemoryBus(InMemoryConfig{})

	sub1, err := bus.Subscribe("topic-a")
	if err != nil {
		t.Fatalf("Subscribe(topic-a) error = %v", err)
	}
	sub2, err := bus.Subscribe("topic-b")
	if err != nil {
		t.Fatalf("Subscribe(topic-b) error = %v", err)
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case _, ok := <-sub1.Receive():
		if ok {
			t.Fatal("sub1 channel should be closed after bus.Close()")
		}
	default:
		t.Fatal("sub1 channel not closed")
	}

	select {
	case _, ok := <-sub2.Receive():
		if ok {
			t.Fatal("sub2 channel should be closed after bus.Close()")
		}
	default:
		t.Fatal("sub2 channel not closed")
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

func TestInMemoryBus_CloseWhilePublishingDoesNotPanic(t *testing.T) {
	bus := NewInMemoryBus(InMemoryConfig{SubscriberBuffer: 1})
	sub, err := bus.Subscribe("orders")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			_ = bus.Publish(context.Background(), Message{Topic: "orders", Payload: []byte("ok")})
		}
	}()

	time.Sleep(5 * time.Millisecond)
	if err := bus.Unsubscribe("orders", sub); err != nil && !errors.Is(err, ErrBusClosed) {
		t.Fatalf("Unsubscribe() error = %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected publish goroutine to complete")
	}
}
