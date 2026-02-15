package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// LoadWithSecrets loads configuration with separate secrets file support.
// Precedence: ENV > secrets file > config file > defaults
//
// Example:
//
//	config.yaml:
//	  database:
//	    type: postgres
//	    host: localhost
//
//	secrets.yaml:
//	  database:
//	    url: postgres://user:password@localhost:5432/db
//
// The secrets file is optional and automatically discovered:
// - If configFile is "config.yaml", looks for "secrets.yaml" in same directory
// - Can be explicitly set via <ENV_PREFIX>_SECRETS_FILE (defaults to APP_SECRETS_FILE)
func (l *ViperLoader) LoadWithSecrets() (*Config, *Config, error) {
	v := viper.New()

	// Start with defaults
	defaults := DefaultConfig()
	l.setDefaults(v, defaults)

	// Read main config file if provided
	if l.configFile != "" {
		v.SetConfigFile(l.configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, nil, fmt.Errorf("failed to read config file %s: %w", l.configFile, err)
		}
	}

	// Discover and load secrets file
	secretsFile, _, err := l.discoverSecretsFile()
	if err != nil {
		return nil, nil, err
	}
	var secrets *Config
	if secretsFile != "" {
		secretsViper := viper.New()
		secretsViper.SetConfigFile(secretsFile)
		if err := secretsViper.ReadInConfig(); err != nil {
			return nil, nil, fmt.Errorf("failed to read secrets file %s: %w", secretsFile, err)
		}
		var secretsCfg Config
		if err := secretsViper.Unmarshal(&secretsCfg); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal secrets file %s: %w", secretsFile, err)
		}
		secrets = &secretsCfg
		// Merge secrets into main config
		if err := v.MergeConfigMap(secretsViper.AllSettings()); err != nil {
			return nil, nil, fmt.Errorf("failed to merge secrets: %w", err)
		}
	}

	// Environment variables override everything
	v.SetEnvPrefix(l.envPrefix)
	l.bindLegacyEnvVars()
	l.bindEnvVars(v)

	// Unmarshal final config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := l.Validate(&cfg); err != nil {
		return nil, nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, secrets, nil
}

// discoverSecretsFile finds the secrets file using these rules:
// 1. Check <ENV_PREFIX>_SECRETS_FILE (default APP_SECRETS_FILE)
// 2. If configFile is set, look for secrets.{ext} in same directory
// 3. Look for secrets.yaml in current directory
// Returns the path, whether it came from explicit env var, and an error for invalid explicit env values.
func (l *ViperLoader) discoverSecretsFile() (string, bool, error) {
	// Check environment variable first
	secretsEnv := l.prefixedEnv("SECRETS_FILE")
	if rawSecretsFile, ok := os.LookupEnv(secretsEnv); ok {
		secretsFile := strings.TrimSpace(rawSecretsFile)
		if secretsFile == "" {
			return "", true, fmt.Errorf("%s is set but empty", secretsEnv)
		}
		info, err := os.Stat(secretsFile)
		if err != nil {
			return "", true, fmt.Errorf("%s points to an inaccessible file %s: %w", secretsEnv, secretsFile, err)
		}
		if info.IsDir() {
			return "", true, fmt.Errorf("%s must point to a file, got directory %s", secretsEnv, secretsFile)
		}
		return secretsFile, true, nil
	}

	// Try to derive from config file
	if l.configFile != "" {
		dir := filepath.Dir(l.configFile)
		ext := filepath.Ext(l.configFile)
		secretsFile := filepath.Join(dir, "secrets"+ext)
		if info, err := os.Stat(secretsFile); err == nil && !info.IsDir() {
			return secretsFile, false, nil
		}
	}

	// Try secrets.yaml in current directory
	for _, ext := range []string{".yaml", ".yml", ".json", ".toml"} {
		secretsFile := "secrets" + ext
		if info, err := os.Stat(secretsFile); err == nil && !info.IsDir() {
			return secretsFile, false, nil
		}
	}

	return "", false, nil
}
