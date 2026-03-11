// Package idempotency provides idempotent execution guards for handlers.
package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// DefaultRetention is the default retention for processed idempotency keys.
	DefaultRetention = 30 * 24 * time.Hour
	// DefaultCleanup is the default cleanup cadence for processed idempotency keys.
	DefaultCleanup = time.Hour
	// DefaultBatchSize is the default cleanup batch size for processed idempotency keys.
	DefaultBatchSize = 1000
)

// CreateTablePostgres is the PostgreSQL schema for idempotency key storage.
const CreateTablePostgres = `
CREATE TABLE IF NOT EXISTS idempotency_keys (
  scope TEXT NOT NULL,
  key TEXT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (scope, key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_processed_at ON idempotency_keys (processed_at);
`

// Store persists processed idempotency keys.
type Store interface {
	IsProcessed(ctx context.Context, scope, key string) (bool, error)
	MarkProcessed(ctx context.Context, scope, key string, processedAt time.Time) error
	CleanupProcessedBefore(ctx context.Context, before time.Time, limit int) (int, error)
}

// AtomicStore provides a safe non-transactional execution path for stores that
// can guarantee duplicate suppression under contention.
type AtomicStore interface {
	ExecuteAtomically(ctx context.Context, scope, key string, handler func(context.Context) error) (bool, error)
}

// ErrAtomicExecutionRequired reports that ExecuteOnce requires a store capable
// of suppressing duplicates atomically under contention.
var ErrAtomicExecutionRequired = errors.New("non-transactional idempotency requires a store with atomic execution support; use ExecuteOnceTransactional or provide an AtomicStore")

// Guard protects handlers from duplicate execution.
type Guard struct {
	store Store
}

// NewGuard constructs a Guard backed by store.
func NewGuard(store Store) (*Guard, error) {
	if store == nil {
		return nil, errors.New("idempotency store is required")
	}
	return &Guard{store: store}, nil
}

// ExecuteOnce runs handler only when scope and key have not been processed before.
func (g *Guard) ExecuteOnce(ctx context.Context, scope, key string, handler func(context.Context) error) (bool, error) {
	if g == nil || g.store == nil {
		return false, errors.New("idempotency guard is not initialized")
	}
	if scope == "" {
		return false, errors.New("scope is required")
	}
	if key == "" {
		return false, errors.New("key is required")
	}
	if handler == nil {
		return false, errors.New("handler is required")
	}
	atomicStore, ok := g.store.(AtomicStore)
	if !ok {
		return false, ErrAtomicExecutionRequired
	}
	return atomicStore.ExecuteAtomically(ctx, scope, key, handler)
}

// TxStore exposes idempotency operations inside a transaction boundary.
type TxStore interface {
	IsProcessed(ctx context.Context, scope, key string) (bool, error)
	MarkProcessed(ctx context.Context, scope, key string, processedAt time.Time) error
}

// TxExecutor executes idempotency operations in a transaction boundary.
type TxExecutor interface {
	WithTransaction(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// ExecuteOnceTransactional runs handler and records scope/key atomically in one transaction.
func ExecuteOnceTransactional(ctx context.Context, executor TxExecutor, scope, key string, handler func(context.Context) error) (bool, error) {
	if executor == nil {
		return false, errors.New("transaction executor is required")
	}
	if scope == "" {
		return false, errors.New("scope is required")
	}
	if key == "" {
		return false, errors.New("key is required")
	}
	if handler == nil {
		return false, errors.New("handler is required")
	}

	executed := false
	err := executor.WithTransaction(ctx, func(txCtx context.Context, txStore TxStore) error {
		processed, err := txStore.IsProcessed(txCtx, scope, key)
		if err != nil {
			return fmt.Errorf("check processed key failed: %w", err)
		}
		if processed {
			executed = false
			return nil
		}
		if err := handler(txCtx); err != nil {
			return err
		}
		if err := txStore.MarkProcessed(txCtx, scope, key, time.Now().UTC()); err != nil {
			return fmt.Errorf("mark processed key failed: %w", err)
		}
		executed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return executed, nil
}

// CleanerConfig configures periodic cleanup of processed idempotency keys.
type CleanerConfig struct {
	CleanupEvery time.Duration
	Retention    time.Duration
	BatchSize    int
}

func (c *CleanerConfig) normalize() {
	if c.CleanupEvery <= 0 {
		c.CleanupEvery = DefaultCleanup
	}
	if c.Retention <= 0 {
		c.Retention = DefaultRetention
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultBatchSize
	}
}

// Cleaner periodically removes old processed idempotency keys from store.
type Cleaner struct {
	store  Store
	config CleanerConfig
}

// NewCleaner constructs a Cleaner backed by store.
func NewCleaner(store Store, config CleanerConfig) (*Cleaner, error) {
	if store == nil {
		return nil, errors.New("idempotency store is required")
	}
	config.normalize()
	return &Cleaner{store: store, config: config}, nil
}

// Run executes cleanup until ctx is done.
func (c *Cleaner) Run(ctx context.Context) error {
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
				return fmt.Errorf("cleanup processed keys failed: %w", err)
			}
		}
	}
}
