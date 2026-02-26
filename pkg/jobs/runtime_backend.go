package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	defaultRuntimeBackendBufferSize = 128
)

// RuntimeBackendConfig configures the lease-aware backend built on top of a jobs runtime.
type RuntimeBackendConfig struct {
	BufferSize   int
	DLQSuffix    string
	CloseRuntime bool
}

func (c *RuntimeBackendConfig) normalize() {
	if c.BufferSize <= 0 {
		c.BufferSize = defaultRuntimeBackendBufferSize
	}
	if strings.TrimSpace(c.DLQSuffix) == "" {
		c.DLQSuffix = DefaultDLQSuffix
	}
}

type runtimeSubscription struct {
	queue      string
	cancel     context.CancelFunc
	deliveries chan *runtimeDelivery
}

type runtimeDelivery struct {
	job  *Job
	done chan error
	once sync.Once
}

func (d *runtimeDelivery) complete(err error) {
	if d == nil {
		return
	}
	d.once.Do(func() {
		d.done <- err
		close(d.done)
	})
}

type runtimeLeaseState struct {
	lease    *Lease
	delivery *runtimeDelivery
	timer    *time.Timer
}

// RuntimeBackend provides lease/ack/nack semantics by coordinating runtime subscriptions.
// It preserves transport-level retries by blocking subscription callback completion until
// ack/nack/dlq decisions are applied.
type RuntimeBackend struct {
	runtime Runtime
	log     logger.Logger
	config  RuntimeBackendConfig

	mu            sync.Mutex
	subscriptions map[string]*runtimeSubscription
	leases        map[string]*runtimeLeaseState
	closed        bool
}

// NewRuntimeBackend creates a lease-aware backend over an existing jobs runtime.
func NewRuntimeBackend(runtime Runtime, log logger.Logger, cfg RuntimeBackendConfig) (*RuntimeBackend, error) {
	if runtime == nil {
		return nil, errors.New("runtime is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	cfg.normalize()

	return &RuntimeBackend{
		runtime:       runtime,
		log:           log,
		config:        cfg,
		subscriptions: map[string]*runtimeSubscription{},
		leases:        map[string]*runtimeLeaseState{},
	}, nil
}

// Enqueue delegates enqueue to the underlying runtime.
func (b *RuntimeBackend) Enqueue(ctx context.Context, job *Job) error {
	if b == nil || b.runtime == nil {
		return errors.New("runtime backend is not initialized")
	}
	return b.runtime.Enqueue(ctx, job)
}

// Reserve waits for a job from a queue and returns a lease for processing.
func (b *RuntimeBackend) Reserve(ctx context.Context, queue string, leaseFor time.Duration) (*Job, *Lease, error) {
	if b == nil || b.runtime == nil {
		return nil, nil, errors.New("runtime backend is not initialized")
	}
	if ctx == nil {
		return nil, nil, errors.New("context is required")
	}

	queue = strings.TrimSpace(queue)
	if queue == "" {
		return nil, nil, errors.New("queue is required")
	}
	if leaseFor <= 0 {
		leaseFor = DefaultLeaseTTL
	}

	sub, err := b.ensureSubscription(queue)
	if err != nil {
		return nil, nil, err
	}

	var delivery *runtimeDelivery
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case delivery = <-sub.deliveries:
	}
	if delivery == nil || delivery.job == nil {
		return nil, nil, errors.New("received empty delivery")
	}

	token := randomToken()
	lease := &Lease{
		JobID:    strings.TrimSpace(delivery.job.ID),
		Token:    token,
		Queue:    queue,
		ExpireAt: time.Now().UTC().Add(leaseFor),
		Attempt:  delivery.job.Attempt,
	}
	state := &runtimeLeaseState{
		lease:    cloneLease(lease),
		delivery: delivery,
	}

	state.timer = time.AfterFunc(leaseFor, func() {
		b.expireLease(token)
	})

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		state.timer.Stop()
		delivery.complete(errors.New("runtime backend is closed"))
		return nil, nil, errors.New("runtime backend is closed")
	}
	b.leases[token] = state
	b.mu.Unlock()

	return cloneJob(delivery.job), cloneLease(lease), nil
}

