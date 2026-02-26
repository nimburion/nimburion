package jobs

import (
	"strings"
	"testing"
	"time"
)

func TestRedisBackendConfigNormalize(t *testing.T) {
	cfg := RedisBackendConfig{}
	cfg.normalize()

	if cfg.Prefix == "" {
		t.Fatal("expected default redis prefix")
	}
	if cfg.OperationTimeout <= 0 {
		t.Fatal("expected positive operation timeout")
	}
	if cfg.PollInterval <= 0 {
		t.Fatal("expected positive poll interval")
	}
	if cfg.DLQSuffix == "" {
		t.Fatal("expected dlq suffix default")
	}
	if cfg.TransferBatch <= 0 {
		t.Fatal("expected positive transfer batch")
	}
}

func TestNewRedisBackend_ValidationErrors(t *testing.T) {
	if _, err := NewRedisBackend(RedisBackendConfig{
		URL: "redis://localhost:6379",
	}, nil); err == nil {
		t.Fatal("expected logger validation error")
	}

	_, err := NewRedisBackend(RedisBackendConfig{}, &workerTestLogger{})
	if err == nil || !strings.Contains(err.Error(), "redis url is required") {
		t.Fatalf("expected missing redis url error, got %v", err)
	}

	_, err = NewRedisBackend(RedisBackendConfig{
		URL: "://bad-url",
	}, &workerTestLogger{})
	if err == nil {
		t.Fatal("expected invalid redis url error")
	}
}

func TestRedisBackendKeyBuilders(t *testing.T) {
	backend := &RedisBackend{
		config: RedisBackendConfig{
			Prefix:           "nimburion:jobs:",
			OperationTimeout: time.Second,
		},
	}

	if got := backend.readyKey("payments"); got != "nimburion:jobs:queue:payments:ready" {
		t.Fatalf("unexpected ready key: %s", got)
	}
	if got := backend.delayedKey("payments"); got != "nimburion:jobs:queue:payments:delayed" {
		t.Fatalf("unexpected delayed key: %s", got)
	}
	if got := backend.leaseKey("token-1"); got != "nimburion:jobs:lease:token-1" {
		t.Fatalf("unexpected lease key: %s", got)
	}
	if got := backend.dlqIndexKey("payments"); got != "nimburion:jobs:dlq:index:payments" {
		t.Fatalf("unexpected dlq index key: %s", got)
	}
	if got := backend.dlqEntryKey("payments", "id-1"); got != "nimburion:jobs:dlq:entry:payments:id-1" {
		t.Fatalf("unexpected dlq entry key: %s", got)
	}
}
