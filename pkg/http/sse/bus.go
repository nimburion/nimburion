package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/internal/rediskit"
	"github.com/nimburion/nimburion/pkg/observability/logger"
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

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
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
	client    *rediskit.Client
	prefix    string
	opTimeout time.Duration
}

// NewRedisBus creates a Redis-backed distributed bus.
func NewRedisBus(cfg RedisBusConfig) (*RedisBus, error) {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = "sse:bus"
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 3 * time.Second
	}
	client, err := rediskit.NewClient(rediskit.Config{
		URL:              cfg.URL,
		MaxConns:         cfg.MaxConns,
		OperationTimeout: cfg.OperationTimeout,
	}, noopLogger{})
	if err != nil {
		return nil, err
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
	return b.client.Raw().Publish(cctx, b.key(event.Channel), raw).Err()
}

// Subscribe consumes redis pub/sub channel and forwards decoded events.
func (b *RedisBus) Subscribe(ctx context.Context, channel string, handler func(Event)) (Subscription, error) {
	pubsub := b.client.Raw().Subscribe(ctx, b.key(channel))
	if _, err := pubsub.Receive(ctx); err != nil {
		if closeErr := pubsub.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
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

type noopLogger struct{}

func (noopLogger) Debug(string, ...any)        {}
func (noopLogger) Info(string, ...any)         {}
func (noopLogger) Warn(string, ...any)         {}
func (noopLogger) Error(string, ...any)        {}
func (n noopLogger) With(...any) logger.Logger { return n }
func (n noopLogger) WithContext(context.Context) logger.Logger {
	return n
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
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
