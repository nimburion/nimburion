package jobs

import (
	"context"
	"fmt"
	"strings"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/eventbus"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	"github.com/nimburion/nimburion/pkg/eventbus/kafka"
	"github.com/nimburion/nimburion/pkg/eventbus/rabbitmq"
	"github.com/nimburion/nimburion/pkg/eventbus/schema"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	"github.com/nimburion/nimburion/pkg/eventbus/sqs"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// NewRuntimeFromConfig creates a jobs runtime adapter from family config.
func NewRuntimeFromConfig(cfg jobsconfig.Config, eventBusCfg eventbusconfig.Config, log logger.Logger) (Runtime, error) {
	return newRuntimeFromConfig(cfg, eventBusCfg, schemavalidationconfig.KafkaValidationConfig{}, log)
}

// NewRuntimeFromConfigWithValidation creates a jobs runtime adapter and applies eventbus schema validation when enabled.
func NewRuntimeFromConfigWithValidation(
	cfg jobsconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (Runtime, error) {
	return newRuntimeFromConfig(cfg, eventBusCfg, validationCfg, log)
}

// NewBackendFromConfig creates a lease-aware jobs backend from family config.
func NewBackendFromConfig(cfg jobsconfig.Config, eventBusCfg eventbusconfig.Config, log logger.Logger) (Backend, error) {
	return newBackendFromConfig(cfg, eventBusCfg, schemavalidationconfig.KafkaValidationConfig{}, log)
}

// NewBackendFromConfigWithValidation creates a jobs backend and applies eventbus schema validation when enabled.
func NewBackendFromConfigWithValidation(
	cfg jobsconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (Backend, error) {
	return newBackendFromConfig(cfg, eventBusCfg, validationCfg, log)
}

func newRuntimeFromConfig(
	cfg jobsconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (Runtime, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = jobsconfig.BackendEventBus
	}
	if err := validateConfigCompatibility(backend, eventBusCfg); err != nil {
		return nil, err
	}

	switch backend {
	case jobsconfig.BackendEventBus:
		bus, err := newEventBusFromConfig(eventBusCfg, validationCfg, log)
		if err != nil {
			return nil, err
		}
		return NewEventBusBridge(bus)
	case jobsconfig.BackendRedis:
		redisBackend, err := NewRedisBackend(RedisBackendConfig{
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
		return nil, coreerrors.NewValidationWithCode(
			"validation.jobs.backend.unsupported",
			fmt.Sprintf("unsupported jobs.backend %q (supported: %s, %s)", cfg.Backend, jobsconfig.BackendEventBus, jobsconfig.BackendRedis),
			nil,
			map[string]interface{}{"backend": cfg.Backend},
		)
	}
}

func newBackendFromConfig(
	cfg jobsconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (Backend, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = jobsconfig.BackendEventBus
	}
	if err := validateConfigCompatibility(backend, eventBusCfg); err != nil {
		return nil, err
	}

	switch backend {
	case jobsconfig.BackendEventBus:
		bus, err := newEventBusFromConfig(eventBusCfg, validationCfg, log)
		if err != nil {
			return nil, err
		}
		runtime, err := NewEventBusBridge(bus)
		if err != nil {
			return nil, err
		}
		return NewRuntimeBackend(runtime, log, RuntimeBackendConfig{
			DLQSuffix:    strings.TrimSpace(cfg.DLQ.QueueSuffix),
			CloseRuntime: true,
		})
	case jobsconfig.BackendRedis:
		return NewRedisBackend(RedisBackendConfig{
			URL:              strings.TrimSpace(cfg.Redis.URL),
			Prefix:           strings.TrimSpace(cfg.Redis.Prefix),
			OperationTimeout: cfg.Redis.OperationTimeout,
			DLQSuffix:        strings.TrimSpace(cfg.DLQ.QueueSuffix),
		}, log)
	default:
		return nil, coreerrors.NewValidationWithCode(
			"validation.jobs.backend.unsupported",
			fmt.Sprintf("unsupported jobs.backend %q (supported: %s, %s)", cfg.Backend, jobsconfig.BackendEventBus, jobsconfig.BackendRedis),
			nil,
			map[string]interface{}{"backend": cfg.Backend},
		)
	}
}

func validateConfigCompatibility(backend string, eventBusCfg eventbusconfig.Config) error {
	if backend != jobsconfig.BackendEventBus {
		return nil
	}
	if strings.TrimSpace(eventBusCfg.Type) == "" {
		return coreerrors.NewValidationWithCode(
			"validation.jobs.eventbus_type.required",
			fmt.Sprintf("eventbus.type is required when jobs.backend=%s", jobsconfig.BackendEventBus),
			nil,
			nil,
		)
	}
	return nil
}

func newEventBusFromConfig(
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (eventbus.EventBus, error) {
	var (
		base eventbus.EventBus
		err  error
	)

	switch strings.ToLower(strings.TrimSpace(eventBusCfg.Type)) {
	case "kafka":
		base, err = kafka.NewFromEventBusConfig(eventBusCfg, log)
	case "rabbitmq":
		base, err = rabbitmq.NewFromEventBusConfig(eventBusCfg, log)
	case "sqs":
		base, err = sqs.NewFromEventBusConfig(eventBusCfg, log)
	default:
		return nil, coreerrors.NewValidationWithCode(
			"validation.eventbus.type.unsupported",
			fmt.Sprintf("unsupported eventbus.type %q (supported: kafka, rabbitmq, sqs)", eventBusCfg.Type),
			nil,
			map[string]interface{}{"type": eventBusCfg.Type},
		)
	}
	if err != nil {
		return nil, err
	}

	wrapped, err := schema.Wrap(base, validationCfg, log)
	if err != nil {
		return nil, err
	}
	return wrapped, nil
}

type redisRuntimeAdapter struct {
	backend Backend
}

func (r *redisRuntimeAdapter) Enqueue(ctx context.Context, job *Job) error {
	if r == nil || r.backend == nil {
		return coreerrors.NewNotInitialized("runtime backend is not initialized", ErrNotInitialized)
	}
	return r.backend.Enqueue(ctx, job)
}

func (r *redisRuntimeAdapter) Subscribe(ctx context.Context, queue string, handler Handler) error {
	return coreerrors.NewValidationWithCode(
		"validation.jobs.backend.redis.subscribe_unsupported",
		"jobs subscribe is not supported when jobs.backend is redis; use jobs worker",
		nil,
		nil,
	)
}

func (r *redisRuntimeAdapter) Unsubscribe(queue string) error { return nil }

func (r *redisRuntimeAdapter) HealthCheck(ctx context.Context) error {
	if r == nil || r.backend == nil {
		return coreerrors.NewNotInitialized("runtime backend is not initialized", ErrNotInitialized)
	}
	return r.backend.HealthCheck(ctx)
}

func (r *redisRuntimeAdapter) Close() error {
	if r == nil || r.backend == nil {
		return nil
	}
	return r.backend.Close()
}
