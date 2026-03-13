package pubsub

import (
	"context"
	"errors"
	"sync"
)

// ErrBusClosed indicates the bus has already been closed.
var ErrBusClosed = errors.New("pubsub bus is closed")

const defaultSubscriberBuffer = 64

// InMemoryConfig configures in-memory bus fan-out behavior.
type InMemoryConfig struct {
	SubscriberBuffer int
	Store            Store
}

// InMemoryBus is a goroutine-safe ephemeral in-process pub/sub implementation.
type InMemoryBus struct {
	mu               sync.RWMutex
	closed           bool
	subscriberBuffer int
	subscribers      map[Topic]map[*inMemorySubscriber]struct{}
	store            Store
}

// NewInMemoryBus constructs one in-memory bus.
func NewInMemoryBus(config InMemoryConfig) *InMemoryBus {
	buffer := config.SubscriberBuffer
	if buffer <= 0 {
		buffer = defaultSubscriberBuffer
	}
	return &InMemoryBus{
		subscriberBuffer: buffer,
		subscribers:      make(map[Topic]map[*inMemorySubscriber]struct{}),
		store:            config.Store,
	}
}

// Subscribe registers one subscriber for a topic.
func (b *InMemoryBus) Subscribe(topic Topic) (Subscriber, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrBusClosed
	}
	sub := &inMemorySubscriber{ch: make(chan Message, b.subscriberBuffer)}
	if _, ok := b.subscribers[topic]; !ok {
		b.subscribers[topic] = make(map[*inMemorySubscriber]struct{})
	}
	b.subscribers[topic][sub] = struct{}{}
	return sub, nil
}

// Publish fans out one message without blocking on slow subscribers.
func (b *InMemoryBus) Publish(ctx context.Context, msg Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrBusClosed
	}
	topicSubs := b.subscribers[msg.Topic]
	snapshot := make([]*inMemorySubscriber, 0, len(topicSubs))
	for sub := range topicSubs {
		snapshot = append(snapshot, sub)
	}
	b.mu.RUnlock()

	for _, sub := range snapshot {
		msgCopy := cloneMessage(msg)
		select {
		case sub.ch <- msgCopy:
		default:
			// non-blocking drop for slow subscribers
		}
	}

	if b.store != nil {
		if err := b.store.Append(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// Unsubscribe removes one subscriber from a topic.
func (b *InMemoryBus) Unsubscribe(topic Topic, sub Subscriber) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrBusClosed
	}
	memorySub, ok := sub.(*inMemorySubscriber)
	if !ok {
		return errors.New("invalid subscriber implementation")
	}
	topicSubs, ok := b.subscribers[topic]
	if !ok {
		return nil
	}
	delete(topicSubs, memorySub)
	if len(topicSubs) == 0 {
		delete(b.subscribers, topic)
	}
	return memorySub.close()
}

// Close shuts down the bus and all active subscribers.
func (b *InMemoryBus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	subscribers := b.subscribers
	b.subscribers = map[Topic]map[*inMemorySubscriber]struct{}{}
	b.mu.Unlock()

	for _, topicSubs := range subscribers {
		for sub := range topicSubs {
			_ = sub.close()
		}
	}
	if b.store != nil {
		if err := b.store.Close(); err != nil {
			return err
		}
	}
	return nil
}

type inMemorySubscriber struct {
	mu     sync.Mutex
	closed bool
	ch     chan Message
}

func (s *inMemorySubscriber) Receive() <-chan Message { return s.ch }

func (s *inMemorySubscriber) Close() error {
	return s.close()
}

func (s *inMemorySubscriber) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.ch)
	return nil
}

func cloneMessage(msg Message) Message {
	cloned := Message{Topic: msg.Topic}
	if len(msg.Payload) > 0 {
		cloned.Payload = append([]byte(nil), msg.Payload...)
	}
	if len(msg.Headers) > 0 {
		cloned.Headers = make(map[string]string, len(msg.Headers))
		for k, v := range msg.Headers {
			cloned.Headers[k] = v
		}
	}
	return cloned
}
