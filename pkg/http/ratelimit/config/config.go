package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Enabled           bool          `mapstructure:"enabled"`
	Type              string        `mapstructure:"type"`
	RequestsPerSecond int           `mapstructure:"requests_per_second"`
	Burst             int           `mapstructure:"burst"`
	Window            time.Duration `mapstructure:"window"`
	Redis             RedisConfig   `mapstructure:"redis"`
}

type RedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	Prefix           string        `mapstructure:"prefix"`
}

type Extension struct {
	RateLimit Config `mapstructure:"rate_limit"`
}

func (Extension) DisabledCoreConfigSections() []string { return []string{"rate_limit"} }

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

func (e Extension) Validate() error {
	if !e.RateLimit.Enabled {
		return nil
	}
	validTypes := []string{"local", "redis"}
	if !contains(validTypes, strings.ToLower(strings.TrimSpace(e.RateLimit.Type))) {
		return fmt.Errorf("invalid rate_limit.type: %s (must be one of: %v)", e.RateLimit.Type, validTypes)
	}
	if strings.EqualFold(strings.TrimSpace(e.RateLimit.Type), "redis") && strings.TrimSpace(e.RateLimit.Redis.URL) == "" {
		return errors.New("rate_limit.redis.url is required when rate_limit.type=redis")
	}
	if e.RateLimit.RequestsPerSecond <= 0 {
		return errors.New("rate_limit.requests_per_second must be greater than zero when rate limiting is enabled")
	}
	if e.RateLimit.Burst < 0 {
		return errors.New("rate_limit.burst cannot be negative")
	}
	if e.RateLimit.Window <= 0 {
		return errors.New("rate_limit.window must be greater than zero")
	}
	return nil
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
