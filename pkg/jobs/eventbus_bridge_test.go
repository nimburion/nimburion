package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

type testEventBus struct {
	publishFn     func(ctx context.Context, topic string, message *eventbus.Message) error
	publishBatch  func(ctx context.Context, topic string, messages []*eventbus.Message) error
	subscribeFn   func(ctx context.Context, topic string, handler eventbus.MessageHandler) error
	unsubscribeFn func(topic string) error
	healthFn      func(ctx context.Context) error
	closeFn       func() error
}

func (b *testEventBus) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	if b.publishFn != nil {
		return b.publishFn(ctx, topic, message)
	}
	return nil
}

func (b *testEventBus) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	if b.publishBatch != nil {
		return b.publishBatch(ctx, topic, messages)
	}
	return nil
}

func (b *testEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	if b.subscribeFn != nil {
		return b.subscribeFn(ctx, topic, handler)
	}
	return nil
}

func (b *testEventBus) Unsubscribe(topic string) error {
	if b.unsubscribeFn != nil {
		return b.unsubscribeFn(topic)
	}
	return nil
}

func (b *testEventBus) HealthCheck(ctx context.Context) error {
	if b.healthFn != nil {
		return b.healthFn(ctx)
	}
	return nil
}

func (b *testEventBus) Close() error {
	if b.closeFn != nil {
		return b.closeFn()
	}
	return nil
}

func TestNewEventBusBridgeRequiresAdapter(t *testing.T) {
	if _, err := NewEventBusBridge(nil); err == nil {
		t.Fatal("expected error for nil adapter")
	}
}

func TestEventBusBridgeEnqueuePublishesToJobQueue(t *testing.T) {
	var (
		publishedTopic string
		publishedMsg   *eventbus.Message
	)

	bus := &testEventBus{
		publishFn: func(ctx context.Context, topic string, message *eventbus.Message) error {
			publishedTopic = topic
			publishedMsg = message
			return nil
		},
	}

	bridge, err := NewEventBusBridge(bus)
	if err != nil {
		t.Fatalf("new bridge failed: %v", err)
	}

	job := validJob()
	if err := bridge.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	if publishedTopic != job.Queue {
		t.Fatalf("topic mismatch: got %q want %q", publishedTopic, job.Queue)
	}
	if publishedMsg == nil {
		t.Fatal("expected published message")
	}
	if publishedMsg.Headers[HeaderJobName] != job.Name {
		t.Fatalf("expected %s header", HeaderJobName)
	}
}

func TestEventBusBridgeSubscribeDecodesJob(t *testing.T) {
	job := validJob()
	msg, err := job.ToEventBusMessage()
	if err != nil {
		t.Fatalf("to message failed: %v", err)
	}

	bus := &testEventBus{
		subscribeFn: func(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
			return handler(ctx, msg)
		},
	}

	bridge, err := NewEventBusBridge(bus)
	if err != nil {
		t.Fatalf("new bridge failed: %v", err)
	}

	var got *Job
	err = bridge.Subscribe(context.Background(), "maintenance", func(ctx context.Context, consumed *Job) error {
		got = consumed
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected decoded job")
	}
	if got.Name != job.Name {
		t.Fatalf("name mismatch: got %q want %q", got.Name, job.Name)
	}
}

func TestEventBusBridgeSubscribeRequiresQueueAndHandler(t *testing.T) {
	bridge, err := NewEventBusBridge(&testEventBus{})
	if err != nil {
		t.Fatalf("new bridge failed: %v", err)
	}

	if err := bridge.Subscribe(context.Background(), "", func(ctx context.Context, job *Job) error { return nil }); err == nil {
		t.Fatal("expected queue validation error")
	}
	if err := bridge.Subscribe(context.Background(), "jobs", nil); err == nil {
		t.Fatal("expected handler validation error")
	}
}

func TestEventBusBridgePassthroughs(t *testing.T) {
	bus := &testEventBus{
		unsubscribeFn: func(topic string) error {
			if topic != "jobs" {
				t.Fatalf("unexpected topic: %q", topic)
			}
			return nil
		},
		healthFn: func(ctx context.Context) error { return nil },
		closeFn:  func() error { return nil },
	}

	bridge, err := NewEventBusBridge(bus)
	if err != nil {
		t.Fatalf("new bridge failed: %v", err)
	}

	if err := bridge.Unsubscribe("jobs"); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}
	if err := bridge.HealthCheck(context.Background()); err != nil {
		t.Fatalf("healthcheck failed: %v", err)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestEventBusBridgePropagatesErrors(t *testing.T) {
	wantErr := errors.New("boom")
	bus := &testEventBus{
		publishFn: func(ctx context.Context, topic string, message *eventbus.Message) error {
			return wantErr
		},
	}
	bridge, err := NewEventBusBridge(bus)
	if err != nil {
		t.Fatalf("new bridge failed: %v", err)
	}
	if err := bridge.Enqueue(context.Background(), validJob()); !errors.Is(err, wantErr) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}
