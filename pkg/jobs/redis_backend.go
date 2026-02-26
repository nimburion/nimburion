package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

var (
	redisReserveScript = redis.NewScript(`
local delayed = KEYS[1]
local ready = KEYS[2]
local leasePrefix = ARGV[1]
local nowMs = tonumber(ARGV[2])
local transferBatch = tonumber(ARGV[3])
local leaseMs = tonumber(ARGV[4])
local token = ARGV[5]

local due = redis.call("ZRANGEBYSCORE", delayed, "-inf", nowMs, "LIMIT", 0, transferBatch)
for _, payload in ipairs(due) do
  redis.call("RPUSH", ready, payload)
  redis.call("ZREM", delayed, payload)
end

local payload = redis.call("LPOP", ready)
if not payload then
  return nil
end

redis.call("SET", leasePrefix .. token, payload, "PX", leaseMs)
return payload
`)

	redisGetAndDeleteScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value then
  return nil
end
redis.call("DEL", KEYS[1])
return value
`)

	redisTransitionLeaseScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current then
  return 0
end
if current ~= ARGV[1] then
  return -1
end

redis.call("DEL", KEYS[1])

local encoded = ARGV[2]
local runAtMs = tonumber(ARGV[3])
local nowMs = tonumber(ARGV[4])
if runAtMs <= nowMs then
  redis.call("RPUSH", KEYS[2], encoded)
else
  redis.call("ZADD", KEYS[3], runAtMs, encoded)
end
return 1
`)
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
	var enqueueErr error
	if !jobCopy.RunAt.After(now) {
		enqueueErr = b.client.RPush(opCtx, b.readyKey(jobCopy.Queue), string(encoded)).Err()
	} else {
		enqueueErr = b.client.ZAdd(opCtx, b.delayedKey(jobCopy.Queue), redis.Z{
			Score:  float64(jobCopy.RunAt.UnixMilli()),
			Member: string(encoded),
		}).Err()
	}
	if enqueueErr != nil {
		return enqueueErr
	}
	recordJobEnqueued("redis", jobCopy)
	return nil
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
	leaseMilliseconds := leaseFor.Milliseconds()
	if leaseMilliseconds <= 0 {
		leaseMilliseconds = 1
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		token := randomToken()
		now := time.Now().UTC()
		opCtx, cancel := b.operationContext(ctx)
		result, reserveErr := redisReserveScript.Run(
			opCtx,
			b.client,
			[]string{b.delayedKey(queue), b.readyKey(queue)},
			b.leaseKeyPrefix(),
			now.UnixMilli(),
			b.config.TransferBatch,
			leaseMilliseconds,
			token,
		).Result()
		cancel()
		if reserveErr != nil && !errors.Is(reserveErr, redis.Nil) {
			return nil, nil, reserveErr
		}
		if errors.Is(reserveErr, redis.Nil) {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(b.config.PollInterval):
				continue
			}
		}
		raw, ok := result.(string)
		if !ok || strings.TrimSpace(raw) == "" {
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
			_ = b.Ack(ctx, &Lease{Token: token})
			continue
		}
		if envelope.Job == nil {
			_ = b.Ack(ctx, &Lease{Token: token})
			continue
		}
		if strings.TrimSpace(envelope.Job.Queue) == "" {
			envelope.Job.Queue = queue
		}
		if err := envelope.Job.Validate(); err != nil {
			b.log.Warn("discarding invalid queued job", "queue", queue, "error", err)
			_ = b.Ack(ctx, &Lease{Token: token})
			continue
		}

		lease := &Lease{
			JobID:    strings.TrimSpace(envelope.Job.ID),
			Token:    token,
			Queue:    queue,
			ExpireAt: now.Add(leaseFor),
			Attempt:  envelope.Job.Attempt,
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
	_, err := redisGetAndDeleteScript.Run(opCtx, b.client, []string{b.leaseKey(strings.TrimSpace(lease.Token))}).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}

// Nack schedules the leased job for retry.
func (b *RedisBackend) Nack(ctx context.Context, lease *Lease, nextRunAt time.Time, reason error) error {
	rawLeasePayload, job, err := b.readLeasedJob(ctx, lease)
	if err != nil {
		return err
	}
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
	encodedJob, err := json.Marshal(redisJobEnvelope{Job: job})
	if err != nil {
		return fmt.Errorf("marshal retry job failed: %w", err)
	}
	if err := b.transitionLeaseToQueue(ctx, lease, rawLeasePayload, string(encodedJob), strings.TrimSpace(job.Queue), job.RunAt); err != nil {
		return err
	}
	recordJobEnqueued("redis", job)
	return nil
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
	expireSet, err := b.client.PExpire(opCtx, b.leaseKey(strings.TrimSpace(lease.Token)), leaseFor).Result()
	if err != nil {
		return err
	}
	if !expireSet {
		return errors.New("lease not found")
	}
	return nil
}

