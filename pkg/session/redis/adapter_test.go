package redis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/session"
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
	var constructorErr *ConstructorError
	if !errors.As(err, &constructorErr) {
		t.Fatalf("expected ConstructorError, got %T", err)
	}
	if !strings.Contains(err.Error(), "redis URL is required") {
		t.Errorf("expected redis URL validation message, got: %v", err)
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

func TestAdapter_MapGetError_NotFoundMapsToSessionErrNotFound(t *testing.T) {
	adapter := &Adapter{}

	err := adapter.mapGetError("missing", redis.Nil)
	if !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("expected session.ErrNotFound, got %v", err)
	}
}

func TestAdapter_WithOperationTimeout(t *testing.T) {
	t.Run("uses adapter timeout when caller has no deadline", func(t *testing.T) {
		adapter := &Adapter{timeout: 2 * time.Second}

		ctx, cancel := adapter.withOperationTimeout(context.Background())
		defer cancel()

		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected deadline from adapter timeout")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > 2*time.Second {
			t.Fatalf("unexpected remaining timeout: %v", remaining)
		}
	})

	t.Run("preserves caller deadline", func(t *testing.T) {
		adapter := &Adapter{timeout: 2 * time.Second}
		parentCtx, parentCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer parentCancel()

		ctx, cancel := adapter.withOperationTimeout(parentCtx)
		defer cancel()

		parentDeadline, _ := parentCtx.Deadline()
		gotDeadline, _ := ctx.Deadline()
		if !gotDeadline.Equal(parentDeadline) {
			t.Fatalf("expected caller deadline to be preserved, got %v want %v", gotDeadline, parentDeadline)
		}
	})
}
