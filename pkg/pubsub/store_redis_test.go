package pubsub

import (
	"testing"
)

func TestNewRedisStore_RequiresURL(t *testing.T) {
	_, err := NewRedisStore(RedisStoreConfig{}, nil)
	if err == nil {
		t.Fatal("expected error when redis URL is missing")
	}
}

func TestRedisStore_CloseNilSafe(t *testing.T) {
	var store *RedisStore
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
