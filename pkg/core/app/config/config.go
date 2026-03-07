package config

import "github.com/spf13/viper"

// AppConfig configures application identity metadata.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

// Extension contributes the app config section as core-app-owned config surface.
type Extension struct {
	App AppConfig `mapstructure:"app"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"app"} }

// ApplyDefaults applies family-owned defaults for application identity.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "app")
	v.SetDefault("app.environment", "production")
}

// BindEnv binds family-owned environment variables for application identity.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	if err := v.BindEnv("app.name", prefixedEnv(prefix, "APP_NAME")); err != nil {
		return err
	}
	if err := v.BindEnv("app.environment", prefixedEnv(prefix, "APP_ENVIRONMENT"), prefixedEnv(prefix, "ENVIRONMENT")); err != nil {
		return err
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if prefix == "" {
		return suffix
	}
	return prefix + "_" + suffix
}