// Ack marks a reserved job as successfully processed.
func (b *RuntimeBackend) Ack(ctx context.Context, lease *Lease) error {
	state, err := b.popLease(lease)
	if err != nil {
		return err
	}
	state.delivery.complete(nil)
	return nil
}

// Nack requeues a reserved job for retry.
func (b *RuntimeBackend) Nack(ctx context.Context, lease *Lease, nextRunAt time.Time, reason error) error {
	state, err := b.popLease(lease)
	if err != nil {
		return err
	}

	retryJob := cloneJob(state.delivery.job)
	retryJob.Attempt++
	if retryJob.Headers == nil {
		retryJob.Headers = map[string]string{}
	}
	if reason != nil {
		retryJob.Headers[HeaderJobFailureReason] = reason.Error()
	}
	retryJob.Headers[HeaderJobFailedAt] = time.Now().UTC().Format(time.RFC3339Nano)

	runAt := nextRunAt.UTC()
	if runAt.IsZero() {
		runAt = time.Now().UTC()
	}
	retryJob.RunAt = runAt

	if delay := time.Until(runAt); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			state.delivery.complete(ctx.Err())
			return ctx.Err()
		case <-timer.C:
		}
	}

	if err := b.runtime.Enqueue(ctx, retryJob); err != nil {
		state.delivery.complete(err)
		return fmt.Errorf("enqueue retry job failed: %w", err)
	}

	state.delivery.complete(nil)
	return nil
}

