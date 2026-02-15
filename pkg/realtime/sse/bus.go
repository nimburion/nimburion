package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Bus transports events across instances (distributed fan-out).
type Bus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, channel string, handler func(Event)) (Subscription, error)
	Close() error
}

// Subscription represents a cancelable bus subscription.
type Subscription interface {
	Close() error
}

// InMemoryBus is a local-only pub/sub bus used for tests/dev.
type InMemoryBus struct {
	mu       sync.RWMutex
	handlers map[string]map[uint64]func(Event)
	nextID   uint64
}

// NewInMemoryBus creates a local in-memory bus.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[string]map[uint64]func(Event)),
	}
}

// Publish delivers event to subscribed handlers on the same process.
func (b *InMemoryBus) Publish(_ context.Context, event Event) error {
	b.mu.RLock()
	handlers := b.handlers[event.Channel]
	copied := make([]func(Event), 0, len(handlers))
	for _, h := range handlers {
		copied = append(copied, h)
	}
	b.mu.RUnlock()

	for _, h := range copied {
		h(event)
	}
	return nil
}

// Subscribe registers a channel handler.
func (b *InMemoryBus) Subscribe(_ context.Context, channel string, handler func(Event)) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	if b.handlers[channel] == nil {
		b.handlers[channel] = make(map[uint64]func(Event))
	}
	id := b.nextID
	b.handlers[channel][id] = handler
	return &inMemoryBusSubscription{
		closeFn: func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.handlers[channel], id)
			if len(b.handlers[channel]) == 0 {
				delete(b.handlers, channel)
			}
		},
	}, nil
}

// Close is a no-op for in-memory bus.
func (b *InMemoryBus) Close() error {
	return nil
}

type inMemoryBusSubscription struct {
	once    sync.Once
	closeFn func()
}

func (s *inMemoryBusSubscription) Close() error {
	s.once.Do(s.closeFn)
	return nil
}

// RedisBusConfig configures Redis pub/sub distributed bus.
type RedisBusConfig struct {
	URL              string
	Prefix           string
	OperationTimeout time.Duration
	MaxConns         int
}

// RedisBus uses Redis pub/sub for distributed fan-out.
type RedisBus struct {
	client    *redis.Client
	prefix    string
	opTimeout time.Duration
}

// NewRedisBus creates a Redis-backed distributed bus.
func NewRedisBus(cfg RedisBusConfig) (*RedisBus, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("redis url is required")
	}
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	if cfg.MaxConns > 0 {
		opts.PoolSize = cfg.MaxConns
	}
	client := redis.NewClient(opts)

	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = "sse:bus"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 3 * time.Second
	}

	return &RedisBus{
		client:    client,
		prefix:    prefix,
		opTimeout: cfg.OperationTimeout,
	}, nil
}

// Publish pushes event to redis channel.
func (b *RedisBus) Publish(ctx context.Context, event Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, b.opTimeout)
	defer cancel()
	return b.client.Publish(cctx, b.key(event.Channel), raw).Err()
}

// Subscribe consumes redis pub/sub channel and forwards decoded events.
func (b *RedisBus) Subscribe(ctx context.Context, channel string, handler func(Event)) (Subscription, error) {
	pubsub := b.client.Subscribe(ctx, b.key(channel))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}

	subCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		msgCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-subCtx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				var evt Event
				if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
					continue
				}
				handler(evt)
			}
		}
	}()

	return &redisBusSubscription{
		cancel: cancel,
		pubsub: pubsub,
	}, nil
}

// Close closes Redis client.
func (b *RedisBus) Close() error {
	if b.client == nil {
		return nil
	}
	return b.client.Close()
}

func (b *RedisBus) key(channel string) string {
	return fmt.Sprintf("%s:%s", b.prefix, channel)
}

type redisBusSubscription struct {
	once   sync.Once
	cancel context.CancelFunc
	pubsub *redis.PubSub
}

func (s *redisBusSubscription) Close() error {
	var err error
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.pubsub != nil {
			err = s.pubsub.Close()
		}
	})
	return err
}
