package sse

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

type fakeEventBus struct {
	publishTopic   string
	publishMessage *eventbus.Message
	publishCtx     context.Context

	subscribeTopic string
	subscribeFn    eventbus.MessageHandler
	unsubscribed   []string
	closed         bool
}

func (f *fakeEventBus) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	f.publishCtx = ctx
	f.publishTopic = topic
	f.publishMessage = message
	return nil
}

func (f *fakeEventBus) PublishBatch(context.Context, string, []*eventbus.Message) error { return nil }

func (f *fakeEventBus) Subscribe(_ context.Context, topic string, handler eventbus.MessageHandler) error {
	f.subscribeTopic = topic
	f.subscribeFn = handler
	return nil
}

func (f *fakeEventBus) Unsubscribe(topic string) error {
	f.unsubscribed = append(f.unsubscribed, topic)
	return nil
}

func (f *fakeEventBus) Close() error {
	f.closed = true
	return nil
}

func (f *fakeEventBus) HealthCheck(context.Context) error { return nil }

func TestNewEventBusAdapter_ValidationAndDefaults(t *testing.T) {
	if _, err := NewEventBusAdapter(nil, EventBusConfig{}); err == nil {
		t.Fatal("expected error for nil eventbus")
	}

	f := &fakeEventBus{}
	a, err := NewEventBusAdapter(f, EventBusConfig{})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	if a.topicPref != "sse" {
		t.Fatalf("expected default topic prefix sse, got %q", a.topicPref)
	}
}

func TestEventBusAdapter_PublishMapsMessage(t *testing.T) {
	f := &fakeEventBus{}
	a, err := NewEventBusAdapter(f, EventBusConfig{TopicPrefix: "events", OperationTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	evt := Event{
		ID:        "e1",
		Channel:   "orders",
		TenantID:  "t1",
		Subject:   "user-1",
		Data:      []byte(`{"x":1}`),
		Timestamp: time.Now().UTC(),
	}
	if err := a.Publish(context.Background(), evt); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if f.publishTopic != "events.orders" {
		t.Fatalf("unexpected topic: %s", f.publishTopic)
	}
	if f.publishMessage == nil || f.publishMessage.Key != "user-1" {
		t.Fatalf("unexpected message mapping: %+v", f.publishMessage)
	}
	if _, ok := f.publishCtx.Deadline(); !ok {
		t.Fatal("expected publish context to have timeout")
	}
}

func TestEventBusAdapter_SubscribeAndClose(t *testing.T) {
	f := &fakeEventBus{}
	a, err := NewEventBusAdapter(f, EventBusConfig{TopicPrefix: "events"})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	var got []Event
	sub, err := a.Subscribe(context.Background(), "orders", func(e Event) {
		got = append(got, e)
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if f.subscribeTopic != "events.orders" {
		t.Fatalf("unexpected subscribe topic: %s", f.subscribeTopic)
	}

	evt := Event{ID: "e1", Channel: "orders"}
	raw, _ := json.Marshal(evt)
	if err := f.subscribeFn(context.Background(), &eventbus.Message{Value: raw}); err != nil {
		t.Fatalf("fake handler: %v", err)
	}
	if len(got) != 1 || got[0].ID != "e1" {
		t.Fatalf("unexpected forwarded events: %+v", got)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("close subscription: %v", err)
	}
	if err := sub.Close(); err != nil {
		t.Fatalf("close subscription idempotency: %v", err)
	}
	if len(f.unsubscribed) != 1 || f.unsubscribed[0] != "events.orders" {
		t.Fatalf("unexpected unsubscribe calls: %+v", f.unsubscribed)
	}

	if err := a.Close(); err != nil {
		t.Fatalf("adapter close: %v", err)
	}
	if !f.closed {
		t.Fatal("expected underlying bus close")
	}
}
