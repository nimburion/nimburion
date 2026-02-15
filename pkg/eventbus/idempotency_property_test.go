package eventbus

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_IdempotentEventProcessing validates Property 4: Idempotent Event Processing.
//
// For any event processed multiple times, the result is identical to processing it once.
//
// **Validates: Requirements 18.1**
func TestProperty_IdempotentEventProcessing(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("repeated event IDs are executed at most once", prop.ForAll(
		func(eventIDs []string) bool {
			if len(eventIDs) == 0 {
				return true
			}

			store := newMemoryProcessedStore()
			guard, err := NewIdempotencyGuard(store)
			if err != nil {
				t.Logf("guard init failed: %v", err)
				return false
			}

			effects := map[string]int{}
			for _, eventID := range eventIDs {
				if eventID == "" {
					continue
				}

				_, err := guard.ProcessOnce(context.Background(), "consumer-a", eventID, func(context.Context) error {
					effects[eventID]++
					return nil
				})
				if err != nil {
					t.Logf("process failed: %v", err)
					return false
				}
			}

			// For each unique non-empty event ID, side effects must have executed exactly once.
			unique := map[string]struct{}{}
			for _, eventID := range eventIDs {
				if eventID != "" {
					unique[eventID] = struct{}{}
				}
			}

			for eventID := range unique {
				if effects[eventID] != 1 {
					t.Logf("event %s executed %d times", eventID, effects[eventID])
					return false
				}
			}

			return true
		},
		gen.SliceOf(gen.OneConstOf("evt_a", "evt_b", "evt_c", "evt_d", "", "evt_a")),
	))

	properties.TestingRun(t)
}

func TestProcessedEventsCleaner_RunStopsOnContextCancel(t *testing.T) {
	store := newMemoryProcessedStore()
	store.MarkProcessed(context.Background(), "c", "e", time.Now().UTC().Add(-31*24*time.Hour))

	cleaner, err := NewProcessedEventsCleaner(store, ProcessedEventsCleanerConfig{
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

	processed, err := store.IsProcessed(context.Background(), "c", "e")
	if err != nil {
		t.Fatalf("is processed failed: %v", err)
	}
	if processed {
		t.Fatalf("expected old marker to be cleaned")
	}
}
