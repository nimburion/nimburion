package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
	"go.opentelemetry.io/otel/attribute"
)

const (
	DefaultDispatchTimeout = 10 * time.Second
	DefaultLockTTL         = 30 * time.Second
	minRenewInterval       = 100 * time.Millisecond
	misfireGraceWindow     = 500 * time.Millisecond
)

// Config controls scheduler runtime behavior.
type Config struct {
	DispatchTimeout time.Duration
	DefaultLockTTL  time.Duration
}

func (c *Config) normalize() {
	if c.DispatchTimeout <= 0 {
		c.DispatchTimeout = DefaultDispatchTimeout
	}
	if c.DefaultLockTTL <= 0 {
		c.DefaultLockTTL = DefaultLockTTL
	}
}

// Runtime dispatches scheduled tasks into the jobs runtime with distributed locking.
type Runtime struct {
	jobs jobs.Runtime
	lock LockProvider
	log  logger.Logger

	config Config

	mu      sync.Mutex
	tasks   map[string]Task
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewRuntime creates a distributed scheduler runtime.
func NewRuntime(jobsRuntime jobs.Runtime, lockProvider LockProvider, log logger.Logger, cfg Config) (*Runtime, error) {
	if jobsRuntime == nil {
		return nil, errors.New("jobs runtime is required")
	}
	if lockProvider == nil {
		return nil, errors.New("lock provider is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}

	cfg.normalize()
	return &Runtime{
		jobs:   jobsRuntime,
		lock:   lockProvider,
		log:    log,
		config: cfg,
		tasks:  map[string]Task{},
	}, nil
}

// Register adds a new scheduled task.
func (r *Runtime) Register(task Task) error {
	if err := task.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tasks[task.Name]; exists {
		return fmt.Errorf("task %q is already registered", task.Name)
	}
	r.tasks[task.Name] = task
	return nil
}

// Trigger dispatches one registered task immediately.
func (r *Runtime) Trigger(ctx context.Context, taskName string) error {
	if r == nil {
		return errors.New("scheduler runtime is not initialized")
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	name := strings.TrimSpace(taskName)
	if name == "" {
		return errors.New("task name is required")
	}

	r.mu.Lock()
	task, ok := r.tasks[name]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %q is not registered", name)
	}

	return r.dispatchTask(ctx, task, time.Now().UTC())
}

// Start runs all registered tasks until context cancellation.
func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return errors.New("scheduler runtime is not initialized")
	}
	if ctx == nil {
		return errors.New("context is required")
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return errors.New("scheduler already running")
	}
	runningCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.running = true

	tasks := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		tasks = append(tasks, task)
	}
	r.mu.Unlock()

	if len(tasks) == 0 {
		return errors.New("no scheduler tasks registered")
	}

	for _, task := range tasks {
		r.wg.Add(1)
		go r.runTaskLoop(runningCtx, task)
	}

	<-runningCtx.Done()
	return r.Stop(context.Background())
}

// Stop requests scheduler shutdown and waits for active loops.
func (r *Runtime) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	cancel := r.cancel
	r.cancel = nil
	r.running = false
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	waitCh := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(waitCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waitCh:
		return nil
	}
}

