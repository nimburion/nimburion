package eventbus

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Idempotency configuration constants
const (
	// DefaultProcessedEventsRetention is the default retention period for processed events
	DefaultProcessedEventsRetention = 30 * 24 * time.Hour
	// DefaultProcessedEventsCleanup is the default cleanup interval
	DefaultProcessedEventsCleanup = time.Hour
	// DefaultProcessedEventsBatchSize is the default batch size for cleanup
	DefaultProcessedEventsBatchSize = 1000
)

// CreateProcessedEventsTablePostgres defines a reference PostgreSQL schema for idempotency tracking.
const CreateProcessedEventsTablePostgres = `
CREATE TABLE IF NOT EXISTS processed_events (
  consumer_name TEXT NOT NULL,
  event_id TEXT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (consumer_name, event_id)
);

CREATE INDEX IF NOT EXISTS idx_processed_events_processed_at ON processed_events (processed_at);
`

// ProcessedEventStore persists event processing markers.
type ProcessedEventStore interface {
	IsProcessed(ctx context.Context, consumerName, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, consumerName, eventID string, processedAt time.Time) error
	CleanupProcessedBefore(ctx context.Context, before time.Time, limit int) (int, error)
}

// IdempotencyGuard ensures handlers process each event only once.
type IdempotencyGuard struct {
	store ProcessedEventStore
}

// NewIdempotencyGuard creates a guard with a backing store.
func NewIdempotencyGuard(store ProcessedEventStore) (*IdempotencyGuard, error) {
	if store == nil {
		return nil, errors.New("processed event store is required")
	}
	return &IdempotencyGuard{store: store}, nil
}

// ProcessOnce executes handler only if the event was not previously processed.
// It returns true when handler executed, false when event was a duplicate.
func (g *IdempotencyGuard) ProcessOnce(
	ctx context.Context,
	consumerName, eventID string,
	handler func(ctx context.Context) error,
) (bool, error) {
	if g == nil || g.store == nil {
		return false, errors.New("idempotency guard is not initialized")
	}
	if consumerName == "" {
		return false, errors.New("consumer name is required")
	}
	if eventID == "" {
		return false, errors.New("event id is required")
	}
	if handler == nil {
		return false, errors.New("handler is required")
	}

	processed, err := g.store.IsProcessed(ctx, consumerName, eventID)
	if err != nil {
		return false, fmt.Errorf("check processed event failed: %w", err)
	}
	if processed {
		return false, nil
	}

	if err := handler(ctx); err != nil {
		return false, err
	}

	if err := g.store.MarkProcessed(ctx, consumerName, eventID, time.Now().UTC()); err != nil {
		return false, fmt.Errorf("mark processed event failed: %w", err)
	}

	return true, nil
}

// ProcessedEventTxStore stores idempotency markers inside a transaction.
type ProcessedEventTxStore interface {
	IsProcessed(ctx context.Context, consumerName, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, consumerName, eventID string, processedAt time.Time) error
}

// ProcessedEventTxExecutor executes a callback in a transaction boundary.
type ProcessedEventTxExecutor interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context, txStore ProcessedEventTxStore) error) error
}

// ProcessOnceTransactional executes business logic and event-marking atomically in one transaction.
// Returns true if handler ran, false if event was already processed.
func ProcessOnceTransactional(
	ctx context.Context,
	executor ProcessedEventTxExecutor,
	consumerName, eventID string,
	handler func(ctx context.Context) error,
) (bool, error) {
	if executor == nil {
		return false, errors.New("transaction executor is required")
	}
	if consumerName == "" {
		return false, errors.New("consumer name is required")
	}
	if eventID == "" {
		return false, errors.New("event id is required")
	}
	if handler == nil {
		return false, errors.New("handler is required")
	}

	executed := false
	err := executor.WithTransaction(ctx, func(txCtx context.Context, txStore ProcessedEventTxStore) error {
		processed, err := txStore.IsProcessed(txCtx, consumerName, eventID)
		if err != nil {
			return fmt.Errorf("check processed event failed: %w", err)
		}
		if processed {
			executed = false
			return nil
		}

		if err := handler(txCtx); err != nil {
			return err
		}

		if err := txStore.MarkProcessed(txCtx, consumerName, eventID, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark processed event failed: %w", err)
		}

		executed = true
		return nil
	})
	if err != nil {
		return false, err
	}

	return executed, nil
}

// ProcessedEventsCleanerConfig configures periodic cleanup of old markers.
type ProcessedEventsCleanerConfig struct {
	CleanupEvery time.Duration
	Retention    time.Duration
	BatchSize    int
}

func (c *ProcessedEventsCleanerConfig) normalize() {
	if c.CleanupEvery <= 0 {
		c.CleanupEvery = DefaultProcessedEventsCleanup
	}
	if c.Retention <= 0 {
		c.Retention = DefaultProcessedEventsRetention
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultProcessedEventsBatchSize
	}
}

// ProcessedEventsCleaner periodically deletes stale processed-event markers.
type ProcessedEventsCleaner struct {
	store  ProcessedEventStore
	config ProcessedEventsCleanerConfig
}

// NewProcessedEventsCleaner creates a cleanup service for idempotency markers.
func NewProcessedEventsCleaner(store ProcessedEventStore, config ProcessedEventsCleanerConfig) (*ProcessedEventsCleaner, error) {
	if store == nil {
		return nil, errors.New("processed event store is required")
	}
	config.normalize()
	return &ProcessedEventsCleaner{store: store, config: config}, nil
}

// Run starts a periodic cleanup loop until context cancellation.
func (c *ProcessedEventsCleaner) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}

	ticker := time.NewTicker(c.config.CleanupEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-ticker.C:
			before := now.UTC().Add(-c.config.Retention)
			if _, err := c.store.CleanupProcessedBefore(ctx, before, c.config.BatchSize); err != nil {
				return fmt.Errorf("cleanup processed events failed: %w", err)
			}
		}
	}
}
