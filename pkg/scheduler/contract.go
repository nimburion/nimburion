package scheduler

import (
	"context"
	"time"
)

// LockLease identifies a distributed lock instance.
type LockLease struct {
	Key      string
	Token    string
	ExpireAt time.Time
}

// LockProvider coordinates singleton execution across multiple scheduler instances.
type LockProvider interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (*LockLease, bool, error)
	Renew(ctx context.Context, lease *LockLease, ttl time.Duration) error
	Release(ctx context.Context, lease *LockLease) error
	HealthCheck(ctx context.Context) error
	Close() error
}
