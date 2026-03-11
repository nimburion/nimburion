package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures shared cache backend connections.
type Config struct {
	Type             string        `mapstructure:"type"` // redis, inmemory
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// Extension contributes the cache config section as family-owned config surface.
type Extension struct {
	Cache Config `mapstructure:"cache"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string {
	return []string{"cache"}
}

// ApplyDefaults registers default cache configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("cache.max_conns", 10)
	v.SetDefault("cache.operation_timeout", 5*time.Second)
}

// BindEnv binds cache configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"cache.type", "CACHE_TYPE",
		"cache.url", "CACHE_URL",
		"cache.max_conns", "CACHE_MAX_CONNS",
		"cache.operation_timeout", "CACHE_OPERATION_TIMEOUT",
	)
}

// Validate checks that cache configuration is valid for the selected backend.
func (e Extension) Validate() error {
	if e.Cache.Type == "" {
		return nil
	}
	validTypes := []string{"redis", "inmemory"}
	if !contains(validTypes, e.Cache.Type) {
		return validationErrorf("validation.cache.type.invalid", "invalid cache.type: %s (must be one of: %v)", e.Cache.Type, validTypes)
	}
	if e.Cache.Type == "redis" && strings.TrimSpace(e.Cache.URL) == "" {
		return validationError("validation.cache.url.required", "cache.url is required when cache.type is redis")
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
