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
	delay   time.Duration
}

func (r *fakeJobsRuntime) Enqueue(_ context.Context, job *jobs.Job) error {
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
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
	mu            sync.Mutex
	acquireResult bool
	leaseCount    int
	releases      int
	renews        int
}

func (p *fakeLockProvider) Acquire(context.Context, string, time.Duration) (*LockLease, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
func (p *fakeLockProvider) Renew(context.Context, *LockLease, time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.renews++
	return nil
}
func (p *fakeLockProvider) Release(context.Context, *LockLease) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releases++
	return nil
}
func (p *fakeLockProvider) Close() error { return nil }

func (p *fakeLockProvider) counts() (leases int, renews int, releases int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.leaseCount, p.renews, p.releases
}

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
	_, _, releases := lockProvider.counts()
	if releases == 0 {
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

func TestRuntime_TriggerDispatchesSingleTask(t *testing.T) {
	jobRuntime := &fakeJobsRuntime{}
	lockProvider := &fakeLockProvider{acquireResult: true}

	runtime, err := NewRuntime(jobRuntime, lockProvider, &schedulerTestLogger{}, Config{})
	if err != nil {
		t.Fatalf("new scheduler runtime: %v", err)
	}
	if err := runtime.Register(Task{
		Name:     "billing-close-day",
		Schedule: "@every 15m",
		Queue:    "billing",
		JobName:  "billing.close_day",
		Payload:  []byte(`{"source":"scheduler"}`),
	}); err != nil {
		t.Fatalf("register task: %v", err)
	}

	if err := runtime.Trigger(context.Background(), "billing-close-day"); err != nil {
		t.Fatalf("trigger task: %v", err)
	}
	if got := jobRuntime.count(); got != 1 {
		t.Fatalf("expected one dispatched job, got %d", got)
	}
}

func TestRuntime_TriggerRejectsUnknownTask(t *testing.T) {
	jobRuntime := &fakeJobsRuntime{}
	lockProvider := &fakeLockProvider{acquireResult: true}

	runtime, err := NewRuntime(jobRuntime, lockProvider, &schedulerTestLogger{}, Config{})
	if err != nil {
		t.Fatalf("new scheduler runtime: %v", err)
	}
	if err := runtime.Trigger(context.Background(), "missing"); err == nil {
		t.Fatal("expected trigger error for unknown task")
	}
}

func TestRuntime_DispatchRenewsLeaseForLongDispatch(t *testing.T) {
	jobRuntime := &fakeJobsRuntime{delay: 320 * time.Millisecond}
	lockProvider := &fakeLockProvider{acquireResult: true}

	runtime, err := NewRuntime(jobRuntime, lockProvider, &schedulerTestLogger{}, Config{
		DispatchTimeout: time.Second,
		DefaultLockTTL:  150 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new scheduler runtime: %v", err)
	}

	task := Task{
		Name:     "billing-close-day",
		Schedule: "@every 15m",
		Queue:    "billing",
		JobName:  "billing.close_day",
		Payload:  []byte(`{"source":"scheduler"}`),
		LockTTL:  150 * time.Millisecond,
	}
	if err := runtime.dispatchTask(context.Background(), task, time.Now().UTC()); err != nil {
		t.Fatalf("dispatch task: %v", err)
	}

	_, renews, releases := lockProvider.counts()
	if renews == 0 {
		t.Fatal("expected at least one lock renewal during long dispatch")
	}
	if releases == 0 {
		t.Fatal("expected lock release after dispatch")
	}
}
