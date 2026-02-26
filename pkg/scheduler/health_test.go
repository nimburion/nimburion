package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/health"
)

type healthLockProvider struct{}

func (p *healthLockProvider) Acquire(context.Context, string, time.Duration) (*LockLease, bool, error) {
	return nil, false, nil
}
func (p *healthLockProvider) Renew(context.Context, *LockLease, time.Duration) error { return nil }
func (p *healthLockProvider) Release(context.Context, *LockLease) error              { return nil }
func (p *healthLockProvider) HealthCheck(context.Context) error                      { return nil }
func (p *healthLockProvider) Close() error                                           { return nil }

func TestNewLockProviderHealthChecker(t *testing.T) {
	checker := NewLockProviderHealthChecker("", &healthLockProvider{}, time.Second)
	if checker.Name() != "scheduler-lock-provider" {
		t.Fatalf("unexpected checker name: %s", checker.Name())
	}
	result := checker.Check(context.Background())
	if result.Status != health.StatusHealthy {
		t.Fatalf("expected healthy result, got %s", result.Status)
	}
}
