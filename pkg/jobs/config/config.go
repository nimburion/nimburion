package config

import (
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/spf13/viper"
)

const (
	BackendEventBus = "eventbus"
	BackendRedis    = "redis"
)

// Config configures jobs runtime and backend behavior.
type Config struct {
	Backend      string       `mapstructure:"backend"`       // eventbus, redis
	DefaultQueue string       `mapstructure:"default_queue"` // default
	Worker       WorkerConfig `mapstructure:"worker"`
	Retry        RetryConfig  `mapstructure:"retry"`
	DLQ          DLQConfig    `mapstructure:"dlq"`
	Redis        RedisConfig  `mapstructure:"redis"`
}

// WorkerConfig configures jobs worker execution behavior.
type WorkerConfig struct {
	Concurrency    int           `mapstructure:"concurrency"`
	LeaseTTL       time.Duration `mapstructure:"lease_ttl"`
	ReserveTimeout time.Duration `mapstructure:"reserve_timeout"`
	StopTimeout    time.Duration `mapstructure:"stop_timeout"`
}

// RetryConfig configures retry policy for jobs processing.
type RetryConfig struct {
	MaxAttempts    int           `mapstructure:"max_attempts"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
	AttemptTimeout time.Duration `mapstructure:"attempt_timeout"`
}

// DLQConfig configures dead-letter routing.
type DLQConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	QueueSuffix string `mapstructure:"queue_suffix"`
}

// RedisConfig configures Redis-backed jobs backend settings.
type RedisConfig struct {
	URL              string        `mapstructure:"url"`
	Prefix           string        `mapstructure:"prefix"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// Extension contributes the jobs config section as family-owned config surface.
type Extension struct {
	Jobs Config `mapstructure:"jobs"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string {
	return []string{"jobs"}
}

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("jobs.backend", BackendEventBus)
	v.SetDefault("jobs.default_queue", "default")
	v.SetDefault("jobs.worker.concurrency", 1)
	v.SetDefault("jobs.worker.lease_ttl", 30*time.Second)
	v.SetDefault("jobs.worker.reserve_timeout", time.Second)
	v.SetDefault("jobs.worker.stop_timeout", 10*time.Second)
	v.SetDefault("jobs.retry.max_attempts", 5)
	v.SetDefault("jobs.retry.initial_backoff", time.Second)
	v.SetDefault("jobs.retry.max_backoff", time.Minute)
	v.SetDefault("jobs.retry.attempt_timeout", 30*time.Second)
	v.SetDefault("jobs.dlq.enabled", true)
	v.SetDefault("jobs.dlq.queue_suffix", ".dlq")
	v.SetDefault("jobs.redis.prefix", "nimburion:jobs")
	v.SetDefault("jobs.redis.operation_timeout", 5*time.Second)
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"jobs.backend", "JOBS_BACKEND",
		"jobs.default_queue", "JOBS_DEFAULT_QUEUE",
		"jobs.worker.concurrency", "JOBS_WORKER_CONCURRENCY",
		"jobs.worker.lease_ttl", "JOBS_WORKER_LEASE_TTL",
		"jobs.worker.reserve_timeout", "JOBS_WORKER_RESERVE_TIMEOUT",
		"jobs.worker.stop_timeout", "JOBS_WORKER_STOP_TIMEOUT",
		"jobs.retry.max_attempts", "JOBS_RETRY_MAX_ATTEMPTS",
		"jobs.retry.initial_backoff", "JOBS_RETRY_INITIAL_BACKOFF",
		"jobs.retry.max_backoff", "JOBS_RETRY_MAX_BACKOFF",
		"jobs.retry.attempt_timeout", "JOBS_RETRY_ATTEMPT_TIMEOUT",
		"jobs.dlq.enabled", "JOBS_DLQ_ENABLED",
		"jobs.dlq.queue_suffix", "JOBS_DLQ_QUEUE_SUFFIX",
		"jobs.redis.url", "JOBS_REDIS_URL",
		"jobs.redis.prefix", "JOBS_REDIS_PREFIX",
		"jobs.redis.operation_timeout", "JOBS_REDIS_OPERATION_TIMEOUT",
	)
}

func (e Extension) Validate() error {
	backend := strings.ToLower(strings.TrimSpace(e.Jobs.Backend))
	if backend == "" {
		backend = BackendEventBus
	}
	validBackends := []string{BackendEventBus, BackendRedis}
	if !contains(validBackends, backend) {
		return validationErrorf("validation.jobs.backend.invalid", "invalid jobs.backend: %s (must be one of: %v)", e.Jobs.Backend, validBackends)
	}
	if e.Jobs.Worker.Concurrency <= 0 {
		return validationError("validation.jobs.worker.concurrency.invalid", "jobs.worker.concurrency must be greater than zero")
	}
	if e.Jobs.Worker.LeaseTTL <= 0 {
		return validationError("validation.jobs.worker.lease_ttl.invalid", "jobs.worker.lease_ttl must be greater than zero")
	}
	if e.Jobs.Worker.ReserveTimeout <= 0 {
		return validationError("validation.jobs.worker.reserve_timeout.invalid", "jobs.worker.reserve_timeout must be greater than zero")
	}
	if e.Jobs.Worker.StopTimeout <= 0 {
		return validationError("validation.jobs.worker.stop_timeout.invalid", "jobs.worker.stop_timeout must be greater than zero")
	}
	if e.Jobs.Retry.MaxAttempts <= 0 {
		return validationError("validation.jobs.retry.max_attempts.invalid", "jobs.retry.max_attempts must be greater than zero")
	}
	if e.Jobs.Retry.InitialBackoff <= 0 {
		return validationError("validation.jobs.retry.initial_backoff.invalid", "jobs.retry.initial_backoff must be greater than zero")
	}
	if e.Jobs.Retry.MaxBackoff <= 0 {
		return validationError("validation.jobs.retry.max_backoff.invalid", "jobs.retry.max_backoff must be greater than zero")
	}
	if e.Jobs.Retry.MaxBackoff < e.Jobs.Retry.InitialBackoff {
		return validationError("validation.jobs.retry.max_backoff.range", "jobs.retry.max_backoff must be greater than or equal to jobs.retry.initial_backoff")
	}
	if e.Jobs.Retry.AttemptTimeout <= 0 {
		return validationError("validation.jobs.retry.attempt_timeout.invalid", "jobs.retry.attempt_timeout must be greater than zero")
	}
	if e.Jobs.DLQ.Enabled && strings.TrimSpace(e.Jobs.DLQ.QueueSuffix) == "" {
		return validationError("validation.jobs.dlq.queue_suffix.required", "jobs.dlq.queue_suffix is required when jobs.dlq.enabled is true")
	}
	if backend == BackendRedis {
		if strings.TrimSpace(e.Jobs.Redis.URL) == "" {
			return validationError("validation.jobs.redis.url.required", "jobs.redis.url is required when jobs.backend is redis")
		}
		if strings.TrimSpace(e.Jobs.Redis.Prefix) == "" {
			return validationError("validation.jobs.redis.prefix.required", "jobs.redis.prefix is required when jobs.backend is redis")
		}
		if e.Jobs.Redis.OperationTimeout <= 0 {
			return validationError("validation.jobs.redis.operation_timeout.invalid", "jobs.redis.operation_timeout must be greater than zero when jobs.backend is redis")
		}
	}
	return nil
}

func validationError(code, message string) error {
	return coreerrors.NewValidationWithCode(code, message, nil, nil)
}

func validationErrorf(code, format string, args ...any) error {
	return validationError(code, fmt.Sprintf(format, args...))
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	for index := 0; index < len(values); index += 2 {
		if err := v.BindEnv(values[index], prefixedEnv(prefix, values[index+1])); err != nil {
			return err
		}
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.TrimSpace(prefix) + "_" + suffix
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
