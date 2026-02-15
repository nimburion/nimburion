package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

type providerExtConfig struct {
	Feature struct {
		Enabled bool   `mapstructure:"enabled" default:"false" env:"APP_FEATURE_ENABLED" flag:"feature-enabled"`
		Name    string `mapstructure:"name" default:"default-name" env:"APP_FEATURE_NAME" flag:"feature-name"`
	} `mapstructure:"feature"`
}

type providerAcronymExtConfig struct {
	APIKey string `mapstructure:"api_key" env:"APP_EXT_API_KEY"`
}

func TestConfigProviderPrecedenceDefaultsFileEnvFlags(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(`
feature:
  enabled: false
  name: from-file
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("APP_FEATURE_NAME", "from-env")
	t.Setenv("APP_FEATURE_ENABLED", "true")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var ext providerExtConfig
	if err := RegisterFlagsFromStruct(flags, &ext); err != nil {
		t.Fatalf("register flags: %v", err)
	}
	if err := flags.Set("feature-name", "from-flag"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	provider := NewConfigProvider(configFile, "APP").WithFlags(flags)
	core := &Config{}
	if err := provider.Load(core, &ext); err != nil {
		t.Fatalf("load provider: %v", err)
	}

	if ext.Feature.Name != "from-flag" {
		t.Fatalf("expected flag value, got %s", ext.Feature.Name)
	}
	if !ext.Feature.Enabled {
		t.Fatalf("expected env override for bool to be true")
	}
}

func TestConfigProvider_AcronymFieldEnvBinding(t *testing.T) {
	t.Setenv("APP_EXT_API_KEY", "from-env")

	provider := NewConfigProvider("", "APP")
	core := &Config{}
	ext := &providerAcronymExtConfig{}
	if err := provider.Load(core, ext); err != nil {
		t.Fatalf("load provider: %v", err)
	}

	if ext.APIKey != "from-env" {
		t.Fatalf("expected ext api key from env, got %q", ext.APIKey)
	}
}

func TestConfigProvider_LoadWithSecrets_NoSecretsFileIsAllowed(t *testing.T) {
	t.Chdir(t.TempDir())
	unsetEnv(t, "APP_SECRETS_FILE")

	provider := NewConfigProvider("", "APP")
	core := &Config{}
	secrets, err := provider.LoadWithSecrets(core)
	if err != nil {
		t.Fatalf("load with secrets: %v", err)
	}
	if secrets != nil {
		t.Fatalf("expected nil secrets map when no secrets file exists")
	}
}

func TestConfigProvider_LoadWithSecrets_ExplicitMissingSecretsFileFails(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing-secrets.yaml")
	t.Setenv("APP_SECRETS_FILE", missingPath)

	provider := NewConfigProvider("", "APP")
	core := &Config{}
	_, err := provider.LoadWithSecrets(core)
	if err == nil {
		t.Fatal("expected error for missing explicit secrets file")
	}
	if !strings.Contains(err.Error(), "APP_SECRETS_FILE") {
		t.Fatalf("expected error mentioning APP_SECRETS_FILE, got %v", err)
	}
}

func TestConfigProvider_LoadWithSecrets_EmptyPrefixFallsBackToAppSecretsEnv(t *testing.T) {
	secretsFile := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(secretsFile, []byte("service:\n  name: from-secrets\n"), 0o600); err != nil {
		t.Fatalf("write secrets: %v", err)
	}
	t.Setenv("APP_SECRETS_FILE", secretsFile)

	provider := NewConfigProvider("", "")
	core := &Config{}
	_, err := provider.LoadWithSecrets(core)
	if err != nil {
		t.Fatalf("load with secrets: %v", err)
	}
	if core.Service.Name != "from-secrets" {
		t.Fatalf("expected service name from secrets, got %q", core.Service.Name)
	}
}

func TestConfigProvider_WithServiceNameDefault_AppliesWhenNotConfigured(t *testing.T) {
	provider := NewConfigProvider("", "APP").WithServiceNameDefault("orders-api")
	core := &Config{}
	if err := provider.Load(core); err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if core.Service.Name != "orders-api" {
		t.Fatalf("expected service name orders-api, got %q", core.Service.Name)
	}
}

func TestConfigProvider_WithServiceNameDefault_EnvOverrideWins(t *testing.T) {
	t.Setenv("APP_SERVICE_NAME", "billing-api")

	provider := NewConfigProvider("", "APP").WithServiceNameDefault("orders-api")
	core := &Config{}
	if err := provider.Load(core); err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if core.Service.Name != "billing-api" {
		t.Fatalf("expected service name billing-api from env, got %q", core.Service.Name)
	}
}

func TestConfigProvider_ObservabilityServiceName_FromEnv(t *testing.T) {
	t.Setenv("APP_OBSERVABILITY_SERVICE_NAME", "otel-billing")

	provider := NewConfigProvider("", "APP")
	core := &Config{}
	if err := provider.Load(core); err != nil {
		t.Fatalf("load provider: %v", err)
	}
	if core.Observability.ServiceName != "otel-billing" {
		t.Fatalf("expected observability service name otel-billing, got %q", core.Observability.ServiceName)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	original, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, original)
			return
		}
		_ = os.Unsetenv(key)
	})
}
