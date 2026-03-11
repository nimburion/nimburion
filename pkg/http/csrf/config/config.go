package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures HTTP CSRF protection.
type Config struct {
	Enabled        bool          `mapstructure:"enabled"`
	HeaderName     string        `mapstructure:"header_name"`
	CookieName     string        `mapstructure:"cookie_name"`
	CookiePath     string        `mapstructure:"cookie_path"`
	CookieDomain   string        `mapstructure:"cookie_domain"`
	CookieSecure   bool          `mapstructure:"cookie_secure"`
	CookieSameSite string        `mapstructure:"cookie_same_site"`
	CookieTTL      time.Duration `mapstructure:"cookie_ttl"`
	ExemptMethods  []string      `mapstructure:"exempt_methods"`
	ExemptPaths    []string      `mapstructure:"exempt_paths"`
}

// Extension contributes the CSRF config section as family-owned config surface.
type Extension struct {
	CSRF Config `mapstructure:"csrf"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"csrf"} }

// ApplyDefaults registers default CSRF configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("csrf.enabled", false)
	v.SetDefault("csrf.header_name", "X-CSRF-Token")
	v.SetDefault("csrf.cookie_name", "XSRF-TOKEN")
	v.SetDefault("csrf.cookie_path", "/")
	v.SetDefault("csrf.cookie_secure", true)
	v.SetDefault("csrf.cookie_same_site", "lax")
	v.SetDefault("csrf.cookie_ttl", 12*time.Hour)
	v.SetDefault("csrf.exempt_methods", []string{"GET", "HEAD", "OPTIONS", "TRACE"})
	v.SetDefault("csrf.exempt_paths", []string{})
}

// BindEnv binds CSRF configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"csrf.enabled", "CSRF_ENABLED",
		"csrf.header_name", "CSRF_HEADER_NAME",
		"csrf.cookie_name", "CSRF_COOKIE_NAME",
		"csrf.cookie_path", "CSRF_COOKIE_PATH",
		"csrf.cookie_domain", "CSRF_COOKIE_DOMAIN",
		"csrf.cookie_secure", "CSRF_COOKIE_SECURE",
		"csrf.cookie_same_site", "CSRF_COOKIE_SAME_SITE",
		"csrf.cookie_ttl", "CSRF_COOKIE_TTL",
		"csrf.exempt_methods", "CSRF_EXEMPT_METHODS",
		"csrf.exempt_paths", "CSRF_EXEMPT_PATHS",
	)
}

// Validate checks that enabled CSRF configuration is coherent.
func (e Extension) Validate() error {
	if !e.CSRF.Enabled {
		return nil
	}
	if strings.TrimSpace(e.CSRF.HeaderName) == "" {
		return validationError("validation.csrf.header_name.required", "csrf.header_name is required when csrf is enabled")
	}
	if strings.TrimSpace(e.CSRF.CookieName) == "" {
		return validationError("validation.csrf.cookie_name.required", "csrf.cookie_name is required when csrf is enabled")
	}
	if e.CSRF.CookieTTL <= 0 {
		return validationError("validation.csrf.cookie_ttl.invalid", "csrf.cookie_ttl must be greater than zero when csrf is enabled")
	}
	validSameSite := []string{"lax", "strict", "none"}
	value := strings.ToLower(strings.TrimSpace(e.CSRF.CookieSameSite))
	for _, candidate := range validSameSite {
		if candidate == value {
			return nil
		}
	}
	return validationErrorf("validation.csrf.cookie_same_site.invalid", "invalid csrf.cookie_same_site: %s (must be one of: %v)", e.CSRF.CookieSameSite, validSameSite)
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
