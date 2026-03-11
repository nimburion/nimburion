// Package outbox provides outbox primitives for reliable event publication.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	// DefaultPollInterval is the default relay polling cadence.
	DefaultPollInterval = time.Second
	// DefaultBatchSize is the default number of outbox entries processed per batch.
	DefaultBatchSize = 100
	// DefaultCleanupEvery is the default cleanup cadence for published entries.
	DefaultCleanupEvery = time.Minute
	// DefaultCleanupRetain is the default retention window for published entries.
	DefaultCleanupRetain = 7 * 24 * time.Hour
	// DefaultInitialBackoff is the default initial relay retry backoff.
	DefaultInitialBackoff = time.Second
	// DefaultMaxBackoff is the default maximum relay retry backoff.
	DefaultMaxBackoff = time.Minute
)

// CreateTablePostgres is the PostgreSQL schema for outbox storage.
const CreateTablePostgres = `
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

// Record is the publishable payload stored in the outbox.
type Record struct {
	ID          string
	Key         string
	Payload     []byte
	Headers     map[string]string
	ContentType string
}

// Entry represents one outbox row.
type Entry struct {
	ID          string
	Topic       string
	Record      *Record
	CreatedAt   time.Time
	AvailableAt time.Time
	Published   bool
	PublishedAt *time.Time
	RetryCount  int
	LastError   string
}

// Validate checks that the outbox entry contains the required fields.
func (e *Entry) Validate() error {
	if e == nil {
		return errors.New("outbox entry is nil")
	}
	if e.ID == "" {
		return errors.New("outbox entry id is required")
	}
	if e.Topic == "" {
		return errors.New("outbox entry topic is required")
	}
	if e.Record == nil {
		return errors.New("outbox entry record is required")
	}
	if len(e.Record.Payload) == 0 {
		return errors.New("outbox entry record payload is required")
	}
	return nil
}

// Store persists outbox entries and relay state.
type Store interface {
	Insert(ctx context.Context, entry *Entry) error
	FetchPending(ctx context.Context, limit int, now time.Time) ([]*Entry, error)
	MarkPublished(ctx context.Context, id string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id string, retryCount int, nextAttemptAt time.Time, reason string) error
	CleanupPublishedBefore(ctx context.Context, before time.Time, limit int) (int, error)
	PendingCount(ctx context.Context, now time.Time) (int, error)
	OldestPendingAgeSeconds(ctx context.Context, now time.Time) (float64, error)
}

// Writer exposes outbox writes inside a transaction boundary.
type Writer interface {
	Insert(ctx context.Context, entry *Entry) error
}

// TxExecutor executes outbox operations in a transaction boundary.
type TxExecutor interface {
	WithTransaction(ctx context.Context, fn func(context.Context, Writer) error) error
}

// ExecuteTransactional runs businessFn and inserts entry atomically.
func ExecuteTransactional(ctx context.Context, executor TxExecutor, entry *Entry, businessFn func(context.Context) error) error {
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
	return executor.WithTransaction(ctx, func(txCtx context.Context, writer Writer) error {
		if err := businessFn(txCtx); err != nil {
			return err
		}
		return writer.Insert(txCtx, entry)
	})
}

// Metrics tracks relay queue depth and publish outcomes.
type Metrics struct {
	pendingSizeGauge      prometheus.Gauge
	oldestEventAgeGauge   prometheus.Gauge
	publishedTotalCounter prometheus.Counter
	failedTotalCounter    prometheus.Counter
}

// NewMetrics registers outbox metrics in registry.
func NewMetrics(registry *prometheus.Registry, namespace string) (*Metrics, error) {
	if registry == nil {
		return nil, errors.New("registry is nil")
	}
	if namespace == "" {
		namespace = "nimburion"
	}

	pendingSize := prometheus.NewGauge(prometheus.GaugeOpts{Namespace: namespace, Subsystem: "outbox", Name: "pending_size", Help: "Current number of pending outbox entries."})
	oldestAge := prometheus.NewGauge(prometheus.GaugeOpts{Namespace: namespace, Subsystem: "outbox", Name: "oldest_event_age_seconds", Help: "Age in seconds of the oldest pending outbox entry."})
	publishedRate := prometheus.NewCounter(prometheus.CounterOpts{Namespace: namespace, Subsystem: "outbox", Name: "published_total", Help: "Total number of outbox entries successfully published."})
	failedRate := prometheus.NewCounter(prometheus.CounterOpts{Namespace: namespace, Subsystem: "outbox", Name: "failed_total", Help: "Total number of outbox publish failures."})

	for _, c := range []prometheus.Collector{pendingSize, oldestAge, publishedRate, failedRate} {
		if err := registry.Register(c); err != nil {
			return nil, fmt.Errorf("register outbox metric failed: %w", err)
		}
	}

	return &Metrics{
		pendingSizeGauge:      pendingSize,
		oldestEventAgeGauge:   oldestAge,
		publishedTotalCounter: publishedRate,
		failedTotalCounter:    failedRate,
	}, nil
}

// Snapshot refreshes gauges from the current store state.
func (m *Metrics) Snapshot(ctx context.Context, store Store, now time.Time) {
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

// IncPublished increments the published counter.
func (m *Metrics) IncPublished() {
	if m != nil {
		m.publishedTotalCounter.Inc()
	}
}

// IncFailed increments the failed counter.
func (m *Metrics) IncFailed() {
	if m != nil {
		m.failedTotalCounter.Inc()
	}
}

// Publisher publishes one outbox record to a topic.
type Publisher interface {
	Publish(ctx context.Context, topic string, record *Record) error
}

// PublisherConfig configures relay polling, cleanup, and backoff behavior.
type PublisherConfig struct {
	PollInterval     time.Duration
	BatchSize        int
	CleanupEvery     time.Duration
	CleanupRetention time.Duration
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
}

func (c *PublisherConfig) normalize() {
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultPollInterval
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultBatchSize
	}
	if c.CleanupEvery <= 0 {
		c.CleanupEvery = DefaultCleanupEvery
	}
	if c.CleanupRetention <= 0 {
		c.CleanupRetention = DefaultCleanupRetain
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultMaxBackoff
	}
}

// Relay polls the outbox and publishes pending entries.
type Relay struct {
	store     Store
	publisher Publisher
	logger    logger.Logger
	metrics   *Metrics
	config    PublisherConfig

	mu      sync.Mutex
	running bool
}

// NewRelay constructs a Relay backed by store and publisher.
func NewRelay(store Store, publisher Publisher, log logger.Logger, metrics *Metrics, config PublisherConfig) (*Relay, error) {
	if store == nil {
		return nil, errors.New("outbox store is required")
	}
	if publisher == nil {
		return nil, errors.New("outbox publisher is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	config.normalize()
	return &Relay{store: store, publisher: publisher, logger: log, metrics: metrics, config: config}, nil
}

// Run starts the relay loop until ctx is done.
func (r *Relay) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return errors.New("outbox relay already running")
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	pollTicker := time.NewTicker(r.config.PollInterval)
	defer pollTicker.Stop()
	cleanupTicker := time.NewTicker(r.config.CleanupEvery)
	defer cleanupTicker.Stop()

	if err := r.tick(ctx, time.Now().UTC()); err != nil {
		r.logger.Error("outbox tick failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-pollTicker.C:
			if err := r.tick(ctx, now.UTC()); err != nil {
				r.logger.Error("outbox tick failed", "error", err)
			}
		case now := <-cleanupTicker.C:
			if err := r.cleanup(ctx, now.UTC()); err != nil {
				r.logger.Error("outbox cleanup failed", "error", err)
			}
		}
	}
}

func (r *Relay) tick(ctx context.Context, now time.Time) error {
	entries, err := r.store.FetchPending(ctx, r.config.BatchSize, now)
	if err != nil {
		return fmt.Errorf("fetch pending outbox entries failed: %w", err)
	}
	for _, entry := range entries {
		if err := r.publishEntry(ctx, entry, now); err != nil {
			r.logger.Warn("outbox entry publish failed", "entry_id", entry.ID, "topic", entry.Topic, "retry_count", entry.RetryCount, "error", err)
		}
	}
	if r.metrics != nil {
		r.metrics.Snapshot(ctx, r.store, now)
	}
	return nil
}

func (r *Relay) publishEntry(ctx context.Context, entry *Entry, now time.Time) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid outbox entry: %w", err)
	}
	if err := r.publisher.Publish(ctx, entry.Topic, entry.Record); err != nil {
		retryCount := entry.RetryCount + 1
		nextAttempt := now.Add(exponentialBackoff(retryCount, r.config.InitialBackoff, r.config.MaxBackoff))
		if markErr := r.store.MarkFailed(ctx, entry.ID, retryCount, nextAttempt, err.Error()); markErr != nil {
			return fmt.Errorf("mark failed error after publish error (%w): %w", err, markErr)
		}
		if r.metrics != nil {
			r.metrics.IncFailed()
		}
		return err
	}
	if err := r.store.MarkPublished(ctx, entry.ID, now); err != nil {
		return fmt.Errorf("mark published failed: %w", err)
	}
	if r.metrics != nil {
		r.metrics.IncPublished()
	}
	return nil
}

func (r *Relay) cleanup(ctx context.Context, now time.Time) error {
	before := now.Add(-r.config.CleanupRetention)
	if _, err := r.store.CleanupPublishedBefore(ctx, before, r.config.BatchSize); err != nil {
		return fmt.Errorf("cleanup published entries failed: %w", err)
	}
	return nil
}

func exponentialBackoff(attempt int, initial, maxBackoff time.Duration) time.Duration {
	if attempt <= 0 {
		return initial
	}
	backoff := initial
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= maxBackoff {
			return maxBackoff
		}
	}
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}
