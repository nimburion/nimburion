package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type workerTestLogger struct{}

func (l *workerTestLogger) Debug(string, ...any) {}
func (l *workerTestLogger) Info(string, ...any)  {}
func (l *workerTestLogger) Warn(string, ...any)  {}
func (l *workerTestLogger) Error(string, ...any) {}
func (l *workerTestLogger) With(...any) logger.Logger {
	return l
}
func (l *workerTestLogger) WithContext(context.Context) logger.Logger {
	return l
}

type fakeDelivery struct {
	job   *Job
	lease *Lease
}

type fakeNack struct {
	lease     *Lease
	nextRunAt time.Time
	reason    error
}

type fakeBackend struct {
	deliveries chan fakeDelivery

	mu         sync.Mutex
	acks       []*Lease
	nacks      []fakeNack
	dlqLeases  []*Lease
	renewCalls int
	closeCalls int
}

func newFakeBackend(buffer int) *fakeBackend {
	return &fakeBackend{
		deliveries: make(chan fakeDelivery, buffer),
		acks:       []*Lease{},
		nacks:      []fakeNack{},
		dlqLeases:  []*Lease{},
	}
}

func (b *fakeBackend) Enqueue(context.Context, *Job) error { return nil }

func (b *fakeBackend) Reserve(ctx context.Context, _ string, _ time.Duration) (*Job, *Lease, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case delivery := <-b.deliveries:
		return cloneJob(delivery.job), cloneLease(delivery.lease), nil
	}
}

func (b *fakeBackend) Ack(_ context.Context, lease *Lease) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.acks = append(b.acks, cloneLease(lease))
	return nil
}

func (b *fakeBackend) Nack(_ context.Context, lease *Lease, nextRunAt time.Time, reason error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nacks = append(b.nacks, fakeNack{
		lease:     cloneLease(lease),
		nextRunAt: nextRunAt,
		reason:    reason,
	})
	return nil
}

func (b *fakeBackend) Renew(context.Context, *Lease, time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.renewCalls++
	return nil
}

func (b *fakeBackend) MoveToDLQ(_ context.Context, lease *Lease, _ error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dlqLeases = append(b.dlqLeases, cloneLease(lease))
	return nil
}

func (b *fakeBackend) HealthCheck(context.Context) error { return nil }

func (b *fakeBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeCalls++
	return nil
}

func (b *fakeBackend) push(job *Job) {
	lease := &Lease{
		JobID:    job.ID,
		Token:    job.ID + "-lease",
		Queue:    job.Queue,
		ExpireAt: time.Now().UTC().Add(time.Minute),
		Attempt:  job.Attempt,
	}
	b.deliveries <- fakeDelivery{job: cloneJob(job), lease: lease}
}

func (b *fakeBackend) snapshot() (acks int, nacks int, dlqs int, closeCalls int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.acks), len(b.nacks), len(b.dlqLeases), b.closeCalls
}

func (b *fakeBackend) renewCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.renewCalls
}

func TestWorker_AckOnSuccess(t *testing.T) {
	backend := newFakeBackend(4)
	worker, err := NewWorker(backend, &workerTestLogger{}, WorkerConfig{
		Queues:      []string{"payments"},
		Concurrency: 1,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	processed := make(chan struct{}, 1)
	if err := worker.Register("invoice.generate", func(context.Context, *Job) error {
		processed <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx)
	}()

	backend.push(&Job{
		ID:      "job-1",
		Name:    "invoice.generate",
		Queue:   "payments",
		Payload: []byte(`{}`),
	})

	select {
	case <-processed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected job to be processed")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}

	acks, nacks, dlqs, _ := backend.snapshot()
	if acks == 0 {
		t.Fatal("expected at least one ack")
	}
	if nacks != 0 {
		t.Fatalf("expected zero nacks, got %d", nacks)
	}
	if dlqs != 0 {
		t.Fatalf("expected zero dlq moves, got %d", dlqs)
	}
}

func TestWorker_RetryThenDLQ(t *testing.T) {
	backend := newFakeBackend(8)
	worker, err := NewWorker(backend, &workerTestLogger{}, WorkerConfig{
		Queues:      []string{"payments"},
		Concurrency: 1,
		Retry: RetryPolicy{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     2 * time.Millisecond,
			AttemptTimeout: time.Second,
		},
		DLQ: DLQPolicy{
			Enabled: true,
		},
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	if err := worker.Register("invoice.generate", func(context.Context, *Job) error {
		return errors.New("boom")
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx)
	}()

	backend.push(&Job{
		ID:          "job-retry",
		Name:        "invoice.generate",
		Queue:       "payments",
		Payload:     []byte(`{}`),
		Attempt:     0,
		MaxAttempts: 3,
	})
	backend.push(&Job{
		ID:          "job-dlq",
		Name:        "invoice.generate",
		Queue:       "payments",
		Payload:     []byte(`{}`),
		Attempt:     2,
		MaxAttempts: 3,
	})

	deadline := time.After(2 * time.Second)
	for {
		_, nacks, dlqs, _ := backend.snapshot()
		if nacks >= 1 && dlqs >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected retry and dlq actions, got nacks=%d dlqs=%d", nacks, dlqs)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorker_Concurrency(t *testing.T) {
	backend := newFakeBackend(16)
	worker, err := NewWorker(backend, &workerTestLogger{}, WorkerConfig{
		Queues:      []string{"payments"},
		Concurrency: 3,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	var current int32
	var maxConcurrent int32
	var processed int32
	if err := worker.Register("invoice.generate", func(context.Context, *Job) error {
		active := atomic.AddInt32(&current, 1)
		for {
			existing := atomic.LoadInt32(&maxConcurrent)
			if active <= existing || atomic.CompareAndSwapInt32(&maxConcurrent, existing, active) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&processed, 1)
		atomic.AddInt32(&current, -1)
		return nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx)
	}()

	for idx := 0; idx < 6; idx++ {
		backend.push(&Job{
			ID:      "job-conc-" + time.Now().Add(time.Duration(idx)*time.Millisecond).Format("150405.000000"),
			Name:    "invoice.generate",
			Queue:   "payments",
			Payload: []byte(`{}`),
		})
	}

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&processed) < 6 {
		select {
		case <-deadline:
			t.Fatalf("expected 6 processed jobs, got %d", atomic.LoadInt32(&processed))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}

	if atomic.LoadInt32(&maxConcurrent) < 2 {
		t.Fatalf("expected concurrent processing >=2, got %d", atomic.LoadInt32(&maxConcurrent))
	}
}

func TestWorker_RenewsLeaseDuringLongProcessing(t *testing.T) {
	backend := newFakeBackend(4)
	worker, err := NewWorker(backend, &workerTestLogger{}, WorkerConfig{
		Queues:      []string{"payments"},
		Concurrency: 1,
		LeaseTTL:    80 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	processed := make(chan struct{}, 1)
	if err := worker.Register("invoice.generate", func(context.Context, *Job) error {
		time.Sleep(220 * time.Millisecond)
		processed <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Start(ctx)
	}()

	backend.push(&Job{
		ID:      "job-renew",
		Name:    "invoice.generate",
		Queue:   "payments",
		Payload: []byte(`{}`),
	})

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("expected job to be processed")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}

	if backend.renewCount() == 0 {
		t.Fatal("expected at least one lease renewal during long processing")
	}
}
