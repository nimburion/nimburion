package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config configures HTTP locale negotiation.
type Config struct {
	Enabled          bool     `mapstructure:"enabled"`
	DefaultLocale    string   `mapstructure:"default_locale"`
	SupportedLocales []string `mapstructure:"supported_locales"`
	QueryParam       string   `mapstructure:"query_param"`
	HeaderName       string   `mapstructure:"header_name"`
	FallbackMode     string   `mapstructure:"fallback_mode"`
	CatalogPath      string   `mapstructure:"catalog_path"`
}

type Extension struct {
	I18n Config `mapstructure:"i18n"`
}

func (Extension) DisabledCoreConfigSections() []string { return []string{"i18n"} }

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("i18n.enabled", false)
	v.SetDefault("i18n.default_locale", "en")
	v.SetDefault("i18n.supported_locales", []string{"en"})
	v.SetDefault("i18n.query_param", "lang")
	v.SetDefault("i18n.header_name", "X-Locale")
	v.SetDefault("i18n.fallback_mode", "base")
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"i18n.enabled", "I18N_ENABLED",
		"i18n.default_locale", "I18N_DEFAULT_LOCALE",
		"i18n.supported_locales", "I18N_SUPPORTED_LOCALES",
		"i18n.query_param", "I18N_QUERY_PARAM",
		"i18n.header_name", "I18N_HEADER_NAME",
		"i18n.fallback_mode", "I18N_FALLBACK_MODE",
		"i18n.catalog_path", "I18N_CATALOG_PATH",
	)
}

func (e Extension) Validate() error {
	if !e.I18n.Enabled {
		return nil
	}
	if strings.TrimSpace(e.I18n.DefaultLocale) == "" {
		return errors.New("i18n.default_locale is required when i18n is enabled")
	}
	if len(e.I18n.SupportedLocales) == 0 {
		return errors.New("i18n.supported_locales must contain at least one locale when i18n is enabled")
	}
	if strings.TrimSpace(e.I18n.QueryParam) == "" {
		return errors.New("i18n.query_param is required when i18n is enabled")
	}
	if strings.TrimSpace(e.I18n.HeaderName) == "" {
		return errors.New("i18n.header_name is required when i18n is enabled")
	}
	normalizedSupported := make([]string, 0, len(e.I18n.SupportedLocales))
	for index, locale := range e.I18n.SupportedLocales {
		trimmed := strings.TrimSpace(locale)
		if trimmed == "" {
			return fmt.Errorf("i18n.supported_locales[%d] cannot be empty", index)
		}
		normalizedSupported = append(normalizedSupported, strings.ToLower(trimmed))
	}
	validFallbackModes := []string{"base", "default"}
	if !contains(validFallbackModes, strings.ToLower(strings.TrimSpace(e.I18n.FallbackMode))) {
		return fmt.Errorf("invalid i18n.fallback_mode: %s (must be one of: %v)", e.I18n.FallbackMode, validFallbackModes)
	}
	defaultLocale := strings.ToLower(strings.TrimSpace(e.I18n.DefaultLocale))
	if defaultLocale != "" && len(normalizedSupported) > 0 && !contains(normalizedSupported, defaultLocale) {
		return errors.New("i18n.default_locale must be included in i18n.supported_locales")
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
