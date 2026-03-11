package idempotency

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_IdempotentProcessing(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("repeated keys execute at most once", prop.ForAll(
		func(keys []string) bool {
			if len(keys) == 0 {
				return true
			}

			store := newMemoryStore()
			guard, err := NewGuard(store)
			if err != nil {
				return false
			}

			effects := map[string]int{}
			for _, key := range keys {
				if key == "" {
					continue
				}
				if _, err := guard.ExecuteOnce(context.Background(), "consumer-a", key, func(context.Context) error {
					effects[key]++
					return nil
				}); err != nil {
					return false
				}
			}

			unique := map[string]struct{}{}
			for _, key := range keys {
				if key != "" {
					unique[key] = struct{}{}
				}
			}
			for key := range unique {
				if effects[key] != 1 {
					return false
				}
			}
			return true
		},
		gen.SliceOf(gen.OneConstOf("evt_a", "evt_b", "evt_c", "evt_d", "", "evt_a")),
	))

	properties.TestingRun(t)
}

func TestCleaner_RunStopsOnContextCancel(t *testing.T) {
	store := newMemoryStore()
	_ = store.MarkProcessed(context.Background(), "consumer-a", "evt-1", time.Now().UTC().Add(-31*24*time.Hour))

	cleaner, err := NewCleaner(store, CleanerConfig{
		CleanupEvery: 5 * time.Millisecond,
		Retention:    30 * 24 * time.Hour,
		BatchSize:    100,
	})
	if err != nil {
		t.Fatalf("new cleaner failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_ = cleaner.Run(ctx)

	processed, err := store.IsProcessed(context.Background(), "consumer-a", "evt-1")
	if err != nil {
		t.Fatalf("is processed failed: %v", err)
	}
	if processed {
		t.Fatal("expected old marker to be cleaned")
	}
}
