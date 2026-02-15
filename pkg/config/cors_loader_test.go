package config

import (
	"strings"
	"testing"
)

func TestViperLoader_CORSAllowAllAndCredentialsInvalid(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	t.Setenv("APP_CORS_ENABLED", "true")
	t.Setenv("APP_CORS_ALLOW_ALL_ORIGINS", "true")
	t.Setenv("APP_CORS_ALLOW_CREDENTIALS", "true")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for allow_all_origins + allow_credentials")
	}
	if !strings.Contains(err.Error(), "cors.allow_credentials cannot be true when cors.allow_all_origins is true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_CORSWildcardRequiresEnableFlag(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	t.Setenv("APP_CORS_ENABLED", "true")
	t.Setenv("APP_CORS_ALLOW_ORIGINS", "https://*.example.com")
	t.Setenv("APP_CORS_ALLOW_WILDCARD", "false")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for wildcard origin with allow_wildcard=false")
	}
	if !strings.Contains(err.Error(), "contains wildcard but cors.allow_wildcard is false") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_CORSWildcardSingleAsterisk(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	t.Setenv("APP_CORS_ENABLED", "true")
	t.Setenv("APP_CORS_ALLOW_WILDCARD", "true")
	t.Setenv("APP_CORS_ALLOW_ORIGINS", "https://*.*.example.com")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for multiple wildcard symbols")
	}
	if !strings.Contains(err.Error(), "can contain only one '*' wildcard") {
		t.Fatalf("unexpected error: %v", err)
	}
}
