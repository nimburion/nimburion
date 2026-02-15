package eventbus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DefaultOutboxPollInterval   = time.Second
	DefaultOutboxBatchSize      = 100
	DefaultOutboxCleanupEvery   = time.Minute
	DefaultOutboxCleanupRetain  = 7 * 24 * time.Hour
	DefaultOutboxInitialBackoff = time.Second
	DefaultOutboxMaxBackoff     = time.Minute
)

// CreateOutboxTablePostgres defines a reference PostgreSQL schema for transactional outbox.
const CreateOutboxTablePostgres = `
CREATE TABLE IF NOT EXISTS outbox (
  id TEXT PRIMARY KEY,
  topic TEXT NOT NULL,
  message_key TEXT NOT NULL,
  payload BYTEA NOT NULL,
  headers JSONB NOT NULL DEFAULT '{}'::jsonb,
  content_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published BOOLEAN NOT NULL DEFAULT FALSE,
  published_at TIMESTAMPTZ,
  retry_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox (published, available_at);
CREATE INDEX IF NOT EXISTS idx_outbox_created_at ON outbox (created_at);
`

// OutboxEntry is a queued message to publish through the event bus.
type OutboxEntry struct {
	ID          string
	Topic       string
	Message     *Message
	CreatedAt   time.Time
	AvailableAt time.Time
	Published   bool
	PublishedAt *time.Time
	RetryCount  int
	LastError   string
}

// Validate checks that an outbox entry is complete.
func (e *OutboxEntry) Validate() error {
	if e == nil {
		return errors.New("outbox entry is nil")
	}
	if e.ID == "" {
		return errors.New("outbox entry id is required")
	}
	if e.Topic == "" {
		return errors.New("outbox entry topic is required")
	}
	if e.Message == nil {
		return errors.New("outbox entry message is required")
	}
	if len(e.Message.Value) == 0 {
		return errors.New("outbox entry message value is required")
	}
	return nil
}

// OutboxStore is the persistence contract for outbox entries.
type OutboxStore interface {
	Insert(ctx context.Context, entry *OutboxEntry) error
	FetchPending(ctx context.Context, limit int, now time.Time) ([]*OutboxEntry, error)
	MarkPublished(ctx context.Context, id string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id string, retryCount int, nextAttemptAt time.Time, reason string) error
	CleanupPublishedBefore(ctx context.Context, before time.Time, limit int) (int, error)
	PendingCount(ctx context.Context, now time.Time) (int, error)
	OldestPendingAgeSeconds(ctx context.Context, now time.Time) (float64, error)
}

// OutboxWriter writes outbox entries within a transaction boundary.
type OutboxWriter interface {
	Insert(ctx context.Context, entry *OutboxEntry) error
}

// TransactionalOutboxExecutor executes a function inside a transaction and
// commits only when the function returns nil.
type TransactionalOutboxExecutor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context, writer OutboxWriter) error) error
}

// ExecuteTransactionalOutbox applies business changes and outbox insert
// atomically through the provided transaction executor.
func ExecuteTransactionalOutbox(
	ctx context.Context,
	executor TransactionalOutboxExecutor,
	entry *OutboxEntry,
	businessFn func(ctx context.Context) error,
) error {
	if executor == nil {
		return errors.New("transaction executor is required")
	}
	if businessFn == nil {
		return errors.New("business function is required")
	}
	if entry == nil {
		return errors.New("outbox entry is required")
	}
	if err := entry.Validate(); err != nil {
		return err
	}

	return executor.WithTransaction(ctx, func(txCtx context.Context, writer OutboxWriter) error {
		if err := businessFn(txCtx); err != nil {
			return err
		}
		return writer.Insert(txCtx, entry)
	})
}

// OutboxMetrics publishes outbox health and throughput metrics.
type OutboxMetrics struct {
	pendingSizeGauge      prometheus.Gauge
	oldestEventAgeGauge   prometheus.Gauge
	publishedTotalCounter prometheus.Counter
	failedTotalCounter    prometheus.Counter
}

// NewOutboxMetrics registers outbox metrics in the provided registry.
func NewOutboxMetrics(registry *prometheus.Registry, namespace string) (*OutboxMetrics, error) {
	if registry == nil {
		return nil, errors.New("registry is nil")
	}
	if namespace == "" {
		namespace = "nimburion"
	}

	pendingSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "outbox",
		Name:      "pending_size",
		Help:      "Current number of pending outbox entries.",
	})
	oldestAge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "outbox",
		Name:      "oldest_event_age_seconds",
		Help:      "Age in seconds of the oldest pending outbox entry.",
	})
	publishedRate := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "outbox",
		Name:      "published_total",
		Help:      "Total number of outbox entries successfully published.",
	})
	failedRate := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "outbox",
		Name:      "failed_total",
		Help:      "Total number of outbox publish failures.",
	})

	for _, c := range []prometheus.Collector{pendingSize, oldestAge, publishedRate, failedRate} {
		if err := registry.Register(c); err != nil {
			return nil, fmt.Errorf("register outbox metric failed: %w", err)
		}
	}

	return &OutboxMetrics{
		pendingSizeGauge:      pendingSize,
		oldestEventAgeGauge:   oldestAge,
		publishedTotalCounter: publishedRate,
		failedTotalCounter:    failedRate,
	}, nil
}

