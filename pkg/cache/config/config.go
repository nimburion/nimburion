package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
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

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("cache.max_conns", 10)
	v.SetDefault("cache.operation_timeout", 5*time.Second)
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"cache.type", "CACHE_TYPE",
		"cache.url", "CACHE_URL",
		"cache.max_conns", "CACHE_MAX_CONNS",
		"cache.operation_timeout", "CACHE_OPERATION_TIMEOUT",
	)
}

func (e Extension) Validate() error {
	if e.Cache.Type == "" {
		return nil
	}
	validTypes := []string{"redis", "inmemory"}
	if !contains(validTypes, e.Cache.Type) {
		return fmt.Errorf("invalid cache.type: %s (must be one of: %v)", e.Cache.Type, validTypes)
	}
	if e.Cache.Type == "redis" && strings.TrimSpace(e.Cache.URL) == "" {
		return errors.New("cache.url is required when cache.type is redis")
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
