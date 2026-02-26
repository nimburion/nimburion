package jobs

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
	"github.com/nimburion/nimburion/pkg/resilience"
	"go.opentelemetry.io/otel/attribute"
)

const (
	DefaultWorkerReserveTimeout = time.Second
	DefaultWorkerStopTimeout    = 10 * time.Second

	DefaultWorkerMaxAttempts    = 5
	DefaultWorkerInitialBackoff = time.Second
	DefaultWorkerMaxBackoff     = 60 * time.Second
	DefaultWorkerAttemptTimeout = 30 * time.Second

	minWorkerLeaseRenewInterval = 100 * time.Millisecond
)

// RetryPolicy controls retry behavior for failed jobs.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	AttemptTimeout time.Duration
}

func (c *RetryPolicy) normalize() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultWorkerMaxAttempts
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultWorkerInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultWorkerMaxBackoff
	}
	if c.AttemptTimeout <= 0 {
		c.AttemptTimeout = DefaultWorkerAttemptTimeout
	}
}

// DLQPolicy controls dead-letter queue behavior.
type DLQPolicy struct {
	Enabled     bool
	QueueSuffix string
}

func (c *DLQPolicy) normalize() {
	if strings.TrimSpace(c.QueueSuffix) == "" {
		c.QueueSuffix = DefaultDLQSuffix
	}
}

// WorkerConfig configures worker lifecycle and concurrency.
type WorkerConfig struct {
	Queues         []string
	Concurrency    int
	LeaseTTL       time.Duration
	ReserveTimeout time.Duration
	StopTimeout    time.Duration
	Retry          RetryPolicy
	DLQ            DLQPolicy
}

func (c *WorkerConfig) normalize() {
	if c.Concurrency <= 0 {
		c.Concurrency = 1
	}
	if c.LeaseTTL <= 0 {
		c.LeaseTTL = DefaultLeaseTTL
	}
	if c.ReserveTimeout <= 0 {
		c.ReserveTimeout = DefaultWorkerReserveTimeout
	}
	if c.StopTimeout <= 0 {
		c.StopTimeout = DefaultWorkerStopTimeout
	}
	c.Retry.normalize()
	c.DLQ.normalize()
}

