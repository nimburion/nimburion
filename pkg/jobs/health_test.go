package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/health"
)

type healthTestBackend struct {
	healthErr error
}

func (b *healthTestBackend) Enqueue(context.Context, *Job) error { return nil }
func (b *healthTestBackend) Reserve(context.Context, string, time.Duration) (*Job, *Lease, error) {
	return nil, nil, nil
}
func (b *healthTestBackend) Ack(context.Context, *Lease) error                    { return nil }
func (b *healthTestBackend) Nack(context.Context, *Lease, time.Time, error) error { return nil }
func (b *healthTestBackend) Renew(context.Context, *Lease, time.Duration) error   { return nil }
func (b *healthTestBackend) MoveToDLQ(context.Context, *Lease, error) error       { return nil }
func (b *healthTestBackend) HealthCheck(context.Context) error                    { return b.healthErr }
func (b *healthTestBackend) Close() error                                         { return nil }

type healthTestRuntime struct {
	healthErr error
}

func (r *healthTestRuntime) Enqueue(context.Context, *Job) error              { return nil }
func (r *healthTestRuntime) Subscribe(context.Context, string, Handler) error { return nil }
func (r *healthTestRuntime) Unsubscribe(string) error                         { return nil }
func (r *healthTestRuntime) HealthCheck(context.Context) error                { return r.healthErr }
func (r *healthTestRuntime) Close() error                                     { return nil }

func TestNewBackendHealthChecker(t *testing.T) {
	checker := NewBackendHealthChecker("", &healthTestBackend{}, time.Second)
	if checker.Name() != "jobs-backend" {
		t.Fatalf("unexpected checker name: %s", checker.Name())
	}
	result := checker.Check(context.Background())
	if result.Status != health.StatusHealthy {
		t.Fatalf("expected healthy result, got %s", result.Status)
	}
}

func TestNewRuntimeHealthChecker(t *testing.T) {
	checker := NewRuntimeHealthChecker("runtime-check", &healthTestRuntime{}, time.Second)
	if checker.Name() != "runtime-check" {
		t.Fatalf("unexpected checker name: %s", checker.Name())
	}
	result := checker.Check(context.Background())
	if result.Status != health.StatusHealthy {
		t.Fatalf("expected healthy result, got %s", result.Status)
	}
}
