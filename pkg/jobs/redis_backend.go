package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisPrefix           = "nimburion:jobs"
	defaultRedisOperationTimeout = 5 * time.Second
	defaultRedisPollInterval     = 100 * time.Millisecond
	defaultRedisTransferBatch    = 100
)

// RedisBackendConfig configures Redis-backed jobs backend.
type RedisBackendConfig struct {
	URL              string
	Prefix           string
	OperationTimeout time.Duration
	PollInterval     time.Duration
	DLQSuffix        string
	TransferBatch    int
}

func (c *RedisBackendConfig) normalize() {
	if strings.TrimSpace(c.Prefix) == "" {
		c.Prefix = defaultRedisPrefix
	}
	if c.OperationTimeout <= 0 {
		c.OperationTimeout = defaultRedisOperationTimeout
	}
	if c.PollInterval <= 0 {
		c.PollInterval = defaultRedisPollInterval
	}
	if strings.TrimSpace(c.DLQSuffix) == "" {
		c.DLQSuffix = DefaultDLQSuffix
	}
	if c.TransferBatch <= 0 {
		c.TransferBatch = defaultRedisTransferBatch
	}
}

type redisJobEnvelope struct {
	Job *Job `json:"job"`
}

type redisLeaseRecord struct {
	Token string `json:"token"`
	Queue string `json:"queue"`
	Job   *Job   `json:"job"`
}

type redisDLQRecord struct {
	ID            string    `json:"id"`
	Queue         string    `json:"queue"`
	OriginalQueue string    `json:"original_queue"`
	Job           *Job      `json:"job"`
	Reason        string    `json:"reason"`
	FailedAt      time.Time `json:"failed_at"`
}

// RedisBackend implements jobs Backend with Redis lists/zsets and lease keys.
type RedisBackend struct {
	client *redis.Client
	log    logger.Logger
	config RedisBackendConfig

	mu     sync.RWMutex
	closed bool
}

// NewRedisBackend creates a Redis-backed jobs backend.
func NewRedisBackend(cfg RedisBackendConfig, log logger.Logger) (*RedisBackend, error) {
	if log == nil {
		return nil, errors.New("logger is required")
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("redis url is required")
	}
	cfg.normalize()

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url failed: %w", err)
	}
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis failed: %w", err)
	}

	return &RedisBackend{
		client: client,
		log:    log,
		config: cfg,
	}, nil
}

// Enqueue schedules a job for immediate or delayed execution.
func (b *RedisBackend) Enqueue(ctx context.Context, job *Job) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}
	if ctx == nil {
		return errors.New("context is required")
	}
	if job == nil {
		return errors.New("job is required")
	}
	jobCopy := cloneJob(job)
	if err := jobCopy.Validate(); err != nil {
		return err
	}
	if jobCopy.CreatedAt.IsZero() {
		jobCopy.CreatedAt = time.Now().UTC()
	}
	if jobCopy.RunAt.IsZero() {
		jobCopy.RunAt = jobCopy.CreatedAt
	}

	encoded, err := json.Marshal(redisJobEnvelope{Job: jobCopy})
	if err != nil {
		return fmt.Errorf("marshal job envelope failed: %w", err)
	}

	opCtx, cancel := b.operationContext(ctx)
	defer cancel()

	now := time.Now().UTC()
	if !jobCopy.RunAt.After(now) {
		return b.client.RPush(opCtx, b.readyKey(jobCopy.Queue), string(encoded)).Err()
	}
	return b.client.ZAdd(opCtx, b.delayedKey(jobCopy.Queue), redis.Z{
		Score:  float64(jobCopy.RunAt.UnixMilli()),
		Member: string(encoded),
	}).Err()
}