// Worker defines a background jobs worker lifecycle.
type Worker interface {
	Register(jobName string, handler Handler) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// RuntimeWorker processes jobs from backend queues with retries and DLQ routing.
type RuntimeWorker struct {
	backend Backend
	log     logger.Logger
	config  WorkerConfig

	mu       sync.RWMutex
	handlers map[string]Handler

	lifecycleMu sync.Mutex
	running     bool
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewWorker creates a worker from backend + configuration.
func NewWorker(backend Backend, log logger.Logger, cfg WorkerConfig) (*RuntimeWorker, error) {
	if backend == nil {
		return nil, errors.New("backend is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	cfg.normalize()
	if len(cfg.Queues) == 0 {
		return nil, errors.New("at least one queue is required")
	}

	queues := make([]string, 0, len(cfg.Queues))
	for _, queue := range cfg.Queues {
		trimmed := strings.TrimSpace(queue)
		if trimmed != "" {
			queues = append(queues, trimmed)
		}
	}
	if len(queues) == 0 {
		return nil, errors.New("at least one non-empty queue is required")
	}
	cfg.Queues = queues

	return &RuntimeWorker{
		backend:  backend,
		log:      log,
		config:   cfg,
		handlers: map[string]Handler{},
	}, nil
}

// Register binds a handler to a logical job name.
func (w *RuntimeWorker) Register(jobName string, handler Handler) error {
	if w == nil {
		return errors.New("worker is not initialized")
	}
	jobName = strings.TrimSpace(jobName)
	if jobName == "" {
		return errors.New("job name is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[jobName] = handler
	return nil
}

// Start launches worker loops and blocks until context cancellation.
func (w *RuntimeWorker) Start(ctx context.Context) error {
	if w == nil {
		return errors.New("worker is not initialized")
	}
	if ctx == nil {
		return errors.New("context is required")
	}

	w.lifecycleMu.Lock()
	if w.running {
		w.lifecycleMu.Unlock()
		return errors.New("worker already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running = true
	w.lifecycleMu.Unlock()

	for _, queue := range w.config.Queues {
		for idx := 0; idx < w.config.Concurrency; idx++ {
			w.wg.Add(1)
			go w.runQueueLoop(runCtx, queue)
		}
	}

	<-runCtx.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), w.config.StopTimeout)
	defer stopCancel()
	stopErr := w.Stop(stopCtx)
	if stopErr != nil {
		return stopErr
	}
	return nil
}

// Stop requests graceful shutdown and waits for active workers to finish.
func (w *RuntimeWorker) Stop(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	w.lifecycleMu.Lock()
	if !w.running {
		w.lifecycleMu.Unlock()
		return nil
	}
	cancel := w.cancel
	w.cancel = nil
	w.running = false
	w.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}

	waitCh := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(waitCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waitCh:
		return w.backend.Close()
	}
}

func (w *RuntimeWorker) runQueueLoop(ctx context.Context, queue string) {
	defer w.wg.Done()

	for {
		if ctx.Err() != nil {
			return
		}

		reserveCtx, cancel := context.WithTimeout(ctx, w.config.ReserveTimeout)
		job, lease, err := w.backend.Reserve(reserveCtx, queue, w.config.LeaseTTL)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			w.log.Warn("jobs reserve failed", "queue", queue, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}
		if job == nil || lease == nil {
			continue
		}

		incrementJobInFlight(queue)
		if err := w.process(ctx, job, lease); err != nil {
			w.log.Warn("jobs processing failed", "queue", queue, "job_id", job.ID, "job_name", job.Name, "error", err)
			recordJobProcessed(queue, job.Name, "error")
		}
		decrementJobInFlight(queue)
	}
}

func (w *RuntimeWorker) process(ctx context.Context, job *Job, lease *Lease) error {
	traceCtx, span := tracing.StartMessagingSpan(
		ctx,
		tracing.SpanOperationMsgProcess,
		tracing.WithMessagingSystem("jobs"),
		tracing.WithMessagingDestination(job.Queue),
		tracing.WithMessagingMessageID(job.ID),
		tracing.WithMessagingPayloadSize(len(job.Payload)),
	)
	span.SetAttributes(
		attribute.String("jobs.job_name", strings.TrimSpace(job.Name)),
		attribute.Int("jobs.attempt", job.Attempt),
		attribute.Int("jobs.max_attempts", job.MaxAttempts),
	)
	defer span.End()

	handler, found := w.lookupHandler(job.Name)
	if !found {
		missingHandlerErr := fmt.Errorf("handler not registered for job %q", job.Name)
		tracing.RecordError(span, missingHandlerErr)
		return w.handleFailure(traceCtx, job, lease, missingHandlerErr)
	}

	stopRenew, renewDone := w.startLeaseRenewal(traceCtx, lease)
	execErr := w.executeHandler(traceCtx, job, handler)
	stopRenew()
	renewErr := <-renewDone
	if renewErr != nil {
		if execErr != nil {
			execErr = errors.Join(execErr, renewErr)
		} else {
			execErr = renewErr
		}
	}

	if execErr != nil {
		tracing.RecordError(span, execErr)
		return w.handleFailure(traceCtx, job, lease, execErr)
	}

	if err := w.backend.Ack(traceCtx, lease); err != nil {
		tracing.RecordError(span, err)
		return fmt.Errorf("ack failed: %w", err)
	}
	recordJobProcessed(job.Queue, job.Name, "success")
	tracing.RecordSuccess(span)
	return nil
}

func (w *RuntimeWorker) executeHandler(ctx context.Context, job *Job, handler Handler) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic while handling job: %v; stack=%s", rec, string(debug.Stack()))
		}
	}()

	return resilience.WithTimeout(ctx, w.config.Retry.AttemptTimeout, func(runCtx context.Context) error {
		return handler(runCtx, job)
	})
}

func (w *RuntimeWorker) handleFailure(ctx context.Context, job *Job, lease *Lease, failure error) error {
	maxAttempts := w.config.Retry.MaxAttempts
	if job.MaxAttempts > 0 {
		maxAttempts = job.MaxAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	nextAttempt := job.Attempt + 1
	if nextAttempt < maxAttempts {
		backoff := exponentialBackoff(nextAttempt, w.config.Retry.InitialBackoff, w.config.Retry.MaxBackoff)
		nextRun := time.Now().UTC().Add(backoff)
		if err := w.backend.Nack(ctx, lease, nextRun, failure); err != nil {
			return fmt.Errorf("nack failed: %w", err)
		}
		recordJobRetry(job.Queue, job.Name)
		recordJobProcessed(job.Queue, job.Name, "retry")
		return nil
	}

	if w.config.DLQ.Enabled {
		if err := w.backend.MoveToDLQ(ctx, lease, failure); err != nil {
			return fmt.Errorf("dlq move failed: %w", err)
		}
		recordJobDLQ(job.Queue, job.Name)
		recordJobProcessed(job.Queue, job.Name, "dlq")
		return nil
	}

	if err := w.backend.Ack(ctx, lease); err != nil {
		return fmt.Errorf("ack failed while dropping job: %w", err)
	}
	recordJobProcessed(job.Queue, job.Name, "dropped")
	return fmt.Errorf("job dropped after %d attempts: %w", maxAttempts, failure)
}

func (w *RuntimeWorker) lookupHandler(jobName string) (Handler, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	handler, ok := w.handlers[strings.TrimSpace(jobName)]
	return handler, ok
}

func exponentialBackoff(attempt int, initial, max time.Duration) time.Duration {
	if initial <= 0 {
		initial = DefaultWorkerInitialBackoff
	}
	if max <= 0 {
		max = DefaultWorkerMaxBackoff
	}
	if attempt <= 0 {
		return initial
	}

	backoff := initial
	for idx := 1; idx < attempt; idx++ {
		if backoff >= max/2 {
			return max
		}
		backoff *= 2
	}
	if backoff > max {
		return max
	}
	return backoff
}

func (w *RuntimeWorker) startLeaseRenewal(ctx context.Context, lease *Lease) (func(), <-chan error) {
	done := make(chan error, 1)
	if lease == nil {
		done <- nil
		close(done)
		return func() {}, done
	}

	renewCtx, cancel := context.WithCancel(ctx)
	interval := w.config.LeaseTTL / 2
	if interval <= 0 {
		interval = w.config.LeaseTTL
	}
	if interval < minWorkerLeaseRenewInterval {
		interval = minWorkerLeaseRenewInterval
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
				if err := w.backend.Renew(renewCtx, lease, w.config.LeaseTTL); err != nil {
					done <- fmt.Errorf("renew lease failed: %w", err)
					close(done)
					return
				}
			}
		}
	}()

	return cancel, done
}