// MoveToDLQ routes the leased job to dead-letter queue and stores DLQ entry metadata.
func (b *RedisBackend) MoveToDLQ(ctx context.Context, lease *Lease, reason error) error {
	rawLeasePayload, job, err := b.readLeasedJob(ctx, lease)
	if err != nil {
		return err
	}
	originalQueue := strings.TrimSpace(job.Queue)
	if originalQueue == "" && lease != nil {
		originalQueue = strings.TrimSpace(lease.Queue)
	}
	job.Queue = originalQueue + b.config.DLQSuffix
	if job.Headers == nil {
		job.Headers = map[string]string{}
	}
	job.Headers[HeaderJobOriginalQueue] = originalQueue
	job.Headers[HeaderJobFailedAt] = time.Now().UTC().Format(time.RFC3339Nano)
	if reason != nil {
		job.Headers[HeaderJobFailureReason] = reason.Error()
	}

	encodedJob, err := json.Marshal(redisJobEnvelope{Job: job})
	if err != nil {
		return fmt.Errorf("marshal dlq job failed: %w", err)
	}
	if err := b.transitionLeaseToQueue(ctx, lease, rawLeasePayload, string(encodedJob), strings.TrimSpace(job.Queue), time.Now().UTC()); err != nil {
		return err
	}
	recordJobEnqueued("redis", job)

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

func (b *RedisBackend) readLeasedJob(ctx context.Context, lease *Lease) (string, *Job, error) {
	if err := b.ensureOpen(); err != nil {
		return "", nil, err
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		return "", nil, errors.New("lease token is required")
	}
	token := strings.TrimSpace(lease.Token)

	opCtx, cancel := b.operationContext(ctx)
	raw, err := b.client.Get(opCtx, b.leaseKey(token)).Result()
	cancel()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil, errors.New("lease not found")
		}
		return "", nil, err
	}

	var envelope redisJobEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return "", nil, fmt.Errorf("decode lease payload failed: %w", err)
	}
	if envelope.Job == nil {
		return "", nil, errors.New("lease payload does not contain a job")
	}
	if strings.TrimSpace(envelope.Job.Queue) == "" && lease != nil {
		envelope.Job.Queue = strings.TrimSpace(lease.Queue)
	}
	if err := envelope.Job.Validate(); err != nil {
		return "", nil, err
	}

	return raw, cloneJob(envelope.Job), nil
}

func (b *RedisBackend) transitionLeaseToQueue(
	ctx context.Context,
	lease *Lease,
	expectedLeasePayload string,
	nextEncodedPayload string,
	queue string,
	runAt time.Time,
) error {
	if err := b.ensureOpen(); err != nil {
		return err
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		return errors.New("lease token is required")
	}
	queue = strings.TrimSpace(queue)
	if queue == "" {
		return errors.New("queue is required")
	}
	if strings.TrimSpace(nextEncodedPayload) == "" {
		return errors.New("next payload is required")
	}
	if strings.TrimSpace(expectedLeasePayload) == "" {
		return errors.New("expected lease payload is required")
	}

	runAtUTC := runAt.UTC()
	if runAtUTC.IsZero() {
		runAtUTC = time.Now().UTC()
	}
	now := time.Now().UTC()

	opCtx, cancel := b.operationContext(ctx)
	transitionResult, err := redisTransitionLeaseScript.Run(
		opCtx,
		b.client,
		[]string{
			b.leaseKey(strings.TrimSpace(lease.Token)),
			b.readyKey(queue),
			b.delayedKey(queue),
		},
		expectedLeasePayload,
		nextEncodedPayload,
		runAtUTC.UnixMilli(),
		now.UnixMilli(),
	).Int()
	cancel()
	if err != nil {
		return err
	}
	switch transitionResult {
	case 1:
		return nil
	case 0:
		return errors.New("lease not found")
	case -1:
		return errors.New("lease payload changed while transitioning")
	default:
		return fmt.Errorf("invalid lease transition result: %d", transitionResult)
	}
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

func (b *RedisBackend) leaseKeyPrefix() string {
	return b.prefix() + ":lease:"
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