// Reserve returns the next available job and a lease token.
func (b *RedisBackend) Reserve(ctx context.Context, queue string, leaseFor time.Duration) (*Job, *Lease, error) {
	if err := b.ensureOpen(); err != nil {
		return nil, nil, err
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

	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		if err := b.moveDueToReady(ctx, queue, time.Now().UTC()); err != nil {
			return nil, nil, err
		}

		opCtx, cancel := b.operationContext(ctx)
		raw, popErr := b.client.LPop(opCtx, b.readyKey(queue)).Result()
		cancel()
		if popErr != nil && !errors.Is(popErr, redis.Nil) {
			return nil, nil, popErr
		}
		if errors.Is(popErr, redis.Nil) {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(b.config.PollInterval):
				continue
			}
		}

		var envelope redisJobEnvelope
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			b.log.Warn("discarding malformed queued job payload", "queue", queue, "error", err)
			continue
		}
		if envelope.Job == nil {
			continue
		}
		if strings.TrimSpace(envelope.Job.Queue) == "" {
			envelope.Job.Queue = queue
		}
		if err := envelope.Job.Validate(); err != nil {
			b.log.Warn("discarding invalid queued job", "queue", queue, "error", err)
			continue
		}

		lease := &Lease{
			JobID:    strings.TrimSpace(envelope.Job.ID),
			Token:    randomToken(),
			Queue:    queue,
			ExpireAt: time.Now().UTC().Add(leaseFor),
			Attempt:  envelope.Job.Attempt,
		}
		record := redisLeaseRecord{
			Token: lease.Token,
			Queue: queue,
			Job:   cloneJob(envelope.Job),
		}
		encodedRecord, err := json.Marshal(record)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal lease record failed: %w", err)
		}

		opCtx, cancel = b.operationContext(ctx)
		setErr := b.client.Set(opCtx, b.leaseKey(lease.Token), string(encodedRecord), leaseFor).Err()
		cancel()
		if setErr != nil {
			_ = b.Enqueue(ctx, envelope.Job)
			return nil, nil, fmt.Errorf("persist lease failed: %w", setErr)
		}
		return cloneJob(envelope.Job), cloneLease(lease), nil
	}
}

// Ack confirms job completion and releases the lease.
func (b *RedisBackend) Ack(ctx context.Context, lease *Lease) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		return errors.New("lease token is required")
	}
	opCtx, cancel := b.operationContext(ctx)
	defer cancel()
	return b.client.Del(opCtx, b.leaseKey(strings.TrimSpace(lease.Token))).Err()
}

// Nack schedules the leased job for retry.
func (b *RedisBackend) Nack(ctx context.Context, lease *Lease, nextRunAt time.Time, reason error) error {
	record, err := b.popLeaseRecord(ctx, lease)
	if err != nil {
		return err
	}

	job := cloneJob(record.Job)
	job.Attempt++
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	if reason != nil {
		job.Headers[HeaderJobFailureReason] = reason.Error()
	}
	job.Headers[HeaderJobFailedAt] = time.Now().UTC().Format(time.RFC3339Nano)
	job.RunAt = nextRunAt.UTC()
	if job.RunAt.IsZero() {
		job.RunAt = time.Now().UTC()
	}
	return b.Enqueue(ctx, job)
}

// Renew extends lease expiration.
func (b *RedisBackend) Renew(ctx context.Context, lease *Lease, leaseFor time.Duration) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		return errors.New("lease token is required")
	}
	if leaseFor <= 0 {
		leaseFor = DefaultLeaseTTL
	}
	opCtx, cancel := b.operationContext(ctx)
	defer cancel()
	return b.client.Expire(opCtx, b.leaseKey(strings.TrimSpace(lease.Token)), leaseFor).Err()
}

