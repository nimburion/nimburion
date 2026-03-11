package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/eventbus"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type providerTestLogger struct{}

func (l *providerTestLogger) Debug(_ string, _ ...any) {}
func (l *providerTestLogger) Info(_ string, _ ...any)  {}
func (l *providerTestLogger) Warn(_ string, _ ...any)  {}
func (l *providerTestLogger) Error(_ string, _ ...any) {}
func (l *providerTestLogger) With(_ ...any) logger.Logger {
	return l
}

func (l *providerTestLogger) WithContext(_ context.Context) logger.Logger {
	return l
}

type providerMockEventBus struct{}

func (m *providerMockEventBus) Publish(_ context.Context, _ string, _ *eventbus.Message) error {
	return nil
}

func (m *providerMockEventBus) PublishBatch(_ context.Context, _ string, _ []*eventbus.Message) error {
	return nil
}

func (m *providerMockEventBus) Subscribe(_ context.Context, _ string, _ eventbus.MessageHandler) error {
	return nil
}

func (m *providerMockEventBus) Unsubscribe(_ string) error {
	return nil
}

func (m *providerMockEventBus) HealthCheck(_ context.Context) error {
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
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs.backend.unsupported" {
		t.Fatalf("Code = %q", appErr.Code)
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
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs.eventbus_type.required" {
		t.Fatalf("Code = %q", appErr.Code)
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
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs.backend.unsupported" {
		t.Fatalf("Code = %q", appErr.Code)
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
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs.eventbus_type.required" {
		t.Fatalf("Code = %q", appErr.Code)
	}
	if !strings.Contains(err.Error(), "eventbus.type is required when jobs.backend=eventbus") {
		t.Fatalf("expected eventbus type validation error, got %v", err)
	}
}

func TestRedisRuntimeAdapter_DefensiveErrors(t *testing.T) {
	var adapter *redisRuntimeAdapter
	if err := adapter.Enqueue(context.Background(), &Job{}); err == nil {
		t.Fatal("expected enqueue error")
	} else {
		var appErr *coreerrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != coreerrors.CodeNotInitialized {
			t.Fatalf("Code = %q", appErr.Code)
		}
	}
	if err := adapter.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected health error")
	} else {
		var appErr *coreerrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != coreerrors.CodeNotInitialized {
			t.Fatalf("Code = %q", appErr.Code)
		}
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("expected nil close error, got %v", err)
	}
}

func TestRedisRuntimeAdapter_SubscribeNotSupported(t *testing.T) {
	adapter := &redisRuntimeAdapter{}
	err := adapter.Subscribe(context.Background(), "queue", func(_ context.Context, _ *Job) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected subscribe error")
	}
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs.backend.redis.subscribe_unsupported" {
		t.Fatalf("Code = %q", appErr.Code)
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
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.eventbus.type.unsupported" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}

func TestProviderTestDoublesCompile(_ *testing.T) {
	var _ eventbus.EventBus = (*providerMockEventBus)(nil)
	var _ logger.Logger = (*providerTestLogger)(nil)
}
