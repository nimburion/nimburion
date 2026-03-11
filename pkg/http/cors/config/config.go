package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures HTTP CORS policy.
type Config struct {
	Enabled                   bool          `mapstructure:"enabled"`
	AllowAllOrigins           bool          `mapstructure:"allow_all_origins"`
	AllowOrigins              []string      `mapstructure:"allow_origins"`
	AllowMethods              []string      `mapstructure:"allow_methods"`
	AllowPrivateNetwork       bool          `mapstructure:"allow_private_network"`
	AllowHeaders              []string      `mapstructure:"allow_headers"`
	ExposeHeaders             []string      `mapstructure:"expose_headers"`
	AllowCredentials          bool          `mapstructure:"allow_credentials"`
	MaxAge                    time.Duration `mapstructure:"max_age"`
	AllowWildcard             bool          `mapstructure:"allow_wildcard"`
	AllowBrowserExtensions    bool          `mapstructure:"allow_browser_extensions"`
	CustomSchemas             []string      `mapstructure:"custom_schemas"`
	AllowWebSockets           bool          `mapstructure:"allow_websockets"`
	AllowFiles                bool          `mapstructure:"allow_files"`
	OptionsResponseStatusCode int           `mapstructure:"options_response_status_code"`
}

// Extension contributes the CORS config section as family-owned config surface.
type Extension struct {
	CORS Config `mapstructure:"cors"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"cors"} }

// ApplyDefaults registers default CORS configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("cors.enabled", false)
	v.SetDefault("cors.allow_all_origins", false)
	v.SetDefault("cors.allow_origins", []string{})
	v.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"})
	v.SetDefault("cors.allow_private_network", false)
	v.SetDefault("cors.allow_headers", []string{})
	v.SetDefault("cors.expose_headers", []string{})
	v.SetDefault("cors.allow_credentials", false)
	v.SetDefault("cors.max_age", 12*time.Hour)
	v.SetDefault("cors.allow_wildcard", false)
	v.SetDefault("cors.allow_browser_extensions", false)
	v.SetDefault("cors.custom_schemas", []string{})
	v.SetDefault("cors.allow_websockets", false)
	v.SetDefault("cors.allow_files", false)
	v.SetDefault("cors.options_response_status_code", 204)
}

// BindEnv binds CORS configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"cors.enabled", "CORS_ENABLED",
		"cors.allow_all_origins", "CORS_ALLOW_ALL_ORIGINS",
		"cors.allow_origins", "CORS_ALLOW_ORIGINS",
		"cors.allow_methods", "CORS_ALLOW_METHODS",
		"cors.allow_private_network", "CORS_ALLOW_PRIVATE_NETWORK",
		"cors.allow_headers", "CORS_ALLOW_HEADERS",
		"cors.expose_headers", "CORS_EXPOSE_HEADERS",
		"cors.allow_credentials", "CORS_ALLOW_CREDENTIALS",
		"cors.max_age", "CORS_MAX_AGE",
		"cors.allow_wildcard", "CORS_ALLOW_WILDCARD",
		"cors.allow_browser_extensions", "CORS_ALLOW_BROWSER_EXTENSIONS",
		"cors.custom_schemas", "CORS_CUSTOM_SCHEMAS",
		"cors.allow_websockets", "CORS_ALLOW_WEBSOCKETS",
		"cors.allow_files", "CORS_ALLOW_FILES",
		"cors.options_response_status_code", "CORS_OPTIONS_RESPONSE_STATUS_CODE",
	)
}

// Validate checks that enabled CORS configuration is coherent.
func (e Extension) Validate() error {
	if !e.CORS.Enabled {
		return nil
	}
	if len(e.CORS.AllowMethods) == 0 {
		return validationError("validation.cors.allow_methods.required", "cors.allow_methods must contain at least one method when cors is enabled")
	}
	if e.CORS.AllowCredentials && e.CORS.AllowAllOrigins {
		return validationError("validation.cors.allow_credentials.invalid", "cors.allow_credentials cannot be true when cors.allow_all_origins is true")
	}
	if e.CORS.AllowAllOrigins && len(e.CORS.AllowOrigins) > 0 {
		return validationError("validation.cors.allow_origins.conflict", "cors.allow_all_origins and cors.allow_origins cannot both be set")
	}
	if e.CORS.MaxAge < 0 {
		return validationError("validation.cors.max_age.invalid", "cors.max_age cannot be negative")
	}
	if e.CORS.OptionsResponseStatusCode < 200 || e.CORS.OptionsResponseStatusCode > 299 {
		return validationError("validation.cors.options_response_status_code.invalid", "cors.options_response_status_code must be between 200 and 299")
	}
	for index, origin := range e.CORS.AllowOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			return validationErrorf("validation.cors.allow_origins.empty", "cors.allow_origins[%d] cannot be empty", index)
		}
		if strings.Contains(trimmed, "*") && trimmed != "*" {
			if !e.CORS.AllowWildcard {
				return validationErrorf("validation.cors.allow_origins.wildcard_disabled", "cors.allow_origins[%d] contains wildcard but cors.allow_wildcard is false", index)
			}
			if strings.Count(trimmed, "*") > 1 {
				return validationErrorf("validation.cors.allow_origins.wildcard_count", "cors.allow_origins[%d] can contain only one '*' wildcard", index)
			}
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