// Renew extends the lease expiry for an in-flight job.
func (b *RuntimeBackend) Renew(ctx context.Context, lease *Lease, leaseFor time.Duration) error {
	if b == nil {
		return errors.New("runtime backend is not initialized")
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		return errors.New("lease token is required")
	}
	if leaseFor <= 0 {
		leaseFor = DefaultLeaseTTL
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	state, ok := b.leases[strings.TrimSpace(lease.Token)]
	if !ok {
		return errors.New("lease not found")
	}
	if state.timer != nil {
		state.timer.Stop()
	}

	expireAt := time.Now().UTC().Add(leaseFor)
	state.lease.ExpireAt = expireAt
	state.timer = time.AfterFunc(leaseFor, func() {
		b.expireLease(strings.TrimSpace(lease.Token))
	})
	return nil
}

// MoveToDLQ forwards a reserved job to dead-letter queue and releases the lease.
func (b *RuntimeBackend) MoveToDLQ(ctx context.Context, lease *Lease, reason error) error {
	state, err := b.popLease(lease)
	if err != nil {
		return err
	}

	dlqJob := cloneJob(state.delivery.job)
	sourceQueue := strings.TrimSpace(dlqJob.Queue)
	if sourceQueue == "" && lease != nil {
		sourceQueue = strings.TrimSpace(lease.Queue)
	}
	dlqJob.Queue = strings.TrimSpace(sourceQueue) + b.config.DLQSuffix
	if dlqJob.Headers == nil {
		dlqJob.Headers = map[string]string{}
	}
	dlqJob.Headers[HeaderJobOriginalQueue] = sourceQueue
	dlqJob.Headers[HeaderJobFailedAt] = time.Now().UTC().Format(time.RFC3339Nano)
	if reason != nil {
		dlqJob.Headers[HeaderJobFailureReason] = reason.Error()
	}

	if err := b.runtime.Enqueue(ctx, dlqJob); err != nil {
		state.delivery.complete(err)
		return fmt.Errorf("enqueue dlq job failed: %w", err)
	}

	state.delivery.complete(nil)
	return nil
}

// HealthCheck verifies backend connectivity through underlying runtime.
func (b *RuntimeBackend) HealthCheck(ctx context.Context) error {
	if b == nil || b.runtime == nil {
		return errors.New("runtime backend is not initialized")
	}
	return b.runtime.HealthCheck(ctx)
}

// Close unsubscribes active queues and releases pending leases.
func (b *RuntimeBackend) Close() error {
	if b == nil {
		return nil
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true

	subs := make([]*runtimeSubscription, 0, len(b.subscriptions))
	for _, sub := range b.subscriptions {
		subs = append(subs, sub)
	}
	b.subscriptions = map[string]*runtimeSubscription{}

	leases := make([]*runtimeLeaseState, 0, len(b.leases))
	for _, lease := range b.leases {
		leases = append(leases, lease)
	}
	b.leases = map[string]*runtimeLeaseState{}
	b.mu.Unlock()

	for _, state := range leases {
		if state.timer != nil {
			state.timer.Stop()
		}
		state.delivery.complete(errors.New("runtime backend closed"))
	}

	var errs []error
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		sub.cancel()
		if err := b.runtime.Unsubscribe(sub.queue); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %q: %w", sub.queue, err))
		}
	}
	if b.config.CloseRuntime {
		if err := b.runtime.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close runtime: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (b *RuntimeBackend) ensureSubscription(queue string) (*runtimeSubscription, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, errors.New("runtime backend is closed")
	}
	if existing, ok := b.subscriptions[queue]; ok {
		b.mu.Unlock()
		return existing, nil
	}

	subCtx, cancel := context.WithCancel(context.Background())
	sub := &runtimeSubscription{
		queue:      queue,
		cancel:     cancel,
		deliveries: make(chan *runtimeDelivery, b.config.BufferSize),
	}
	b.subscriptions[queue] = sub
	b.mu.Unlock()

	err := b.runtime.Subscribe(subCtx, queue, func(handlerCtx context.Context, job *Job) error {
		delivery := &runtimeDelivery{
			job:  cloneJob(job),
			done: make(chan error, 1),
		}
		select {
		case <-subCtx.Done():
			return subCtx.Err()
		case <-handlerCtx.Done():
			return handlerCtx.Err()
		case sub.deliveries <- delivery:
		}

		select {
		case <-subCtx.Done():
			return subCtx.Err()
		case <-handlerCtx.Done():
			return handlerCtx.Err()
		case result := <-delivery.done:
			return result
		}
	})
	if err != nil {
		cancel()
		b.mu.Lock()
		delete(b.subscriptions, queue)
		b.mu.Unlock()
		return nil, fmt.Errorf("subscribe queue %q failed: %w", queue, err)
	}

	return sub, nil
}

func (b *RuntimeBackend) popLease(lease *Lease) (*runtimeLeaseState, error) {
	if b == nil {
		return nil, errors.New("runtime backend is not initialized")
	}
	if lease == nil {
		return nil, errors.New("lease is required")
	}
	token := strings.TrimSpace(lease.Token)
	if token == "" {
		return nil, errors.New("lease token is required")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	state, ok := b.leases[token]
	if !ok {
		return nil, errors.New("lease not found")
	}
	delete(b.leases, token)
	if state.timer != nil {
		state.timer.Stop()
	}
	return state, nil
}

func (b *RuntimeBackend) expireLease(token string) {
	b.mu.Lock()
	state, ok := b.leases[token]
	if !ok {
		b.mu.Unlock()
		return
	}
	delete(b.leases, token)
	b.mu.Unlock()

	state.delivery.complete(errors.New("lease expired"))
	b.log.Warn("jobs lease expired", "token", token, "job_id", state.lease.JobID, "queue", state.lease.Queue)
}

func cloneJob(job *Job) *Job {
	if job == nil {
		return nil
	}
	copyJob := *job
	copyJob.Payload = cloneBytes(job.Payload)
	copyJob.Headers = cloneHeaders(job.Headers)
	return &copyJob
}

func cloneLease(lease *Lease) *Lease {
	if lease == nil {
		return nil
	}
	copyLease := *lease
	return &copyLease
}

func randomToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}
