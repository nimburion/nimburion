package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures shared session state and backing stores.
type Config struct {
	Enabled        bool            `mapstructure:"enabled"`
	Store          string          `mapstructure:"store"` // inmemory, redis, memcached
	TTL            time.Duration   `mapstructure:"ttl"`
	IdleTimeout    time.Duration   `mapstructure:"idle_timeout"`
	CookieName     string          `mapstructure:"cookie_name"`
	CookiePath     string          `mapstructure:"cookie_path"`
	CookieDomain   string          `mapstructure:"cookie_domain"`
	CookieSecure   bool            `mapstructure:"cookie_secure"`
	CookieHTTPOnly bool            `mapstructure:"cookie_http_only"`
	CookieSameSite string          `mapstructure:"cookie_same_site"` // lax, strict, none
	AutoCreate     bool            `mapstructure:"auto_create"`
	Redis          RedisConfig     `mapstructure:"redis"`
	Memcached      MemcachedConfig `mapstructure:"memcached"`
}

// RedisConfig configures Redis-backed sessions.
type RedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	Prefix           string        `mapstructure:"prefix"`
}

// MemcachedConfig configures memcached-backed sessions.
type MemcachedConfig struct {
	Addresses []string      `mapstructure:"addresses"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Prefix    string        `mapstructure:"prefix"`
}

// Extension contributes the session config section as family-owned config surface.
type Extension struct {
	Session Config `mapstructure:"session"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string {
	return []string{"session"}
}

// ApplyDefaults registers default session configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("session.enabled", false)
	v.SetDefault("session.store", "inmemory")
	v.SetDefault("session.ttl", 12*time.Hour)
	v.SetDefault("session.idle_timeout", 30*time.Minute)
	v.SetDefault("session.cookie_name", "sid")
	v.SetDefault("session.cookie_path", "/")
	v.SetDefault("session.cookie_secure", true)
	v.SetDefault("session.cookie_http_only", true)
	v.SetDefault("session.cookie_same_site", "lax")
	v.SetDefault("session.auto_create", true)
	v.SetDefault("session.redis.max_conns", 10)
	v.SetDefault("session.redis.operation_timeout", 5*time.Second)
	v.SetDefault("session.redis.prefix", "session")
	v.SetDefault("session.memcached.addresses", []string{})
	v.SetDefault("session.memcached.timeout", 500*time.Millisecond)
	v.SetDefault("session.memcached.prefix", "session")
}

// BindEnv binds session configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"session.enabled", "SESSION_ENABLED",
		"session.store", "SESSION_STORE",
		"session.ttl", "SESSION_TTL",
		"session.idle_timeout", "SESSION_IDLE_TIMEOUT",
		"session.cookie_name", "SESSION_COOKIE_NAME",
		"session.cookie_path", "SESSION_COOKIE_PATH",
		"session.cookie_domain", "SESSION_COOKIE_DOMAIN",
		"session.cookie_secure", "SESSION_COOKIE_SECURE",
		"session.cookie_http_only", "SESSION_COOKIE_HTTP_ONLY",
		"session.cookie_same_site", "SESSION_COOKIE_SAME_SITE",
		"session.auto_create", "SESSION_AUTO_CREATE",
		"session.redis.url", "SESSION_REDIS_URL",
		"session.redis.max_conns", "SESSION_REDIS_MAX_CONNS",
		"session.redis.operation_timeout", "SESSION_REDIS_OPERATION_TIMEOUT",
		"session.redis.prefix", "SESSION_REDIS_PREFIX",
		"session.memcached.addresses", "SESSION_MEMCACHED_ADDRESSES",
		"session.memcached.timeout", "SESSION_MEMCACHED_TIMEOUT",
		"session.memcached.prefix", "SESSION_MEMCACHED_PREFIX",
	)
}

// Validate checks that enabled session configuration is valid for the selected store.
func (e Extension) Validate() error {
	if !e.Session.Enabled {
		return nil
	}
	validStores := []string{"inmemory", "redis", "memcached"}
	store := strings.ToLower(strings.TrimSpace(e.Session.Store))
	if !contains(validStores, store) {
		return validationErrorf("validation.session.store.invalid", "invalid session.store: %s (must be one of: %v)", e.Session.Store, validStores)
	}
	if e.Session.TTL <= 0 {
		return validationError("validation.session.ttl.invalid", "session.ttl must be greater than zero when session is enabled")
	}
	if e.Session.IdleTimeout <= 0 {
		return validationError("validation.session.idle_timeout.invalid", "session.idle_timeout must be greater than zero when session is enabled")
	}
	validSameSite := []string{"lax", "strict", "none"}
	if !contains(validSameSite, strings.ToLower(strings.TrimSpace(e.Session.CookieSameSite))) {
		return validationErrorf("validation.session.cookie_same_site.invalid", "invalid session.cookie_same_site: %s (must be one of: %v)", e.Session.CookieSameSite, validSameSite)
	}
	switch store {
	case "redis":
		if strings.TrimSpace(e.Session.Redis.URL) == "" {
			return validationError("validation.session.redis.url.required", "session.redis.url is required when session.store=redis")
		}
	case "memcached":
		if len(e.Session.Memcached.Addresses) == 0 {
			return validationError("validation.session.memcached.addresses.required", "session.memcached.addresses must contain at least one endpoint when session.store=memcached")
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