// Snapshot updates pending-size and oldest-age gauges from store state.
func (m *OutboxMetrics) Snapshot(ctx context.Context, store OutboxStore, now time.Time) {
	if m == nil || store == nil {
		return
	}

	if pending, err := store.PendingCount(ctx, now); err == nil {
		m.pendingSizeGauge.Set(float64(pending))
	}

	if oldestAge, err := store.OldestPendingAgeSeconds(ctx, now); err == nil {
		m.oldestEventAgeGauge.Set(oldestAge)
	}
}

// IncPublished increments successful publishes.
func (m *OutboxMetrics) IncPublished() {
	if m == nil {
		return
	}
	m.publishedTotalCounter.Inc()
}

// IncFailed increments failed publish attempts.
func (m *OutboxMetrics) IncFailed() {
	if m == nil {
		return
	}
	m.failedTotalCounter.Inc()
}

// OutboxPublisherConfig controls outbox polling and retry behavior.
type OutboxPublisherConfig struct {
	PollInterval     time.Duration
	BatchSize        int
	CleanupEvery     time.Duration
	CleanupRetention time.Duration
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
}

// normalize applies default values for zero fields.
func (c *OutboxPublisherConfig) normalize() {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultOutboxPollInterval
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultOutboxBatchSize
	}
	if c.CleanupEvery <= 0 {
		c.CleanupEvery = DefaultOutboxCleanupEvery
	}
	if c.CleanupRetention <= 0 {
		c.CleanupRetention = DefaultOutboxCleanupRetain
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultOutboxInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultOutboxMaxBackoff
	}
}

// OutboxPublisher publishes pending outbox entries to the broker.
type OutboxPublisher struct {
	store    OutboxStore
	producer Producer
	logger   logger.Logger
	metrics  *OutboxMetrics
	config   OutboxPublisherConfig

	mu      sync.Mutex
	running bool
}

// NewOutboxPublisher creates an outbox publisher with normalized config.
func NewOutboxPublisher(store OutboxStore, producer Producer, log logger.Logger, metrics *OutboxMetrics, config OutboxPublisherConfig) (*OutboxPublisher, error) {
	if store == nil {
		return nil, errors.New("outbox store is required")
	}
	if producer == nil {
		return nil, errors.New("outbox producer is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	config.normalize()

	return &OutboxPublisher{
		store:    store,
		producer: producer,
		logger:   log,
		metrics:  metrics,
		config:   config,
	}, nil
}

// Run starts the polling loop and blocks until context cancellation.
func (p *OutboxPublisher) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return errors.New("outbox publisher already running")
	}
	p.running = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	pollTicker := time.NewTicker(p.config.PollInterval)
	defer pollTicker.Stop()

	cleanupTicker := time.NewTicker(p.config.CleanupEvery)
	defer cleanupTicker.Stop()

	// First tick immediately.
	if err := p.tick(ctx, time.Now().UTC()); err != nil {
		p.logger.Error("outbox tick failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-pollTicker.C:
			if err := p.tick(ctx, now.UTC()); err != nil {
				p.logger.Error("outbox tick failed", "error", err)
			}
		case now := <-cleanupTicker.C:
			if err := p.cleanup(ctx, now.UTC()); err != nil {
				p.logger.Error("outbox cleanup failed", "error", err)
			}
		}
	}
}

func (p *OutboxPublisher) tick(ctx context.Context, now time.Time) error {
	entries, err := p.store.FetchPending(ctx, p.config.BatchSize, now)
	if err != nil {
		return fmt.Errorf("fetch pending outbox entries failed: %w", err)
	}

	for _, entry := range entries {
		if err := p.publishEntry(ctx, entry, now); err != nil {
			p.logger.Warn("outbox entry publish failed",
				"entry_id", entry.ID,
				"topic", entry.Topic,
				"retry_count", entry.RetryCount,
				"error", err,
			)
		}
	}

	if p.metrics != nil {
		p.metrics.Snapshot(ctx, p.store, now)
	}

	return nil
}

func (p *OutboxPublisher) publishEntry(ctx context.Context, entry *OutboxEntry, now time.Time) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid outbox entry: %w", err)
	}

	if err := p.producer.Publish(ctx, entry.Topic, entry.Message); err != nil {
		retryCount := entry.RetryCount + 1
		nextAttempt := now.Add(exponentialBackoff(retryCount, p.config.InitialBackoff, p.config.MaxBackoff))
		if markErr := p.store.MarkFailed(ctx, entry.ID, retryCount, nextAttempt, err.Error()); markErr != nil {
			return fmt.Errorf("mark failed error after publish error (%v): %w", err, markErr)
		}
		if p.metrics != nil {
			p.metrics.IncFailed()
		}
		return err
	}

	if err := p.store.MarkPublished(ctx, entry.ID, now); err != nil {
		return fmt.Errorf("mark published failed: %w", err)
	}

	if p.metrics != nil {
		p.metrics.IncPublished()
	}

	return nil
}

func (p *OutboxPublisher) cleanup(ctx context.Context, now time.Time) error {
	before := now.Add(-p.config.CleanupRetention)
	_, err := p.store.CleanupPublishedBefore(ctx, before, p.config.BatchSize)
	if err != nil {
		return fmt.Errorf("cleanup published entries failed: %w", err)
	}
	return nil
}

func exponentialBackoff(attempt int, initial, max time.Duration) time.Duration {
	if attempt <= 0 {
		return initial
	}

	backoff := initial
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= max {
			return max
		}
	}
	if backoff > max {
		return max
	}
	return backoff
}
