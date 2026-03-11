package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures HTTP rate limiting.
type Config struct {
	Enabled           bool          `mapstructure:"enabled"`
	Type              string        `mapstructure:"type"`
	RequestsPerSecond int           `mapstructure:"requests_per_second"`
	Burst             int           `mapstructure:"burst"`
	Window            time.Duration `mapstructure:"window"`
	Redis             RedisConfig   `mapstructure:"redis"`
}

// RedisConfig configures Redis-backed rate limiting.
type RedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	Prefix           string        `mapstructure:"prefix"`
}

// Extension contributes the rate limit config section as family-owned config surface.
type Extension struct {
	RateLimit Config `mapstructure:"rate_limit"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"rate_limit"} }

// ApplyDefaults registers default rate limit configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("rate_limit.enabled", false)
	v.SetDefault("rate_limit.type", "local")
	v.SetDefault("rate_limit.requests_per_second", 10)
	v.SetDefault("rate_limit.burst", 20)
	v.SetDefault("rate_limit.window", time.Second)
	v.SetDefault("rate_limit.redis.url", "")
	v.SetDefault("rate_limit.redis.max_conns", 10)
	v.SetDefault("rate_limit.redis.operation_timeout", 3*time.Second)
	v.SetDefault("rate_limit.redis.prefix", "nimburion:ratelimit")
}

// BindEnv binds rate limit configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"rate_limit.enabled", "RATE_LIMIT_ENABLED",
		"rate_limit.type", "RATE_LIMIT_TYPE",
		"rate_limit.requests_per_second", "RATE_LIMIT_REQUESTS_PER_SECOND",
		"rate_limit.burst", "RATE_LIMIT_BURST",
		"rate_limit.window", "RATE_LIMIT_WINDOW",
		"rate_limit.redis.url", "RATE_LIMIT_REDIS_URL",
		"rate_limit.redis.max_conns", "RATE_LIMIT_REDIS_MAX_CONNS",
		"rate_limit.redis.operation_timeout", "RATE_LIMIT_REDIS_OPERATION_TIMEOUT",
		"rate_limit.redis.prefix", "RATE_LIMIT_REDIS_PREFIX",
	)
}

// Validate checks that enabled rate limit configuration is coherent.
func (e Extension) Validate() error {
	if !e.RateLimit.Enabled {
		return nil
	}
	validTypes := []string{"local", "redis"}
	if !contains(validTypes, strings.ToLower(strings.TrimSpace(e.RateLimit.Type))) {
		return validationErrorf("validation.rate_limit.type.invalid", "invalid rate_limit.type: %s (must be one of: %v)", e.RateLimit.Type, validTypes)
	}
	if strings.EqualFold(strings.TrimSpace(e.RateLimit.Type), "redis") && strings.TrimSpace(e.RateLimit.Redis.URL) == "" {
		return validationError("validation.rate_limit.redis.url.required", "rate_limit.redis.url is required when rate_limit.type=redis")
	}
	if e.RateLimit.RequestsPerSecond <= 0 {
		return validationError("validation.rate_limit.requests_per_second.invalid", "rate_limit.requests_per_second must be greater than zero when rate limiting is enabled")
	}
	if e.RateLimit.Burst < 0 {
		return validationError("validation.rate_limit.burst.invalid", "rate_limit.burst cannot be negative")
	}
	if e.RateLimit.Window <= 0 {
		return validationError("validation.rate_limit.window.invalid", "rate_limit.window must be greater than zero")
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
