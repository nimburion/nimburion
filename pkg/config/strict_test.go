package config

import (
	"os"
	"strings"
	"testing"
)

func TestViperLoader_LoadRejectsUnknownTopLevelKey(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	configFile := createTempYAMLFile(t, map[string]interface{}{
		"http": map[string]interface{}{
			"port": 8081,
		},
		"unknown_root": map[string]interface{}{
			"enabled": true,
		},
	})
	defer os.Remove(configFile)

	_, err := NewViperLoader(configFile, "APP").Load()
	if err == nil {
		t.Fatal("expected unknown top-level key validation error")
	}
	if !strings.Contains(err.Error(), `unknown config key "unknown_root"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadRejectsUnknownNestedKey(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	configFile := createTempYAMLFile(t, map[string]interface{}{
		"http": map[string]interface{}{
			"port":         8081,
			"unknown_leaf": true,
		},
	})
	defer os.Remove(configFile)

	_, err := NewViperLoader(configFile, "APP").Load()
	if err == nil {
		t.Fatal("expected unknown nested key validation error")
	}
	if !strings.Contains(err.Error(), `unknown config key "http.unknown_leaf"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadRejectsSchemaArtifactFile(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	configFile := createTempJSONFile(t, map[string]interface{}{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": false,
	})
	defer os.Remove(configFile)

	_, err := NewViperLoader(configFile, "APP").Load()
	if err == nil {
		t.Fatal("expected schema artifact validation error")
	}
	if !strings.Contains(err.Error(), "does not look like a runtime config document") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigProvider_LoadWithSecretsRejectsUnknownSecretKey(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	configFile := createTempYAMLFile(t, map[string]interface{}{
		"http": map[string]interface{}{
			"port": 8081,
		},
	})
	defer os.Remove(configFile)

	secretsFile := createTempYAMLFile(t, map[string]interface{}{
		"database": map[string]interface{}{
			"url": "postgres://user:pass@localhost:5432/app",
		},
		"not_a_secret_section": map[string]interface{}{
			"value": "bad",
		},
	})
	defer os.Remove(secretsFile)

	t.Setenv("APP_SECRETS_FILE", secretsFile)

	var cfg Config
	_, err := NewConfigProvider(configFile, "APP").LoadWithSecrets(&cfg)
	if err == nil {
		t.Fatal("expected strict validation failure for secrets file")
	}
	if !strings.Contains(err.Error(), `unknown config key "not_a_secret_section"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
