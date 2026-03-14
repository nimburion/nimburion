package pubsub

import (
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestNewRedisStore_RequiresURL(t *testing.T) {
	_, err := NewRedisStore(RedisStoreConfig{}, nil)
	if err == nil {
		t.Fatal("expected error when redis URL is missing")
	}
	var constructorErr *coreerrors.ConstructorError
	if !errors.As(err, &constructorErr) {
		t.Fatalf("expected ConstructorError, got %T", err)
	}
}

func TestRedisStore_CloseNilSafe(t *testing.T) {
	var store *RedisStore
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
