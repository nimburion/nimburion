package redis

import (
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// TestNewAdapter_InvalidURL tests adapter creation with invalid URL
func TestNewAdapter_InvalidURL(t *testing.T) {
	log, _ := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})

	cfg := Config{
		URL:              "invalid://url",
		MaxConns:         10,
		OperationTimeout: 5 * time.Second,
	}

	_, err := NewAdapter(cfg, log)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

// TestNewAdapter_EmptyURL tests adapter creation with empty URL
func TestNewAdapter_EmptyURL(t *testing.T) {
	log, _ := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})

	cfg := Config{
		URL:              "",
		MaxConns:         10,
		OperationTimeout: 5 * time.Second,
	}

	_, err := NewAdapter(cfg, log)
	if err == nil {
		t.Error("Expected error for empty URL, got nil")
	}
	if err.Error() != "redis URL is required" {
		t.Errorf("Expected 'redis URL is required' error, got: %v", err)
	}
}

// TestConfig_Defaults tests that config has reasonable defaults
func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		URL:              "redis://localhost:6379/0",
		MaxConns:         10,
		OperationTimeout: 5 * time.Second,
	}

	if cfg.MaxConns != 10 {
		t.Errorf("Expected MaxConns=10, got %d", cfg.MaxConns)
	}
	if cfg.OperationTimeout != 5*time.Second {
		t.Errorf("Expected OperationTimeout=5s, got %v", cfg.OperationTimeout)
	}
}
