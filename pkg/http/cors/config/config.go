package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

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

type Extension struct {
	CORS Config `mapstructure:"cors"`
}

func (Extension) DisabledCoreConfigSections() []string { return []string{"cors"} }

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

func (e Extension) Validate() error {
	if !e.CORS.Enabled {
		return nil
	}
	if len(e.CORS.AllowMethods) == 0 {
		return errors.New("cors.allow_methods must contain at least one method when cors is enabled")
	}
	if e.CORS.AllowCredentials && e.CORS.AllowAllOrigins {
		return errors.New("cors.allow_credentials cannot be true when cors.allow_all_origins is true")
	}
	if e.CORS.AllowAllOrigins && len(e.CORS.AllowOrigins) > 0 {
		return errors.New("cors.allow_all_origins and cors.allow_origins cannot both be set")
	}
	if e.CORS.MaxAge < 0 {
		return errors.New("cors.max_age cannot be negative")
	}
	if e.CORS.OptionsResponseStatusCode < 200 || e.CORS.OptionsResponseStatusCode > 299 {
		return errors.New("cors.options_response_status_code must be between 200 and 299")
	}
	for index, origin := range e.CORS.AllowOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			return fmt.Errorf("cors.allow_origins[%d] cannot be empty", index)
		}
		if strings.Contains(trimmed, "*") && trimmed != "*" {
			if !e.CORS.AllowWildcard {
				return fmt.Errorf("cors.allow_origins[%d] contains wildcard but cors.allow_wildcard is false", index)
			}
			if strings.Count(trimmed, "*") > 1 {
				return fmt.Errorf("cors.allow_origins[%d] can contain only one '*' wildcard", index)
			}
		}
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
