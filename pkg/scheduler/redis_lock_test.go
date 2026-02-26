package scheduler

import (
	"testing"
	"time"
)

func TestRedisLockProviderConfigNormalize(t *testing.T) {
	cfg := &RedisLockProviderConfig{}
	cfg.normalize()
	
	if cfg.Prefix != "nimburion:scheduler:lock" {
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
