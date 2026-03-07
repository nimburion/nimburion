package sse

import (
	"strings"
	"testing"
)

func TestNewRedisStore_ValidationAndConnectivity(t *testing.T) {
	if _, err := NewRedisStore(RedisStoreConfig{}); err == nil {
		t.Fatal("expected error for empty redis url")
	}

	_, err := NewRedisStore(RedisStoreConfig{URL: "redis://localhost:6379/0"})
	if err == nil {
		t.Fatal("expected connection validation error")
	}
	if !strings.Contains(err.Error(), "failed to ping redis") {
		t.Fatalf("expected ping error, got %v", err)
	}
}

func TestMin(t *testing.T) {
	if got := min(2, 3); got != 2 {
		t.Fatalf("min(2,3) = %d, want 2", got)
	}
	if got := min(5, 1); got != 1 {
		t.Fatalf("min(5,1) = %d, want 1", got)
	}
}
