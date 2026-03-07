package dedup

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	DefaultRetention = 24 * time.Hour
	DefaultCleanup   = time.Hour
	DefaultBatchSize = 1000
)

type Store interface {
	IsDuplicate(ctx context.Context, scope, key string) (bool, error)
	MarkSeen(ctx context.Context, scope, key string, seenAt time.Time, retainUntil time.Time) error
	CleanupExpiredBefore(ctx context.Context, before time.Time, limit int) (int, error)
}

type Deduplicator struct {
	store Store
}

func New(store Store) (*Deduplicator, error) {
	if store == nil {
		return nil, errors.New("dedup store is required")
	}
	return &Deduplicator{store: store}, nil
}

func (d *Deduplicator) CheckAndMark(ctx context.Context, scope, key string, retention time.Duration) (bool, error) {
	if d == nil || d.store == nil {
		return false, errors.New("deduplicator is not initialized")
	}
	if scope == "" {
		return false, errors.New("scope is required")
	}
	if key == "" {
		return false, errors.New("key is required")
	}
	if retention <= 0 {
		retention = DefaultRetention
	}

	duplicate, err := d.store.IsDuplicate(ctx, scope, key)
	if err != nil {
		return false, fmt.Errorf("check duplicate failed: %w", err)
	}
	if duplicate {
		return true, nil
	}
	now := time.Now().UTC()
	if err := d.store.MarkSeen(ctx, scope, key, now, now.Add(retention)); err != nil {
		return false, fmt.Errorf("mark seen failed: %w", err)
	}
	return false, nil
}

type CleanerConfig struct {
	CleanupEvery time.Duration
	BatchSize    int
}

func (c *CleanerConfig) normalize() {
	if c.CleanupEvery <= 0 {
		c.CleanupEvery = DefaultCleanup
	}
	if c.BatchSize <= 0 {
		c.BatchSize = DefaultBatchSize
	}
}

type Cleaner struct {
	store  Store
	config CleanerConfig
}

func NewCleaner(store Store, config CleanerConfig) (*Cleaner, error) {
	if store == nil {
		return nil, errors.New("dedup store is required")
	}
	config.normalize()
	return &Cleaner{store: store, config: config}, nil
}

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
			if _, err := c.store.CleanupExpiredBefore(ctx, now.UTC(), c.config.BatchSize); err != nil {
				return fmt.Errorf("cleanup expired dedup markers failed: %w", err)
			}
		}
	}
}