// MoveToDLQ routes the leased job to dead-letter queue and stores DLQ entry metadata.
func (b *RedisBackend) MoveToDLQ(ctx context.Context, lease *Lease, reason error) error {
	record, err := b.popLeaseRecord(ctx, lease)
	if err != nil {
		return err
	}
	job := cloneJob(record.Job)
	originalQueue := strings.TrimSpace(record.Queue)
	job.Queue = originalQueue + b.config.DLQSuffix
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	job.Headers[HeaderJobOriginalQueue] = originalQueue
	job.Headers[HeaderJobFailedAt] = time.Now().UTC().Format(time.RFC3339Nano)
	if reason != nil {
		job.Headers[HeaderJobFailureReason] = reason.Error()
	}

	if err := b.Enqueue(ctx, job); err != nil {
		return err
	}

	entry := &DLQEntry{
		ID:            randomToken(),
		Queue:         job.Queue,
		OriginalQueue: originalQueue,
		Job:           cloneJob(job),
		Reason:        strings.TrimSpace(job.Headers[HeaderJobFailureReason]),
		FailedAt:      time.Now().UTC(),
	}
	return b.saveDLQEntry(ctx, entry)
}

// ListDLQ lists latest dead-letter records for one original queue.
func (b *RedisBackend) ListDLQ(ctx context.Context, queue string, limit int) ([]*DLQEntry, error) {
	if err := b.ensureOpen(); err != nil {
		return nil, err
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return nil, errors.New("queue is required")
	}
	if limit <= 0 {
		limit = 50
	}

	opCtx, cancel := b.operationContext(ctx)
	ids, err := b.client.ZRevRange(opCtx, b.dlqIndexKey(queue), 0, int64(limit-1)).Result()
	cancel()
	if err != nil {
		return nil, err
	}

	entries := make([]*DLQEntry, 0, len(ids))
	for _, id := range ids {
		opCtx, cancel := b.operationContext(ctx)
		raw, getErr := b.client.Get(opCtx, b.dlqEntryKey(queue, id)).Result()
		cancel()
		if getErr != nil {
			if errors.Is(getErr, redis.Nil) {
				continue
			}
			return nil, getErr
		}
		var record redisDLQRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			continue
		}
		entries = append(entries, &DLQEntry{
			ID:            record.ID,
			Queue:         record.Queue,
			OriginalQueue: record.OriginalQueue,
			Job:           cloneJob(record.Job),
			Reason:        record.Reason,
			FailedAt:      record.FailedAt,
		})
	}
	return entries, nil
}

// ReplayDLQ re-enqueues selected DLQ entries back to original queue.
func (b *RedisBackend) ReplayDLQ(ctx context.Context, queue string, ids []string) (int, error) {
	if err := b.ensureOpen(); err != nil {
		return 0, err
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return 0, errors.New("queue is required")
	}
	if len(ids) == 0 {
		return 0, nil
	}

	replayed := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}

		opCtx, cancel := b.operationContext(ctx)
		raw, err := b.client.Get(opCtx, b.dlqEntryKey(queue, id)).Result()
		cancel()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return replayed, err
		}

		var record redisDLQRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			continue
		}
		job := cloneJob(record.Job)
		job.Queue = record.OriginalQueue
		if job.Headers == nil {
			job.Headers = map[string]string{}
		}
		job.Headers["dlq_replay"] = "true"
		job.Attempt = 0
		job.RunAt = time.Now().UTC()

		if err := b.Enqueue(ctx, job); err != nil {
			return replayed, err
		}

		opCtx, cancel = b.operationContext(ctx)
		_, err = b.client.TxPipelined(opCtx, func(pipe redis.Pipeliner) error {
			pipe.ZRem(opCtx, b.dlqIndexKey(queue), id)
			pipe.Del(opCtx, b.dlqEntryKey(queue, id))
			return nil
		})
		cancel()
		if err != nil {
			return replayed, err
		}
		replayed++
	}

	return replayed, nil
}

// HealthCheck verifies Redis connectivity.
func (b *RedisBackend) HealthCheck(ctx context.Context) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}
	opCtx, cancel := b.operationContext(ctx)
	defer cancel()
	return b.client.Ping(opCtx).Err()
}

// Close closes Redis connections.
func (b *RedisBackend) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	return b.client.Close()
}

func (b *RedisBackend) ensureOpen() error {
	if b == nil || b.client == nil {
		return errors.New("redis backend is not initialized")
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errors.New("redis backend is closed")
	}
	return nil
}

