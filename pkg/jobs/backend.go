package jobs

import (
	"context"
	"time"
)

const (
	// DefaultLeaseTTL is the default lease duration when reserve does not provide one.
	DefaultLeaseTTL = 30 * time.Second
	// DefaultDLQSuffix is appended to queue names when moving jobs to dead-letter queue.
	DefaultDLQSuffix = ".dlq"
)

const (
	HeaderJobFailureReason = "job_failure_reason"
	HeaderJobFailedAt      = "job_failed_at"
	HeaderJobOriginalQueue = "job_original_queue"
)

// Lease tracks temporary ownership over a reserved job.
type Lease struct {
	JobID    string
	Token    string
	Queue    string
	ExpireAt time.Time
	Attempt  int
}

// Backend defines a reliable jobs backend contract with reserve/ack/nack semantics.
type Backend interface {
	Enqueue(ctx context.Context, job *Job) error
	Reserve(ctx context.Context, queue string, leaseFor time.Duration) (*Job, *Lease, error)
	Ack(ctx context.Context, lease *Lease) error
	Nack(ctx context.Context, lease *Lease, nextRunAt time.Time, reason error) error
	Renew(ctx context.Context, lease *Lease, leaseFor time.Duration) error
	MoveToDLQ(ctx context.Context, lease *Lease, reason error) error
	HealthCheck(ctx context.Context) error
	Close() error
}
