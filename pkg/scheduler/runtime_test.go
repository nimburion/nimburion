package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type schedulerTestLogger struct{}

func (l *schedulerTestLogger) Debug(string, ...any) {}
func (l *schedulerTestLogger) Info(string, ...any)  {}
func (l *schedulerTestLogger) Warn(string, ...any)  {}
func (l *schedulerTestLogger) Error(string, ...any) {}
func (l *schedulerTestLogger) With(...any) logger.Logger {
	return l
}
func (l *schedulerTestLogger) WithContext(context.Context) logger.Logger {
	return l
}

type fakeJobsRuntime struct {
	mu      sync.Mutex
	records []*jobs.Job
}

func (r *fakeJobsRuntime) Enqueue(_ context.Context, job *jobs.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *job
	copied.Payload = append([]byte(nil), job.Payload...)
	copied.Headers = map[string]string{}
	for key, value := range job.Headers {
		copied.Headers[key] = value
	}
	r.records = append(r.records, &copied)
	return nil
}

func (r *fakeJobsRuntime) Subscribe(context.Context, string, jobs.Handler) error { return nil }
func (r *fakeJobsRuntime) Unsubscribe(string) error                              { return nil }
func (r *fakeJobsRuntime) HealthCheck(context.Context) error                     { return nil }
func (r *fakeJobsRuntime) Close() error                                          { return nil }

func (r *fakeJobsRuntime) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

type fakeLockProvider struct {
	acquireResult bool
	leaseCount    int
	releases      int
}

func (p *fakeLockProvider) Acquire(context.Context, string, time.Duration) (*LockLease, bool, error) {
	if !p.acquireResult {
		return nil, false, nil
	}
	p.leaseCount++
	return &LockLease{
		Key:      "lock",
		Token:    "token",
		ExpireAt: time.Now().UTC().Add(time.Second),
	}, true, nil
}
func (p *fakeLockProvider) Renew(context.Context, *LockLease, time.Duration) error { return nil }
func (p *fakeLockProvider) Release(context.Context, *LockLease) error {
	p.releases++
	return nil
}
func (p *fakeLockProvider) Close() error { return nil }

func TestRuntime_StartDispatchesJobs(t *testing.T) {
	jobRuntime := &fakeJobsRuntime{}
	lockProvider := &fakeLockProvider{acquireResult: true}

	runtime, err := NewRuntime(jobRuntime, lockProvider, &schedulerTestLogger{}, Config{})
	if err != nil {
		t.Fatalf("new scheduler runtime: %v", err)
	}
	if err := runtime.Register(Task{
		Name:     "billing-close-day",
		Schedule: "@every 20ms",
		Queue:    "billing",
		JobName:  "billing.close_day",
		Payload:  []byte(`{"source":"scheduler"}`),
	}); err != nil {
		t.Fatalf("register task: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("runtime start: %v", err)
	}
	if jobRuntime.count() == 0 {
		t.Fatal("expected at least one dispatched job")
	}
	if lockProvider.releases == 0 {
		t.Fatal("expected lock release calls")
	}
}

func TestRuntime_SkipsDispatchWhenLockNotAcquired(t *testing.T) {
	jobRuntime := &fakeJobsRuntime{}
	lockProvider := &fakeLockProvider{acquireResult: false}

	runtime, err := NewRuntime(jobRuntime, lockProvider, &schedulerTestLogger{}, Config{})
	if err != nil {
		t.Fatalf("new scheduler runtime: %v", err)
	}
	if err := runtime.Register(Task{
		Name:     "billing-close-day",
		Schedule: "@every 15ms",
		Queue:    "billing",
		JobName:  "billing.close_day",
		Payload:  []byte(`{"source":"scheduler"}`),
	}); err != nil {
		t.Fatalf("register task: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("runtime start: %v", err)
	}
	if got := jobRuntime.count(); got != 0 {
		t.Fatalf("expected no dispatched job, got %d", got)
	}
}
