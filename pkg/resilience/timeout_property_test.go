package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Validates: Requirements 32.1-32.6**
// Property 30: Timeout Enforcement
// For any operation with a configured timeout (HTTP request, database query, cache operation,
// message broker operation), the operation should be cancelled and return a timeout error when
// the timeout duration is exceeded.
func TestProperty_TimeoutEnforcement(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for timeout duration (10-200ms)
	genTimeout := gen.IntRange(10, 200).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	// Generator for operation duration (5-300ms)
	genOperationDuration := gen.IntRange(5, 300).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	properties.Property("operations exceeding timeout return ErrTimeout", prop.ForAll(
		func(timeout time.Duration, operationDuration time.Duration) bool {
			ctx := context.Background()

			fn := func(ctx context.Context) error {
				// Simulate operation with the given duration
				select {
				case <-time.After(operationDuration):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			err := WithTimeout(ctx, timeout, fn)

			// Add tolerance for timing precision (5ms buffer)
			tolerance := 5 * time.Millisecond

			// If operation duration significantly exceeds timeout, should get ErrTimeout
			if operationDuration > timeout+tolerance {
				if err != ErrTimeout {
					t.Logf("Expected ErrTimeout when operation (%v) exceeds timeout (%v), got %v",
						operationDuration, timeout, err)
					return false
				}
			} else if operationDuration < timeout-tolerance {
				// If operation completes well within timeout, should succeed
				if err != nil {
					t.Logf("Expected no error when operation (%v) completes within timeout (%v), got %v",
						operationDuration, timeout, err)
					return false
				}
			}
			// For operations within tolerance range, accept either outcome

			return true
		},
		genTimeout,
		genOperationDuration,
	))

	properties.Property("operations completing within timeout succeed", prop.ForAll(
		func(timeout time.Duration) bool {
			ctx := context.Background()

			// Operation that completes quickly (within 5ms)
			fn := func(ctx context.Context) error {
				time.Sleep(5 * time.Millisecond)
				return nil
			}

			err := WithTimeout(ctx, timeout, fn)

			if err != nil {
				t.Logf("Expected no error for fast operation with timeout %v, got %v", timeout, err)
				return false
			}

			return true
		},
		genTimeout,
	))

	properties.Property("timeout respects function errors", prop.ForAll(
		func(timeout time.Duration) bool {
			ctx := context.Background()

			expectedErr := errors.New("function error")
			fn := func(ctx context.Context) error {
				// Return error quickly
				return expectedErr
			}

			err := WithTimeout(ctx, timeout, fn)

			if err != expectedErr {
				t.Logf("Expected function error to be returned, got %v", err)
				return false
			}

			return true
		},
		genTimeout,
	))

	properties.Property("timeout context is passed to function", prop.ForAll(
		func(timeout time.Duration) bool {
			ctx := context.Background()

			contextHadDeadline := false
			fn := func(ctx context.Context) error {
				_, ok := ctx.Deadline()
				contextHadDeadline = ok
				return nil
			}

			err := WithTimeout(ctx, timeout, fn)

			if err != nil {
				t.Logf("Expected no error, got %v", err)
				return false
			}

			if !contextHadDeadline {
				t.Log("Expected context to have deadline")
				return false
			}

			return true
		},
		genTimeout,
	))

	properties.Property("zero or negative timeout times out immediately", prop.ForAll(
		func(timeoutMs int) bool {
			// Generate zero or negative timeout
			if timeoutMs > 0 {
				return true // Skip positive timeouts
			}

			timeout := time.Duration(timeoutMs) * time.Millisecond
			ctx := context.Background()

			fn := func(ctx context.Context) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			}

			err := WithTimeout(ctx, timeout, fn)

			if err != ErrTimeout {
				t.Logf("Expected ErrTimeout for timeout %v, got %v", timeout, err)
				return false
			}

			return true
		},
		gen.IntRange(-100, 0),
	))

	properties.TestingRun(t)
}

// Property test: Verify TimeoutFunc wrapper works correctly
func TestProperty_TimeoutFuncWrapper(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	genTimeout := gen.IntRange(10, 200).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	genOperationDuration := gen.IntRange(5, 300).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	properties.Property("TimeoutFunc enforces configured timeout", prop.ForAll(
		func(timeout time.Duration, operationDuration time.Duration) bool {
			tf := NewTimeoutFunc(timeout)
			ctx := context.Background()

			fn := func(ctx context.Context) error {
				select {
				case <-time.After(operationDuration):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			err := tf.Execute(ctx, fn)

			// Add tolerance for timing precision (5ms buffer)
			tolerance := 5 * time.Millisecond

			if operationDuration > timeout+tolerance {
				if err != ErrTimeout {
					t.Logf("Expected ErrTimeout, got %v", err)
					return false
				}
			} else if operationDuration < timeout-tolerance {
				if err != nil {
					t.Logf("Expected no error, got %v", err)
					return false
				}
			}
			// For operations within tolerance range, accept either outcome

			return true
		},
		genTimeout,
		genOperationDuration,
	))

	properties.Property("TimeoutFunc custom timeout overrides default", prop.ForAll(
		func(defaultTimeout time.Duration, customTimeout time.Duration, operationDuration time.Duration) bool {
			tf := NewTimeoutFunc(defaultTimeout)
			ctx := context.Background()

			fn := func(ctx context.Context) error {
				select {
				case <-time.After(operationDuration):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			err := tf.ExecuteWithCustomTimeout(ctx, customTimeout, fn)

			// Add tolerance for timing precision (5ms buffer)
			tolerance := 5 * time.Millisecond

			// Should use custom timeout, not default
			if operationDuration > customTimeout+tolerance {
				if err != ErrTimeout {
					t.Logf("Expected ErrTimeout with custom timeout %v, got %v", customTimeout, err)
					return false
				}
			} else if operationDuration < customTimeout-tolerance {
				if err != nil {
					t.Logf("Expected no error with custom timeout %v, got %v", customTimeout, err)
					return false
				}
			}
			// For operations within tolerance range, accept either outcome

			return true
		},
		genTimeout,
		genTimeout,
		genOperationDuration,
	))

	properties.TestingRun(t)
}

// Property test: Verify timeout behavior with context cancellation
func TestProperty_TimeoutWithContextCancellation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	genTimeout := gen.IntRange(50, 200).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	genCancelDelay := gen.IntRange(5, 100).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	properties.Property("parent context cancellation propagates to function", prop.ForAll(
		func(timeout time.Duration, cancelDelay time.Duration) bool {
			ctx, cancel := context.WithCancel(context.Background())

			// Cancel context after delay
			go func() {
				time.Sleep(cancelDelay)
				cancel()
			}()

			fn := func(ctx context.Context) error {
				// Wait for context cancellation
				<-ctx.Done()
				return ctx.Err()
			}

			err := WithTimeout(ctx, timeout, fn)

			// Should get either timeout or cancellation error
			if err == nil {
				t.Log("Expected error from cancelled context or timeout")
				return false
			}

			// Error should be either ErrTimeout or context.Canceled
			if err != ErrTimeout && !errors.Is(err, context.Canceled) {
				t.Logf("Expected ErrTimeout or context.Canceled, got %v", err)
				return false
			}

			return true
		},
		genTimeout,
		genCancelDelay,
	))

	properties.TestingRun(t)
}

// Property test: Verify concurrent timeout operations
func TestProperty_ConcurrentTimeoutOperations(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Fewer iterations due to concurrency
	properties := gopter.NewProperties(parameters)

	genTimeout := gen.IntRange(50, 150).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	genGoroutines := gen.IntRange(2, 10)

	properties.Property("concurrent timeout operations work correctly", prop.ForAll(
		func(timeout time.Duration, numGoroutines int) bool {
			tf := NewTimeoutFunc(timeout)
			ctx := context.Background()

			type result struct {
				id  int
				err error
			}
			results := make(chan result, numGoroutines)

			// Launch concurrent operations
			for i := 0; i < numGoroutines; i++ {
				go func(id int) {
					// Alternate between fast and slow operations
					var operationDuration time.Duration
					if id%2 == 0 {
						operationDuration = timeout / 3 // Fast - well within timeout
					} else {
						operationDuration = timeout * 3 // Slow - well beyond timeout
					}

					fn := func(ctx context.Context) error {
						select {
						case <-time.After(operationDuration):
							return nil
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					err := tf.Execute(ctx, fn)
					results <- result{id: id, err: err}
				}(i)
			}

			// Collect results
			for i := 0; i < numGoroutines; i++ {
				res := <-results

				// Fast operations (even IDs) should succeed, slow ones (odd IDs) should timeout
				if res.id%2 == 0 {
					if res.err != nil {
						t.Logf("Expected fast operation %d to succeed, got error: %v", res.id, res.err)
						return false
					}
				} else {
					if res.err != ErrTimeout {
						t.Logf("Expected slow operation %d to timeout, got: %v", res.id, res.err)
						return false
					}
				}
			}

			return true
		},
		genTimeout,
		genGoroutines,
	))

	properties.TestingRun(t)
}
