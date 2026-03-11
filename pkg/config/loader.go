package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"

	schedulerconfig "github.com/nimburion/nimburion/pkg/scheduler/config"
)

// Loader defines the interface for loading configuration
type Loader interface {
	Load() (*Config, error)
	Validate(*Config) error
}

// ValidationRequirements captures opt-in cross-family validation rules that
// should only run when a specific feature/runtime is actually in use.
type ValidationRequirements struct {
	RequireJobsEventBus bool
}

// ViperLoader implements Loader using Viper for configuration management
type ViperLoader struct {
	configFile             string
	envPrefix              string
	appNameDefault         string
	validationRequirements ValidationRequirements
}

// NewViperLoader creates a new ViperLoader
// configFile: path to configuration file (optional, can be empty)
// envPrefix: prefix for environment variables (e.g., "APP")
func NewViperLoader(configFile, envPrefix string) *ViperLoader {
	return &ViperLoader{
		configFile: configFile,
		envPrefix:  envPrefix,
	}
}

// WithAppNameDefault sets the default app.name used when no config/env override is provided.
func (l *ViperLoader) WithAppNameDefault(appName string) *ViperLoader {
	if l == nil {
		return l
	}
	l.appNameDefault = strings.TrimSpace(appName)
	return l
}

// WithValidationRequirements enables feature-aware validation rules.
func (l *ViperLoader) WithValidationRequirements(req ValidationRequirements) *ViperLoader {
	if l == nil {
		return l
	}
	l.validationRequirements = req
	return l
}

// Load loads configuration with precedence: ENV > file > defaults
func (l *ViperLoader) Load() (*Config, error) {
	v := viper.New()

	// Start with defaults
	defaults := DefaultConfig()
	l.setDefaults(v, defaults)

	// Read config file if provided
	if l.configFile != "" {
		v.SetConfigFile(l.configFile)
		if err := v.ReadInConfig(); err != nil {
			// Only return error if file was explicitly specified but couldn't be read
			return nil, fmt.Errorf("failed to read config file %s: %w", l.configFile, err)
		}
	}

	// Environment variables override file config through explicit bindings.
	v.SetEnvPrefix(l.envPrefix)

	// Bind all environment variables explicitly for nested structs
	if err := l.bindEnvVars(v); err != nil {
		return nil, fmt.Errorf("failed to bind environment variables: %w", err)
	}

	// Unmarshal into a new config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := l.Validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// bindEnvVars explicitly binds environment variables for nested structs.
func (l *ViperLoader) bindEnvVars(v *viper.Viper) error {
	for _, extension := range builtInConfigExtensions() {
		if err := bindConfigEnv(v, l.envPrefix, extension); err != nil {
			return err
		}
	}
	return nil
}

func (l *ViperLoader) prefixedEnv(suffix string) string {
	prefix := strings.TrimSpace(l.envPrefix)
	if prefix == "" {
		prefix = "APP"
	}
	return fmt.Sprintf("%s_%s", strings.ToUpper(prefix), suffix)
}

func (l *ViperLoader) defaultAppName(fallback string) string {
	if l != nil {
		if configured := strings.TrimSpace(l.appNameDefault); configured != "" {
			return configured
		}
	}
	return strings.TrimSpace(fallback)
}

// setDefaults sets default values in Viper from the default config
func (l *ViperLoader) setDefaults(v *viper.Viper, cfg *Config) {
	for _, extension := range builtInConfigExtensions() {
		if err := applyConfigDefaults(v, extension); err != nil {
			panic(err)
		}
	}
	v.SetDefault("app.name", l.defaultAppName(cfg.App.Name))
	v.SetDefault("app.environment", cfg.App.Environment)
}

// Validate validates the configuration and returns detailed errors
func (l *ViperLoader) Validate(cfg *Config) error {
	var errs []error

	for _, extension := range builtInConfigExtensions() {
		if err := unmarshalCoreExtension(cfg, extension); err != nil {
			errs = append(errs, err)
			continue
		}
		if validator, ok := extension.(extensionValidator); ok {
			if err := validator.Validate(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// First run family-owned validators, then apply only cross-family rules below.
	// Nothing in this section should validate an isolated config section when that
	// section already has an owner extension validator.

	// Normalize slices used by cross-family constraints.
	cfg.CORS.AllowOrigins = normalizeStringSlice(cfg.CORS.AllowOrigins)
	cfg.Observability.RequestLogging.Fields = normalizeStringSlice(cfg.Observability.RequestLogging.Fields)
	if cfg.Management.AuthEnabled && !cfg.Auth.Enabled {
		errs = append(errs, errors.New("auth.enabled must be true when management.auth_enabled is true"))
	}
	if cfg.CSRF.Enabled {
		if !cfg.Session.Enabled {
			errs = append(errs, errors.New("session.enabled must be true when csrf.enabled is true"))
		}
	}
	if cfg.SSE.Enabled {
		bus := strings.ToLower(strings.TrimSpace(cfg.SSE.Bus))
		if bus == "eventbus" && strings.TrimSpace(cfg.EventBus.Type) == "" {
			errs = append(errs, errors.New("eventbus.type is required when sse.bus=eventbus"))
		}
	}
	if l != nil && l.validationRequirements.RequireJobsEventBus {
		backend := strings.ToLower(strings.TrimSpace(cfg.Jobs.Backend))
		if backend == "" {
			backend = "eventbus"
		}
		if backend == "eventbus" && strings.TrimSpace(cfg.EventBus.Type) == "" {
			errs = append(errs, errors.New("eventbus.type is required when jobs.backend=eventbus"))
		}
	}
	// Validate Scheduler configuration
	if cfg.Scheduler.Enabled {
		lockProvider := strings.ToLower(strings.TrimSpace(cfg.Scheduler.LockProvider))
		if lockProvider == "" {
			lockProvider = schedulerconfig.LockProviderRedis
		}
		switch lockProvider {
		case schedulerconfig.LockProviderRedis:
			redisURL := strings.TrimSpace(cfg.Scheduler.Redis.URL)
			if redisURL == "" {
				redisURL = strings.TrimSpace(cfg.Cache.URL)
			}
			if redisURL == "" {
				errs = append(errs, errors.New("scheduler.redis.url (or cache.url) is required when scheduler.lock_provider is redis"))
			}
		case schedulerconfig.LockProviderPostgres:
			postgresURL := strings.TrimSpace(cfg.Scheduler.Postgres.URL)
			if postgresURL == "" {
				postgresURL = strings.TrimSpace(cfg.Database.URL)
			}
			if postgresURL == "" {
				errs = append(errs, errors.New("scheduler.postgres.url (or database.url) is required when scheduler.lock_provider is postgres"))
			}
		}
	}

	// Root-level composition rule: public and management listeners cannot collide.
	if cfg.Management.Enabled {
		if cfg.HTTP.Port == cfg.Management.Port {
			errs = append(errs, errors.New("http.port and management.port must be different"))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// normalizeStringSlice removes empty strings and trims whitespace
func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
