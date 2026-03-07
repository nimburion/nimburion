package cache

import (
	"testing"
	"time"
)

func TestMemcachedAdapterClient_OpContextNoTimeout(t *testing.T) {
	client := &memcachedAdapterClient{}

	ctx, cancel := client.opContext()
	defer cancel()

	if _, ok := ctx.Deadline(); ok {
		t.Fatal("expected no deadline when timeout is not configured")
	}
}

func TestMemcachedAdapterClient_OpContextWithTimeout(t *testing.T) {
	client := &memcachedAdapterClient{timeout: 200 * time.Millisecond}

	ctx, cancel := client.opContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline when timeout is configured")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > 200*time.Millisecond {
		t.Fatalf("unexpected remaining timeout: %v", remaining)
	}
}
