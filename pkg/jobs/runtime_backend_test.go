package jobs

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type backendTestLogger struct{}

func (l *backendTestLogger) Debug(string, ...any) {}
func (l *backendTestLogger) Info(string, ...any)  {}
func (l *backendTestLogger) Warn(string, ...any)  {}
func (l *backendTestLogger) Error(string, ...any) {}
func (l *backendTestLogger) With(...any) logger.Logger {
	return l
}
func (l *backendTestLogger) WithContext(context.Context) logger.Logger {
	return l
}

type fakeRuntime struct {
	mu           sync.Mutex
	handlers     map[string]Handler
	enqueued     []*Job
	unsubscribed []string
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{
		handlers: map[string]Handler{},
		enqueued: []*Job{},
	}
}

func (r *fakeRuntime) Enqueue(_ context.Context, job *Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enqueued = append(r.enqueued, cloneJob(job))
	return nil
}

func (r *fakeRuntime) Subscribe(_ context.Context, queue string, handler Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[queue] = handler
	return nil
}

func (r *fakeRuntime) Unsubscribe(queue string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unsubscribed = append(r.unsubscribed, queue)
	delete(r.handlers, queue)
	return nil
}

func (r *fakeRuntime) HealthCheck(context.Context) error { return nil }

func (r *fakeRuntime) Close() error { return nil }

func (r *fakeRuntime) emit(queue string, job *Job) <-chan error {
	r.mu.Lock()
	handler := r.handlers[queue]
	r.mu.Unlock()

	out := make(chan error, 1)
	go func() {
		if handler == nil {
			out <- errors.New("handler not found")
			return
		}
		out <- handler(context.Background(), cloneJob(job))
	}()
	return out
}

func (r *fakeRuntime) waitForHandler(t *testing.T, queue string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		r.mu.Lock()
		_, ok := r.handlers[queue]
		r.mu.Unlock()
		if ok {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("handler for queue %q not registered in time", queue)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func (r *fakeRuntime) enqueuedJobs() []*Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Job, 0, len(r.enqueued))
	for _, job := range r.enqueued {
		out = append(out, cloneJob(job))
	}
	return out
}

func TestRuntimeBackend_ReserveAckFlow(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	reserveCh := make(chan struct {
		job   *Job
		lease *Lease
		err   error
	}, 1)
	go func() {
		job, lease, reserveErr := backend.Reserve(context.Background(), "payments", 2*time.Second)
		reserveCh <- struct {
			job   *Job
			lease *Lease
			err   error
		}{job: job, lease: lease, err: reserveErr}
	}()
	runtime.waitForHandler(t, "payments")

	message := &Job{
		ID:      "job-1",
		Name:    "invoice.generate",
		Queue:   "payments",
		Payload: []byte(`{"invoice":"x"}`),
	}
	handlerResult := runtime.emit("payments", message)

	got := <-reserveCh
	if got.err != nil {
		t.Fatalf("reserve returned error: %v", got.err)
	}
	if got.job == nil || got.lease == nil {
		t.Fatal("expected reserved job and lease")
	}

	if err := backend.Ack(context.Background(), got.lease); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if err := <-handlerResult; err != nil {
		t.Fatalf("expected successful callback completion, got %v", err)
	}
}

func TestRuntimeBackend_NackRequeues(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	reserveCh := make(chan *Lease, 1)
	go func() {
		_, lease, _ := backend.Reserve(context.Background(), "payments", 2*time.Second)
		reserveCh <- lease
	}()
	runtime.waitForHandler(t, "payments")

	message := &Job{
		ID:      "job-2",
		Name:    "invoice.generate",
		Queue:   "payments",
		Payload: []byte(`{"invoice":"x"}`),
		Attempt: 1,
	}
	handlerResult := runtime.emit("payments", message)
	lease := <-reserveCh
	if lease == nil {
		t.Fatal("expected lease")
	}

	reason := errors.New("temporary failure")
	if err := backend.Nack(context.Background(), lease, time.Now().UTC(), reason); err != nil {
		t.Fatalf("nack: %v", err)
	}
	if err := <-handlerResult; err != nil {
		t.Fatalf("expected successful callback completion, got %v", err)
	}

	enqueued := runtime.enqueuedJobs()
	if len(enqueued) != 1 {
		t.Fatalf("expected one retry job, got %d", len(enqueued))
	}
	if enqueued[0].Attempt != 2 {
		t.Fatalf("expected attempt=2, got %d", enqueued[0].Attempt)
	}
	if enqueued[0].Headers[HeaderJobFailureReason] != reason.Error() {
		t.Fatalf("expected failure reason header, got %q", enqueued[0].Headers[HeaderJobFailureReason])
	}
}

