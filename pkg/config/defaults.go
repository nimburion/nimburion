package config

import "github.com/spf13/viper"

// DefaultConfig returns the root config assembled from family-owned defaults.
func DefaultConfig() *Config {
	v := viper.New()

	for _, extension := range builtInConfigExtensions() {
		if err := applyConfigDefaults(v, extension); err != nil {
			panic(err)
		}
	}

	v.SetDefault("app.name", "app")
	v.SetDefault("app.environment", "production")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(err)
	}

	return &cfg
}
