package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// SwaggerConfig configures OpenAPI/Swagger serving behavior.
type SwaggerConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	SpecPath string `mapstructure:"spec_path"`
}

// Extension contributes the swagger config section as HTTP-family-owned config surface.
type Extension struct {
	Swagger SwaggerConfig `mapstructure:"swagger"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"swagger"} }

// ApplyDefaults registers default OpenAPI configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("swagger.enabled", false)
	v.SetDefault("swagger.spec_path", "/api/openapi/openapi.yaml")
}

// BindEnv binds OpenAPI configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"swagger.enabled", "SWAGGER_ENABLED",
		"swagger.spec_path", "SWAGGER_SPEC_PATH",
	)
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