func TestRuntimeBackend_MoveToDLQ(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{DLQSuffix: ".dead"})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	reserveCh := make(chan *Lease, 1)
	go func() {
		_, lease, _ := backend.Reserve(context.Background(), "emails", 2*time.Second)
		reserveCh <- lease
	}()
	runtime.waitForHandler(t, "emails")

	message := &Job{
		ID:      "job-3",
		Name:    "email.send",
		Queue:   "emails",
		Payload: []byte(`{"to":"x@y"}`),
	}
	handlerResult := runtime.emit("emails", message)
	lease := <-reserveCh
	if lease == nil {
		t.Fatal("expected lease")
	}

	if err := backend.MoveToDLQ(context.Background(), lease, errors.New("bad payload")); err != nil {
		t.Fatalf("move to dlq: %v", err)
	}
	if err := <-handlerResult; err != nil {
		t.Fatalf("expected successful callback completion, got %v", err)
	}

	enqueued := runtime.enqueuedJobs()
	if len(enqueued) != 1 {
		t.Fatalf("expected one dlq job, got %d", len(enqueued))
	}
	if enqueued[0].Queue != "emails.dead" {
		t.Fatalf("expected dlq queue emails.dead, got %q", enqueued[0].Queue)
	}
	if enqueued[0].Headers[HeaderJobOriginalQueue] != "emails" {
		t.Fatalf("expected original queue header, got %q", enqueued[0].Headers[HeaderJobOriginalQueue])
	}
}

func TestRuntimeBackend_LeaseExpires(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	reserveCh := make(chan *Lease, 1)
	go func() {
		_, lease, _ := backend.Reserve(context.Background(), "emails", 15*time.Millisecond)
		reserveCh <- lease
	}()
	runtime.waitForHandler(t, "emails")

	handlerResult := runtime.emit("emails", &Job{
		ID:      "job-4",
		Name:    "email.send",
		Queue:   "emails",
		Payload: []byte(`x`),
	})

	lease := <-reserveCh
	if lease == nil {
		t.Fatal("expected lease")
	}

	select {
	case err := <-handlerResult:
		if err == nil || !strings.Contains(err.Error(), "lease expired") {
			t.Fatalf("expected lease expired error, got %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected callback completion from lease expiry")
	}
}

func TestRuntimeBackend_CloseReleasesPendingLeases(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	reserveCh := make(chan *Lease, 1)
	go func() {
		_, lease, _ := backend.Reserve(context.Background(), "emails", 2*time.Second)
		reserveCh <- lease
	}()
	runtime.waitForHandler(t, "emails")

	handlerResult := runtime.emit("emails", &Job{
		ID:      "job-5",
		Name:    "email.send",
		Queue:   "emails",
		Payload: []byte(`x`),
	})
	lease := <-reserveCh
	if lease == nil {
		t.Fatal("expected lease")
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("close backend: %v", err)
	}

	select {
	case err := <-handlerResult:
		if err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("expected closed error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected callback to be released on close")
	}
}

func TestRuntimeBackend_RenewMissingLeaseReturnsTypedNotFound(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	err = backend.Renew(context.Background(), &Lease{Token: "missing"}, time.Second)
	if err == nil {
		t.Fatal("expected renew error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRuntimeBackend_AckNilLeaseReturnsTypedValidationError(t *testing.T) {
	runtime := newFakeRuntime()
	backend, err := NewRuntimeBackend(runtime, &backendTestLogger{}, RuntimeBackendConfig{})
	if err != nil {
		t.Fatalf("new backend: %v", err)
	}

	err = backend.Ack(context.Background(), nil)
	if err == nil {
		t.Fatal("expected ack error")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}
