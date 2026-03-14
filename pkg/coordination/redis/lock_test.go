package redis

import (
	"errors"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/coordination"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestRedisLockProviderConfigNormalize(t *testing.T) {
	cfg := &RedisLockProviderConfig{}
	cfg.normalize()

	if cfg.Prefix != "nimburion:coordination:lock" {
		t.Errorf("expected default prefix, got %s", cfg.Prefix)
	}
	if cfg.OperationTimeout != 3*time.Second {
		t.Errorf("expected default timeout, got %v", cfg.OperationTimeout)
	}
}

func TestRedisLockProviderConfigNormalizeCustom(t *testing.T) {
	cfg := &RedisLockProviderConfig{
		Prefix:           "custom:",
		OperationTimeout: 10 * time.Second,
	}
	cfg.normalize()

	if cfg.Prefix != "custom:" {
		t.Errorf("expected custom prefix, got %s", cfg.Prefix)
	}
	if cfg.OperationTimeout != 10*time.Second {
		t.Errorf("expected custom timeout, got %v", cfg.OperationTimeout)
	}
}

func TestNewRedisLockProvider_TypedValidationError(t *testing.T) {
	_, err := NewRedisLockProvider(RedisLockProviderConfig{}, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, coordination.ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
	var constructorErr *coreerrors.ConstructorError
	if !errors.As(err, &constructorErr) {
		t.Fatalf("expected ConstructorError, got %T", err)
	}
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "argument.coordination.invalid" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}
