package sse

import "testing"

func TestNewRedisStore_ValidationAndDefaults(t *testing.T) {
	if _, err := NewRedisStore(RedisStoreConfig{}); err == nil {
		t.Fatal("expected error for empty redis url")
	}

	store, err := NewRedisStore(RedisStoreConfig{URL: "redis://localhost:6379/0"})
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	if store.prefix != "sse:history" {
		t.Fatalf("expected default prefix, got %q", store.prefix)
	}
	if store.maxSize != 256 {
		t.Fatalf("expected default maxSize 256, got %d", store.maxSize)
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
