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
)

const (
	DefaultDispatchTimeout = 10 * time.Second
	DefaultLockTTL         = 30 * time.Second
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

	now := time.Now().UTC()
	for {
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

		if err := r.dispatchTask(ctx, task, nextRun); err != nil {
			r.log.Error("scheduler dispatch failed", "task", task.Name, "error", err)
		}

		now = nextRun.Add(time.Second)
	}
}

func (r *Runtime) dispatchTask(ctx context.Context, task Task, runAt time.Time) error {
	lockTTL := task.LockTTL
	if lockTTL <= 0 {
		lockTTL = r.config.DefaultLockTTL
	}

	lockKey := fmt.Sprintf("scheduler:%s:%d", task.Name, runAt.Unix())
	lease, acquired, err := r.lock.Acquire(ctx, lockKey, lockTTL)
	if err != nil {
		return fmt.Errorf("acquire lock failed: %w", err)
	}
	if !acquired {
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
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	job.Headers["scheduler_task"] = task.Name
	job.Headers["scheduler_run_at"] = runAt.UTC().Format(time.RFC3339Nano)
	if job.IdempotencyKey == "" {
		job.IdempotencyKey = fmt.Sprintf("%s:%d", task.Name, runAt.Unix())
	}

	enqueueErr := r.jobs.Enqueue(dispatchCtx, job)
	releaseLockErr := releaseErr()
	if enqueueErr != nil || releaseLockErr != nil {
		return errors.Join(enqueueErr, releaseLockErr)
	}
	return nil
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
