package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

// EventBusConfig configures generic SSE bus backed by framework eventbus adapters.
type EventBusConfig struct {
	TopicPrefix      string
	OperationTimeout time.Duration
}

// EventBusAdapter bridges SSE bus interface to framework eventbus adapters
// (Kafka/RabbitMQ/SQS depending on selected eventbus implementation).
type EventBusAdapter struct {
	bus       eventbus.EventBus
	topicPref string
	timeout   time.Duration
}

// NewEventBusAdapter wraps a framework eventbus into SSE distributed bus.
func NewEventBusAdapter(bus eventbus.EventBus, cfg EventBusConfig) (*EventBusAdapter, error) {
	if bus == nil {
		return nil, fmt.Errorf("eventbus adapter is required")
	}
	prefix := strings.TrimSpace(cfg.TopicPrefix)
	if prefix == "" {
		prefix = "sse"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 5 * time.Second
	}
	return &EventBusAdapter{
		bus:       bus,
		topicPref: prefix,
		timeout:   cfg.OperationTimeout,
	}, nil
}

// Publish sends one SSE event into event bus topic.
func (b *EventBusAdapter) Publish(ctx context.Context, event Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()
	return b.bus.Publish(cctx, b.topic(event.Channel), &eventbus.Message{
		ID:          event.ID,
		Key:         event.Subject,
		Value:       raw,
		Headers:     map[string]string{"tenant_id": event.TenantID, "channel": event.Channel},
		ContentType: "application/json",
		Timestamp:   event.Timestamp,
	})
}

// Subscribe listens to one channel topic and forwards decoded events.
func (b *EventBusAdapter) Subscribe(ctx context.Context, channel string, handler func(Event)) (Subscription, error) {
	cctx, cancel := context.WithCancel(ctx)
	topic := b.topic(channel)
	err := b.bus.Subscribe(cctx, topic, func(_ context.Context, msg *eventbus.Message) error {
		var evt Event
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			return nil
		}
		handler(evt)
		return nil
	})
	if err != nil {
		cancel()
		return nil, err
	}
	return &eventBusSubscription{
		cancel: cancel,
		closeFn: func() error {
			return b.bus.Unsubscribe(topic)
		},
	}, nil
}

// Close closes underlying event bus.
func (b *EventBusAdapter) Close() error {
	return b.bus.Close()
}

func (b *EventBusAdapter) topic(channel string) string {
	return fmt.Sprintf("%s.%s", b.topicPref, channel)
}

type eventBusSubscription struct {
	cancel  context.CancelFunc
	once    sync.Once
	closeFn func() error
}

func (s *eventBusSubscription) Close() error {
	var err error
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.closeFn != nil {
			err = s.closeFn()
		}
	})
	return err
}
