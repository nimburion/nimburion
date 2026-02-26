package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

type scriptedLockProvider struct {
	mu       sync.Mutex
	outcomes []bool
	index    int
	acquires int
	releases int
}

func newScriptedLockProvider(outcomes []bool) *scriptedLockProvider {
	copied := make([]bool, len(outcomes))
	copy(copied, outcomes)
	return &scriptedLockProvider{outcomes: copied}
}

func (p *scriptedLockProvider) Acquire(_ context.Context, key string, ttl time.Duration) (*LockLease, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.acquires++

	outcome := false
	if p.index < len(p.outcomes) {
		outcome = p.outcomes[p.index]
	}
	p.index++
	if !outcome {
		return nil, false, nil
	}

	return &LockLease{
		Key:      key,
		Token:    fmt.Sprintf("lease-%d", p.acquires),
		ExpireAt: time.Now().UTC().Add(ttl),
	}, true, nil
}

func (p *scriptedLockProvider) Renew(context.Context, *LockLease, time.Duration) error { return nil }

func (p *scriptedLockProvider) Release(context.Context, *LockLease) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releases++
	return nil
}

func (p *scriptedLockProvider) HealthCheck(context.Context) error { return nil }
func (p *scriptedLockProvider) Close() error                      { return nil }

func (p *scriptedLockProvider) stats() (acquires int, releases int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.acquires, p.releases
}

func TestRuntime_Property_DispatchCountMatchesAcquiredLocks(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 40
	properties := gopter.NewProperties(parameters)

	properties.Property("dispatches and releases happen only when lock is acquired", prop.ForAll(
		func(outcomes []bool) bool {
			lockProvider := newScriptedLockProvider(outcomes)
			jobRuntime := &fakeJobsRuntime{}

			runtime, err := NewRuntime(jobRuntime, lockProvider, &schedulerTestLogger{}, Config{})
			if err != nil {
				return false
			}

			task := Task{
				Name:     "billing-close-day",
				Schedule: "@every 1h",
				Queue:    "billing",
				JobName:  "billing.close_day",
				Payload:  []byte(`{"source":"scheduler"}`),
				LockTTL:  150 * time.Millisecond,
			}

			for idx := range outcomes {
				runAt := time.Unix(int64(idx+1), 0).UTC()
				if err := runtime.dispatchTask(context.Background(), task, runAt); err != nil {
					return false
				}
			}

			expectedDispatches := 0
			for _, acquired := range outcomes {
				if acquired {
					expectedDispatches++
				}
			}

			acquires, releases := lockProvider.stats()
			return acquires == len(outcomes) &&
				releases == expectedDispatches &&
				jobRuntime.count() == expectedDispatches
		},
		gen.SliceOf(gen.Bool()),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
