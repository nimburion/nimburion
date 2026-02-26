package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/eventbus"
	eventbusfactory "github.com/nimburion/nimburion/pkg/eventbus/factory"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	BackendEventBus = config.JobsBackendEventBus
	BackendRedis    = config.JobsBackendRedis
)

// Config configures jobs runtime adapter selection.
type Config = config.JobsConfig

type eventBusAdapterFactory func(
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (eventbus.EventBus, error)

// NewRuntime creates a jobs runtime adapter from config.
// Default backend is eventbus to remain aligned with existing Nimburion async integrations.
func NewRuntime(cfg Config, eventBusCfg config.EventBusConfig, log logger.Logger) (jobs.Runtime, error) {
	return newRuntime(cfg, eventBusCfg, config.KafkaValidationConfig{}, log, eventbusfactory.NewEventBusAdapterWithValidation)
}

// NewRuntimeWithValidation creates a jobs runtime and applies eventbus schema validation when enabled.
func NewRuntimeWithValidation(
	cfg Config,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Runtime, error) {
	return newRuntime(cfg, eventBusCfg, validationCfg, log, eventbusfactory.NewEventBusAdapterWithValidation)
}

// NewBackend creates a lease-aware backend for jobs worker runtime.
func NewBackend(cfg Config, eventBusCfg config.EventBusConfig, log logger.Logger) (jobs.Backend, error) {
	return newBackend(cfg, eventBusCfg, config.KafkaValidationConfig{}, log, eventbusfactory.NewEventBusAdapterWithValidation)
}

// NewBackendWithValidation creates a backend and applies eventbus schema validation when enabled.
func NewBackendWithValidation(
	cfg Config,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Backend, error) {
	return newBackend(cfg, eventBusCfg, validationCfg, log, eventbusfactory.NewEventBusAdapterWithValidation)
}

func newRuntime(
	cfg Config,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
	newEventBus eventBusAdapterFactory,
) (jobs.Runtime, error) {
	if newEventBus == nil {
		return nil, fmt.Errorf("eventbus factory is required")
	}

	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = BackendEventBus
	}

	switch backend {
	case BackendEventBus:
		bus, err := newEventBus(eventBusCfg, validationCfg, log)
		if err != nil {
			return nil, err
		}
		return jobs.NewEventBusBridge(bus)
	case BackendRedis:
		redisBackend, err := jobs.NewRedisBackend(jobs.RedisBackendConfig{
			URL:              strings.TrimSpace(cfg.Redis.URL),
			Prefix:           strings.TrimSpace(cfg.Redis.Prefix),
			OperationTimeout: cfg.Redis.OperationTimeout,
			DLQSuffix:        strings.TrimSpace(cfg.DLQ.QueueSuffix),
		}, log)
		if err != nil {
			return nil, err
		}
		return &redisRuntimeAdapter{backend: redisBackend}, nil
	default:
		return nil, fmt.Errorf("unsupported jobs.backend %q (supported: %s, %s)", cfg.Backend, BackendEventBus, BackendRedis)
	}
}

func newBackend(
	cfg Config,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
	newEventBus eventBusAdapterFactory,
) (jobs.Backend, error) {
	if newEventBus == nil {
		return nil, fmt.Errorf("eventbus factory is required")
	}

	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = BackendEventBus
	}

	switch backend {
	case BackendEventBus:
		bus, err := newEventBus(eventBusCfg, validationCfg, log)
		if err != nil {
			return nil, err
		}
		runtime, err := jobs.NewEventBusBridge(bus)
		if err != nil {
			return nil, err
		}
		return jobs.NewRuntimeBackend(runtime, log, jobs.RuntimeBackendConfig{
			DLQSuffix:    strings.TrimSpace(cfg.DLQ.QueueSuffix),
			CloseRuntime: true,
		})
	case BackendRedis:
		return jobs.NewRedisBackend(jobs.RedisBackendConfig{
			URL:              strings.TrimSpace(cfg.Redis.URL),
			Prefix:           strings.TrimSpace(cfg.Redis.Prefix),
			OperationTimeout: cfg.Redis.OperationTimeout,
			DLQSuffix:        strings.TrimSpace(cfg.DLQ.QueueSuffix),
		}, log)
	default:
		return nil, fmt.Errorf("unsupported jobs.backend %q (supported: %s, %s)", cfg.Backend, BackendEventBus, BackendRedis)
	}
}

type redisRuntimeAdapter struct {
	backend jobs.Backend
}

// Enqueue adds a job to the processing queue.
func (r *redisRuntimeAdapter) Enqueue(ctx context.Context, job *jobs.Job) error {
	if r == nil || r.backend == nil {
		return fmt.Errorf("runtime backend is not initialized")
	}
	return r.backend.Enqueue(ctx, job)
}

// Subscribe registers a handler to receive messages from the specified topic.
func (r *redisRuntimeAdapter) Subscribe(ctx context.Context, queue string, handler jobs.Handler) error {
	return fmt.Errorf("jobs subscribe is not supported when jobs.backend is redis; use jobs worker")
}

// Unsubscribe removes the subscription for the specified topic.
func (r *redisRuntimeAdapter) Unsubscribe(queue string) error {
	return nil
}

// HealthCheck verifies the component is operational and can perform its intended function.
func (r *redisRuntimeAdapter) HealthCheck(ctx context.Context) error {
	if r == nil || r.backend == nil {
		return fmt.Errorf("runtime backend is not initialized")
	}
	return r.backend.HealthCheck(ctx)
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (r *redisRuntimeAdapter) Close() error {
	if r == nil || r.backend == nil {
		return nil
	}
	return r.backend.Close()
}
