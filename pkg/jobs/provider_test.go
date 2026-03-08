package jobs

import (
	"context"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/eventbus"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type providerTestLogger struct{}

func (l *providerTestLogger) Debug(msg string, args ...any) {}
func (l *providerTestLogger) Info(msg string, args ...any)  {}
func (l *providerTestLogger) Warn(msg string, args ...any)  {}
func (l *providerTestLogger) Error(msg string, args ...any) {}
func (l *providerTestLogger) With(args ...any) logger.Logger {
	return l
}
func (l *providerTestLogger) WithContext(ctx context.Context) logger.Logger {
	return l
}

type providerMockEventBus struct{}

func (m *providerMockEventBus) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	return nil
}
func (m *providerMockEventBus) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	return nil
}
func (m *providerMockEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	return nil
}
func (m *providerMockEventBus) Unsubscribe(topic string) error {
	return nil
}
func (m *providerMockEventBus) HealthCheck(ctx context.Context) error {
	return nil
}
func (m *providerMockEventBus) Close() error {
	return nil
}

func TestNewRuntimeFromConfig_DefaultBackendEventBus(t *testing.T) {
	runtime, err := newRuntimeFromConfig(
		jobsconfig.Config{},
		eventbusconfig.Config{Type: "kafka", Brokers: []string{"localhost:9092"}},
		schemavalidationconfig.KafkaValidationConfig{},
		&providerTestLogger{},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := runtime.(*EventBusBridge); !ok {
		t.Fatalf("expected *EventBusBridge, got %T", runtime)
	}
}

func TestNewRuntimeFromConfig_UnsupportedBackend(t *testing.T) {
	_, err := NewRuntimeFromConfig(
		jobsconfig.Config{Backend: "unknown"},
		eventbusconfig.Config{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
}

func TestNewRuntimeFromConfig_RedisRequiresURL(t *testing.T) {
	_, err := NewRuntimeFromConfig(
		jobsconfig.Config{Backend: "redis"},
		eventbusconfig.Config{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected redis url validation error")
	}
	if !strings.Contains(err.Error(), "redis url is required") {
		t.Fatalf("expected redis url error, got %v", err)
	}
}

func TestNewRuntimeFromConfig_EventBusBackendRequiresEventBusType(t *testing.T) {
	_, err := NewRuntimeFromConfig(
		jobsconfig.Config{Backend: jobsconfig.BackendEventBus},
		eventbusconfig.Config{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected missing eventbus type error")
	}
	if !strings.Contains(err.Error(), "eventbus.type is required when jobs.backend=eventbus") {
		t.Fatalf("expected eventbus type validation error, got %v", err)
	}
}

func TestNewBackendFromConfig_UnsupportedBackend(t *testing.T) {
	_, err := NewBackendFromConfig(
		jobsconfig.Config{Backend: "unknown"},
		eventbusconfig.Config{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
}

func TestNewBackendFromConfig_RedisRequiresURL(t *testing.T) {
	_, err := NewBackendFromConfig(
		jobsconfig.Config{Backend: "redis"},
		eventbusconfig.Config{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected redis url validation error")
	}
	if !strings.Contains(err.Error(), "redis url is required") {
		t.Fatalf("expected redis url error, got %v", err)
	}
}

func TestNewBackendFromConfig_EventBusBackendRequiresEventBusType(t *testing.T) {
	_, err := NewBackendFromConfig(
		jobsconfig.Config{Backend: jobsconfig.BackendEventBus},
		eventbusconfig.Config{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected missing eventbus type error")
	}
	if !strings.Contains(err.Error(), "eventbus.type is required when jobs.backend=eventbus") {
		t.Fatalf("expected eventbus type validation error, got %v", err)
	}
}

func TestRedisRuntimeAdapter_DefensiveErrors(t *testing.T) {
	var adapter *redisRuntimeAdapter
	if err := adapter.Enqueue(context.Background(), &Job{}); err == nil {
		t.Fatal("expected enqueue error")
	}
	if err := adapter.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected health error")
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("expected nil close error, got %v", err)
	}
}

func TestRedisRuntimeAdapter_SubscribeNotSupported(t *testing.T) {
	adapter := &redisRuntimeAdapter{}
	err := adapter.Subscribe(context.Background(), "queue", func(ctx context.Context, job *Job) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected subscribe error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected not supported error, got %v", err)
	}
}

func TestNewEventBusFromConfig_UnsupportedType(t *testing.T) {
	_, err := newEventBusFromConfig(
		eventbusconfig.Config{Type: "unknown"},
		schemavalidationconfig.KafkaValidationConfig{},
		&providerTestLogger{},
	)
	if err == nil {
		t.Fatal("expected unsupported eventbus type error")
	}
}

func TestProviderTestDoublesCompile(t *testing.T) {
	var _ eventbus.EventBus = (*providerMockEventBus)(nil)
	var _ logger.Logger = (*providerTestLogger)(nil)
}