func (b *RedisBackend) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, b.config.OperationTimeout)
}

func (b *RedisBackend) moveDueToReady(ctx context.Context, queue string, now time.Time) error {
	opCtx, cancel := b.operationContext(ctx)
	defer cancel()

	due, err := b.client.ZRangeByScore(opCtx, b.delayedKey(queue), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(now.UnixMilli(), 10),
		Offset: 0,
		Count:  int64(b.config.TransferBatch),
	}).Result()
	if err != nil {
		return err
	}
	if len(due) == 0 {
		return nil
	}

	_, err = b.client.TxPipelined(opCtx, func(pipe redis.Pipeliner) error {
		for _, encoded := range due {
			pipe.RPush(opCtx, b.readyKey(queue), encoded)
			pipe.ZRem(opCtx, b.delayedKey(queue), encoded)
		}
		return nil
	})
	return err
}

func (b *RedisBackend) popLeaseRecord(ctx context.Context, lease *Lease) (*redisLeaseRecord, error) {
	if err := b.ensureOpen(); err != nil {
		return nil, err
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		return nil, errors.New("lease token is required")
	}
	token := strings.TrimSpace(lease.Token)

	opCtx, cancel := b.operationContext(ctx)
	raw, err := b.client.Get(opCtx, b.leaseKey(token)).Result()
	cancel()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("lease not found")
		}
		return nil, err
	}

	var record redisLeaseRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return nil, fmt.Errorf("decode lease record failed: %w", err)
	}
	if strings.TrimSpace(record.Token) == "" {
		record.Token = token
	}

	opCtx, cancel = b.operationContext(ctx)
	delErr := b.client.Del(opCtx, b.leaseKey(token)).Err()
	cancel()
	if delErr != nil {
		return nil, delErr
	}
	return &record, nil
}

func (b *RedisBackend) saveDLQEntry(ctx context.Context, entry *DLQEntry) error {
	if entry == nil {
		return errors.New("dlq entry is required")
	}
	queue := strings.TrimSpace(entry.OriginalQueue)
	if queue == "" {
		return errors.New("dlq original queue is required")
	}
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = randomToken()
	}
	if entry.FailedAt.IsZero() {
		entry.FailedAt = time.Now().UTC()
	}
	record := redisDLQRecord{
		ID:            entry.ID,
		Queue:         entry.Queue,
		OriginalQueue: queue,
		Job:           cloneJob(entry.Job),
		Reason:        entry.Reason,
		FailedAt:      entry.FailedAt.UTC(),
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}

	opCtx, cancel := b.operationContext(ctx)
	_, err = b.client.TxPipelined(opCtx, func(pipe redis.Pipeliner) error {
		pipe.Set(opCtx, b.dlqEntryKey(queue, entry.ID), string(encoded), 0)
		pipe.ZAdd(opCtx, b.dlqIndexKey(queue), redis.Z{
			Score:  float64(entry.FailedAt.UnixMilli()),
			Member: entry.ID,
		})
		return nil
	})
	cancel()
	return err
}

func (b *RedisBackend) readyKey(queue string) string {
	return b.prefix() + ":queue:" + strings.TrimSpace(queue) + ":ready"
}

func (b *RedisBackend) delayedKey(queue string) string {
	return b.prefix() + ":queue:" + strings.TrimSpace(queue) + ":delayed"
}

func (b *RedisBackend) leaseKey(token string) string {
	return b.prefix() + ":lease:" + strings.TrimSpace(token)
}

func (b *RedisBackend) dlqIndexKey(queue string) string {
	return b.prefix() + ":dlq:index:" + strings.TrimSpace(queue)
}

func (b *RedisBackend) dlqEntryKey(queue, id string) string {
	return b.prefix() + ":dlq:entry:" + strings.TrimSpace(queue) + ":" + strings.TrimSpace(id)
}

func (b *RedisBackend) prefix() string {
	return strings.TrimRight(strings.TrimSpace(b.config.Prefix), ":")
}
