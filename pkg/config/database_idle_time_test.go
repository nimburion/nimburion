package config

import (
	"testing"
	"time"
)

func TestDefaultConfig_DatabaseConnMaxIdleTime(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Database.ConnMaxIdleTime != 2*time.Minute {
		t.Fatalf("expected default conn max idle time 2m, got %v", cfg.Database.ConnMaxIdleTime)
	}
}

func TestViperLoader_Load_DatabaseConnMaxIdleTimeFromEnv(t *testing.T) {
	t.Setenv("APP_DB_CONN_MAX_IDLE_TIME", "45s")

	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Database.ConnMaxIdleTime != 45*time.Second {
		t.Fatalf("expected conn max idle time 45s, got %v", cfg.Database.ConnMaxIdleTime)
	}
}