func (r *Runtime) runTaskLoop(ctx context.Context, task Task) {
	defer r.wg.Done()

	for {
		now := time.Now().UTC()
		nextRun, err := task.nextRun(now)
		if err != nil {
			r.log.Error("scheduler task has invalid schedule", "task", task.Name, "error", err)
			return
		}

		wait := time.Until(nextRun)
		if wait < 0 {
			wait = 0
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		dispatchAt := nextRun
		actualNow := time.Now().UTC()
		if actualNow.After(nextRun.Add(misfireGraceWindow)) && task.MisfirePolicy == MisfirePolicySkip {
			r.log.Warn("scheduler skipped misfired run", "task", task.Name, "scheduled_at", nextRun.Format(time.RFC3339Nano), "actual_at", actualNow.Format(time.RFC3339Nano))
			continue
		}
		if actualNow.After(nextRun) && task.MisfirePolicy == MisfirePolicyFireOnce {
			dispatchAt = actualNow
		}

		if err := r.dispatchTask(ctx, task, dispatchAt); err != nil {
			r.log.Error("scheduler dispatch failed", "task", task.Name, "error", err)
		}
	}
}

func (r *Runtime) dispatchTask(ctx context.Context, task Task, runAt time.Time) error {
	incrementSchedulerDispatchInFlight(task.Name)
	defer decrementSchedulerDispatchInFlight(task.Name)

	lockTTL := task.LockTTL
	if lockTTL <= 0 {
		lockTTL = r.config.DefaultLockTTL
	}

	lockKey := fmt.Sprintf("scheduler:%s:%d", task.Name, runAt.Unix())
	lease, acquired, err := r.lock.Acquire(ctx, lockKey, lockTTL)
	if err != nil {
		recordSchedulerDispatch(task.Name, "lock_error")
		return fmt.Errorf("acquire lock failed: %w", err)
	}
	if !acquired {
		recordSchedulerDispatch(task.Name, "lock_miss")
		return nil
	}

	releaseErr := func() error {
		if lease == nil {
			return nil
		}
		return r.lock.Release(ctx, lease)
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, r.config.DispatchTimeout)
	defer cancel()
	dispatchCtx, span := tracing.StartMessagingSpan(
		dispatchCtx,
		tracing.SpanOperationMsgPublish,
		tracing.WithMessagingSystem("scheduler"),
		tracing.WithMessagingDestination(task.Queue),
	)
	span.SetAttributes(
		attribute.String("scheduler.task", strings.TrimSpace(task.Name)),
		attribute.String("scheduler.lock_key", lockKey),
		attribute.String("scheduler.run_at", runAt.UTC().Format(time.RFC3339Nano)),
	)
	defer span.End()

	stopRenew, renewDone := r.startLeaseRenewal(dispatchCtx, lease, lockTTL)

	job := &jobs.Job{
		ID:             randomSchedulerID(),
		Name:           strings.TrimSpace(task.JobName),
		Queue:          strings.TrimSpace(task.Queue),
		Payload:        cloneBytes(task.Payload),
		Headers:        cloneHeaders(task.Headers),
		TenantID:       strings.TrimSpace(task.TenantID),
		IdempotencyKey: strings.TrimSpace(task.IdempotencyKey),
		RunAt:          runAt.UTC(),
		Attempt:        0,
		CreatedAt:      time.Now().UTC(),
	}
	if len(job.Payload) == 0 {
		job.Payload = []byte("{}")
	}
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	job.Headers["scheduler_task"] = task.Name
	job.Headers["scheduler_run_at"] = runAt.UTC().Format(time.RFC3339Nano)
	if job.IdempotencyKey == "" {
		job.IdempotencyKey = fmt.Sprintf("%s:%d", task.Name, runAt.Unix())
	}
	span.SetAttributes(
		attribute.String("messaging.message_id", job.ID),
		attribute.Int("messaging.payload_size_bytes", len(job.Payload)),
	)

	enqueueErr := r.jobs.Enqueue(dispatchCtx, job)
	stopRenew()
	renewErr := <-renewDone
	releaseLockErr := releaseErr()
	if renewErr != nil {
		r.log.Warn("scheduler lock renew failed", "task", task.Name, "error", renewErr)
	}
	if enqueueErr != nil {
		recordSchedulerDispatch(task.Name, "enqueue_error")
		tracing.RecordError(span, enqueueErr)
	}
	if renewErr != nil {
		tracing.RecordError(span, renewErr)
	}
	if releaseLockErr != nil {
		tracing.RecordError(span, releaseLockErr)
	}

	dispatchErr := errors.Join(enqueueErr, renewErr, releaseLockErr)
	if dispatchErr != nil {
		recordSchedulerDispatch(task.Name, "error")
		return dispatchErr
	}
	recordSchedulerDispatch(task.Name, "success")
	tracing.RecordSuccess(span)
	return nil
}

func (r *Runtime) startLeaseRenewal(ctx context.Context, lease *LockLease, ttl time.Duration) (func(), <-chan error) {
	done := make(chan error, 1)
	if lease == nil || ttl <= 0 {
		done <- nil
		close(done)
		return func() {}, done
	}

	renewCtx, cancel := context.WithCancel(ctx)
	interval := ttl / 2
	if interval <= 0 {
		interval = ttl
	}
	if interval < minRenewInterval {
		interval = minRenewInterval
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				done <- nil
				close(done)
				return
			case <-ticker.C:
				if err := r.lock.Renew(renewCtx, lease, ttl); err != nil {
					recordSchedulerLockRenew(taskNameFromLockKey(lease.Key), "error")
					done <- fmt.Errorf("renew lock failed: %w", err)
					close(done)
					return
				}
				recordSchedulerLockRenew(taskNameFromLockKey(lease.Key), "success")
			}
		}
	}()

	return cancel, done
}

func randomSchedulerID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("sched-%d", time.Now().UnixNano())
	}
	return "sched-" + hex.EncodeToString(raw)
}

func cloneBytes(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}
	out := make([]byte, len(input))
	copy(out, input)
	return out
}

func cloneHeaders(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func taskNameFromLockKey(lockKey string) string {
	trimmed := strings.TrimSpace(lockKey)
	if trimmed == "" {
		return "unknown"
	}
	if !strings.HasPrefix(trimmed, "scheduler:") {
		return trimmed
	}
	payload := strings.TrimPrefix(trimmed, "scheduler:")
	lastSeparator := strings.LastIndex(payload, ":")
	if lastSeparator <= 0 {
		return payload
	}
	return payload[:lastSeparator]
}
