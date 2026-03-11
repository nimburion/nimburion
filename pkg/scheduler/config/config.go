package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

const (
	// LockProviderRedis selects Redis as scheduler lock provider.
	LockProviderRedis = "redis"
	// LockProviderPostgres selects Postgres as scheduler lock provider.
	LockProviderPostgres = "postgres"
)

// Config configures distributed scheduler runtime behavior.
type Config struct {
	Enabled         bool           `mapstructure:"enabled"`
	Timezone        string         `mapstructure:"timezone"`
	LockProvider    string         `mapstructure:"lock_provider"` // redis, postgres
	LockTTL         time.Duration  `mapstructure:"lock_ttl"`
	DispatchTimeout time.Duration  `mapstructure:"dispatch_timeout"`
	Redis           RedisConfig    `mapstructure:"redis"`
	Postgres        PostgresConfig `mapstructure:"postgres"`
	Tasks           []TaskConfig   `mapstructure:"tasks"`
}

// RedisConfig configures Redis lock provider settings.
type RedisConfig struct {
	URL              string        `mapstructure:"url"`
	Prefix           string        `mapstructure:"prefix"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// PostgresConfig configures Postgres lock provider settings.
type PostgresConfig struct {
	URL              string        `mapstructure:"url"`
	Table            string        `mapstructure:"table"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// TaskConfig configures one scheduled task.
type TaskConfig struct {
	Name           string            `mapstructure:"name"`
	Cron           string            `mapstructure:"cron"`
	Queue          string            `mapstructure:"queue"`
	JobName        string            `mapstructure:"job_name"`
	Payload        string            `mapstructure:"payload"`
	Headers        map[string]string `mapstructure:"headers"`
	TenantID       string            `mapstructure:"tenant_id"`
	IdempotencyKey string            `mapstructure:"idempotency_key"`
	Timezone       string            `mapstructure:"timezone"`
	LockTTL        time.Duration     `mapstructure:"lock_ttl"`
	MisfirePolicy  string            `mapstructure:"misfire_policy"` // skip, fire_once
}

// Extension contributes the scheduler config section as family-owned config surface.
type Extension struct {
	Scheduler Config `mapstructure:"scheduler"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string {
	return []string{"scheduler"}
}

// ApplyDefaults registers default scheduler configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("scheduler.enabled", false)
	v.SetDefault("scheduler.timezone", "UTC")
	v.SetDefault("scheduler.lock_provider", LockProviderRedis)
	v.SetDefault("scheduler.lock_ttl", 45*time.Second)
	v.SetDefault("scheduler.dispatch_timeout", 10*time.Second)
	v.SetDefault("scheduler.redis.prefix", "nimburion:scheduler:lock")
	v.SetDefault("scheduler.redis.operation_timeout", 3*time.Second)
	v.SetDefault("scheduler.postgres.table", "nimburion_scheduler_locks")
	v.SetDefault("scheduler.postgres.operation_timeout", 3*time.Second)
}

// BindEnv binds scheduler configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"scheduler.enabled", "SCHEDULER_ENABLED",
		"scheduler.timezone", "SCHEDULER_TIMEZONE",
		"scheduler.lock_provider", "SCHEDULER_LOCK_PROVIDER",
		"scheduler.lock_ttl", "SCHEDULER_LOCK_TTL",
		"scheduler.dispatch_timeout", "SCHEDULER_DISPATCH_TIMEOUT",
		"scheduler.redis.url", "SCHEDULER_REDIS_URL",
		"scheduler.redis.prefix", "SCHEDULER_REDIS_PREFIX",
		"scheduler.redis.operation_timeout", "SCHEDULER_REDIS_OPERATION_TIMEOUT",
		"scheduler.postgres.url", "SCHEDULER_POSTGRES_URL",
		"scheduler.postgres.table", "SCHEDULER_POSTGRES_TABLE",
		"scheduler.postgres.operation_timeout", "SCHEDULER_POSTGRES_OPERATION_TIMEOUT",
	)
}

// Validate checks that enabled scheduler configuration is coherent.
func (e Extension) Validate() error {
	if !e.Scheduler.Enabled {
		return nil
	}
	lockProvider := strings.ToLower(strings.TrimSpace(e.Scheduler.LockProvider))
	if lockProvider == "" {
		lockProvider = LockProviderRedis
	}
	validProviders := []string{LockProviderRedis, LockProviderPostgres}
	if !contains(validProviders, lockProvider) {
		return validationErrorf("validation.scheduler.lock_provider.invalid", "invalid scheduler.lock_provider: %s (must be one of: %v)", e.Scheduler.LockProvider, validProviders)
	}
	if e.Scheduler.LockTTL <= 0 {
		return validationError("validation.scheduler.lock_ttl.invalid", "scheduler.lock_ttl must be greater than zero")
	}
	if e.Scheduler.DispatchTimeout <= 0 {
		return validationError("validation.scheduler.dispatch_timeout.invalid", "scheduler.dispatch_timeout must be greater than zero")
	}
	timezone := strings.TrimSpace(e.Scheduler.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return coreerrors.NewValidationWithCode("validation.scheduler.timezone.invalid", "invalid scheduler.timezone", nil, map[string]interface{}{"value": timezone}).
			WithMessage("invalid scheduler.timezone: " + err.Error())
	}
	switch lockProvider {
	case LockProviderRedis:
		if strings.TrimSpace(e.Scheduler.Redis.Prefix) == "" {
			return validationError("validation.scheduler.redis.prefix.required", "scheduler.redis.prefix is required when scheduler.lock_provider is redis")
		}
		if e.Scheduler.Redis.OperationTimeout <= 0 {
			return validationError("validation.scheduler.redis.operation_timeout.invalid", "scheduler.redis.operation_timeout must be greater than zero when scheduler.lock_provider is redis")
		}
	case LockProviderPostgres:
		if strings.TrimSpace(e.Scheduler.Postgres.Table) == "" {
			return validationError("validation.scheduler.postgres.table.required", "scheduler.postgres.table is required when scheduler.lock_provider is postgres")
		}
		if e.Scheduler.Postgres.OperationTimeout <= 0 {
			return validationError("validation.scheduler.postgres.operation_timeout.invalid", "scheduler.postgres.operation_timeout must be greater than zero when scheduler.lock_provider is postgres")
		}
	}
	taskNames := map[string]struct{}{}
	for idx, task := range e.Scheduler.Tasks {
		name := strings.TrimSpace(task.Name)
		if name == "" {
			return validationErrorf("validation.scheduler.tasks.name.required", "scheduler.tasks[%d].name is required", idx)
		}
		if _, exists := taskNames[name]; exists {
			return validationErrorf("validation.scheduler.tasks.name.duplicate", "scheduler.tasks contains duplicate name %q", name)
		}
		taskNames[name] = struct{}{}
		if strings.TrimSpace(task.Cron) == "" {
			return validationErrorf("validation.scheduler.tasks.cron.required", "scheduler.tasks[%s].cron is required", name)
		}
		if strings.TrimSpace(task.Queue) == "" {
			return validationErrorf("validation.scheduler.tasks.queue.required", "scheduler.tasks[%s].queue is required", name)
		}
		if strings.TrimSpace(task.JobName) == "" {
			return validationErrorf("validation.scheduler.tasks.job_name.required", "scheduler.tasks[%s].job_name is required", name)
		}
		misfire := strings.ToLower(strings.TrimSpace(task.MisfirePolicy))
		if misfire != "" && misfire != "skip" && misfire != "fire_once" {
			return validationErrorf("validation.scheduler.tasks.misfire_policy.invalid", "scheduler.tasks[%s].misfire_policy must be one of: skip, fire_once", name)
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
	if len(values)%2 != 0 {
		return fmt.Errorf("bindEnvPairs requires even number of values, got %d", len(values))
	}
	for len(values) > 0 {
		key, suffix := values[0], values[1]
		if err := v.BindEnv(key, prefixedEnv(prefix, suffix)); err != nil {
			return err
		}
		values = values[2:]
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
