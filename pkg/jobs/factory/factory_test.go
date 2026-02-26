package factory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type testLogger struct{}

func (l *testLogger) Debug(msg string, args ...any) {}
func (l *testLogger) Info(msg string, args ...any)  {}
func (l *testLogger) Warn(msg string, args ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {}
func (l *testLogger) With(args ...any) logger.Logger {
	return l
}
func (l *testLogger) WithContext(ctx context.Context) logger.Logger {
	return l
}

type mockEventBus struct{}

func (m *mockEventBus) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	return nil
}
func (m *mockEventBus) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	return nil
}
func (m *mockEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	return nil
}
func (m *mockEventBus) Unsubscribe(topic string) error {
	return nil
}
func (m *mockEventBus) HealthCheck(ctx context.Context) error {
	return nil
}
func (m *mockEventBus) Close() error {
	return nil
}

func TestNewRuntime_DefaultBackendEventBus(t *testing.T) {
	called := false
	runtime, err := newRuntime(
		Config{},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			called = true
			return &mockEventBus{}, nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected eventbus factory to be called")
	}
	if _, ok := runtime.(*jobs.EventBusBridge); !ok {
		t.Fatalf("expected *jobs.EventBusBridge, got %T", runtime)
	}
}

func TestNewRuntime_ExplicitEventBus(t *testing.T) {
	runtime, err := newRuntime(
		Config{Backend: "eventbus"},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			return &mockEventBus{}, nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := runtime.(*jobs.EventBusBridge); !ok {
		t.Fatalf("expected *jobs.EventBusBridge, got %T", runtime)
	}
}

func TestNewRuntime_UnsupportedBackend(t *testing.T) {
	_, err := newRuntime(
		Config{Backend: "redis"},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			return &mockEventBus{}, nil
		},
	)
	if err == nil {
		t.Fatal("expected redis runtime initialization error")
	}
	if !strings.Contains(err.Error(), "redis url is required") {
		t.Fatalf("expected redis url error, got %v", err)
	}
}

func TestNewRuntime_PropagatesEventBusFactoryError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := newRuntime(
		Config{Backend: "eventbus"},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			return nil, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

func TestNewRuntime_RequiresFactory(t *testing.T) {
	_, err := newRuntime(
		Config{},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for nil eventbus factory")
	}
}

func TestNewRuntimeWithValidation_UnsupportedBackend(t *testing.T) {
	_, err := NewRuntimeWithValidation(
		Config{Backend: "unknown"},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
	)
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
}

func TestNewRuntime_UnsupportedBackendPublic(t *testing.T) {
	_, err := NewRuntime(
		Config{Backend: "unknown"},
		config.EventBusConfig{},
		&testLogger{},
	)
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
}

func TestNewBackend_DefaultEventBus(t *testing.T) {
	backend, err := newBackend(
		Config{},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			return &mockEventBus{}, nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := backend.(*jobs.RuntimeBackend); !ok {
		t.Fatalf("expected *jobs.RuntimeBackend, got %T", backend)
	}
}

func TestNewBackend_RedisRequiresURL(t *testing.T) {
	_, err := newBackend(
		Config{Backend: "redis"},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			return &mockEventBus{}, nil
		},
	)
	if err == nil {
		t.Fatal("expected redis url validation error")
	}
	if !strings.Contains(err.Error(), "redis url is required") {
		t.Fatalf("expected redis url error, got %v", err)
	}
}

func TestNewBackend_UnsupportedBackend(t *testing.T) {
	_, err := newBackend(
		Config{Backend: "unknown"},
		config.EventBusConfig{},
		config.KafkaValidationConfig{},
		&testLogger{},
		func(eventBusCfg config.EventBusConfig, validationCfg config.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
			return &mockEventBus{}, nil
		},
	)
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
}
