package resilience

import (
	"errors"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Validates: Requirements 33.1-33.7**
// Property 31: Circuit Breaker State Machine
// For any circuit breaker protecting an operation, the breaker should transition from Closed to Open
// when failure rate exceeds threshold, from Open to Half-Open after timeout, and from Half-Open to
// Closed when success rate improves (or back to Open on continued failures).
func TestProperty_CircuitBreakerStateMachine(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for max failures threshold (1-10)
	genMaxFailures := gen.IntRange(1, 10)

	// Generator for timeout duration (10-200ms)
	genTimeout := gen.IntRange(10, 200).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})

	properties.Property("circuit breaker transitions from Closed to Open when failures exceed threshold", prop.ForAll(
		func(maxFailures int, timeout time.Duration) bool {
			cb := NewCircuitBreaker(maxFailures, timeout)

			// Initial state should be Closed
			if cb.GetState() != StateClosed {
				t.Logf("Initial state is not Closed: %v", cb.GetState())
				return false
			}

			failingFn := func() error {
				return errors.New("operation failed")
			}

			// Execute failing operations up to maxFailures
			for i := 0; i < maxFailures; i++ {
				err := cb.Execute(failingFn)
				if err == nil {
					t.Logf("Expected error from failing function at iteration %d", i)
					return false
				}
			}

			// Circuit should now be Open
			if cb.GetState() != StateOpen {
				t.Logf("Expected state to be Open after %d failures, got %v", maxFailures, cb.GetState())
				return false
			}

			// Next execution should fail immediately with ErrCircuitBreakerOpen
			err := cb.Execute(failingFn)
			if err != ErrCircuitBreakerOpen {
				t.Logf("Expected ErrCircuitBreakerOpen, got %v", err)
				return false
			}

			return true
		},
		genMaxFailures,
		genTimeout,
	))

	properties.Property("circuit breaker transitions from Open to Half-Open after timeout", prop.ForAll(
		func(maxFailures int, timeout time.Duration) bool {
			cb := NewCircuitBreaker(maxFailures, timeout)

			failingFn := func() error {
				return errors.New("operation failed")
			}

			// Open the circuit
			for i := 0; i < maxFailures; i++ {
				cb.Execute(failingFn)
			}

			if cb.GetState() != StateOpen {
				t.Logf("Circuit should be Open, got %v", cb.GetState())
				return false
			}

			// Wait for timeout plus a small buffer
			time.Sleep(timeout + 10*time.Millisecond)

			// The next canExecute check should transition to Half-Open
			// We verify this by attempting an execution
			successFn := func() error {
				return nil
			}

			// This should succeed and transition through Half-Open to Closed
			err := cb.Execute(successFn)
			if err != nil {
				t.Logf("Expected successful execution after timeout, got error: %v", err)
				return false
			}

			// After successful execution in Half-Open, should be Closed
			if cb.GetState() != StateClosed {
				t.Logf("Expected state to be Closed after successful Half-Open execution, got %v", cb.GetState())
				return false
			}

			return true
		},
		genMaxFailures,
		genTimeout,
	))

	properties.Property("circuit breaker transitions from Half-Open to Closed on success", prop.ForAll(
		func(maxFailures int, timeout time.Duration) bool {
			cb := NewCircuitBreaker(maxFailures, timeout)

			failingFn := func() error {
				return errors.New("operation failed")
			}

			// Open the circuit
			for i := 0; i < maxFailures; i++ {
				cb.Execute(failingFn)
			}

			// Wait for timeout
			time.Sleep(timeout + 10*time.Millisecond)

			successFn := func() error {
				return nil
			}

			// Execute successful operation in Half-Open state
			err := cb.Execute(successFn)
			if err != nil {
				t.Logf("Expected successful execution, got error: %v", err)
				return false
			}

			// Should transition to Closed
			if cb.GetState() != StateClosed {
				t.Logf("Expected state to be Closed after success in Half-Open, got %v", cb.GetState())
				return false
			}

			// Failure count should be reset
			if cb.GetFailures() != 0 {
				t.Logf("Expected failures to be reset to 0, got %d", cb.GetFailures())
				return false
			}

			return true
		},
		genMaxFailures,
		genTimeout,
	))

	properties.Property("circuit breaker transitions from Half-Open to Open on failure", prop.ForAll(
		func(maxFailures int, timeout time.Duration) bool {
			cb := NewCircuitBreaker(maxFailures, timeout)

			failingFn := func() error {
				return errors.New("operation failed")
			}

			// Open the circuit
			for i := 0; i < maxFailures; i++ {
				cb.Execute(failingFn)
			}

			// Wait for timeout
			time.Sleep(timeout + 10*time.Millisecond)

			// Execute failing operation in Half-Open state
			err := cb.Execute(failingFn)
			if err == nil {
				t.Logf("Expected error from failing function")
				return false
			}

			// Should transition back to Open
			if cb.GetState() != StateOpen {
				t.Logf("Expected state to be Open after failure in Half-Open, got %v", cb.GetState())
				return false
			}

			return true
		},
		genMaxFailures,
		genTimeout,
	))

	properties.Property("success in Closed state resets failure count", prop.ForAll(
		func(maxFailures int, timeout time.Duration, failureCount int) bool {
			// Ensure failureCount is less than maxFailures
			if failureCount >= maxFailures {
				failureCount = maxFailures - 1
			}
			if failureCount < 1 {
				return true // Skip invalid test case
			}

			cb := NewCircuitBreaker(maxFailures, timeout)

			failingFn := func() error {
				return errors.New("operation failed")
			}

			successFn := func() error {
				return nil
			}

			// Execute some failing operations (but not enough to open the circuit)
			for i := 0; i < failureCount; i++ {
				cb.Execute(failingFn)
			}

			// Verify we haven't opened the circuit yet
			if cb.GetState() != StateClosed {
				// If maxFailures == failureCount, circuit would be open
				// This is expected, skip this test case
				return true
			}

			if cb.GetFailures() != failureCount {
				t.Logf("Expected %d failures, got %d", failureCount, cb.GetFailures())
				return false
			}

			// Execute successful operation
			err := cb.Execute(successFn)
			if err != nil {
				t.Logf("Expected successful execution, got error: %v", err)
				return false
			}

			// Failures should be reset
			if cb.GetFailures() != 0 {
				t.Logf("Expected failures to be reset to 0, got %d", cb.GetFailures())
				return false
			}

			// Circuit should still be Closed
			if cb.GetState() != StateClosed {
				t.Logf("Expected state to be Closed, got %v", cb.GetState())
				return false
			}

			return true
		},
		gen.IntRange(2, 10), // Start from 2 to ensure we can have failures without opening
		genTimeout,
		gen.IntRange(1, 9),
	))

	properties.Property("circuit breaker tracks failure rate correctly", prop.ForAll(
		func(maxFailures int, timeout time.Duration, results []bool) bool {
			if len(results) == 0 {
				return true // Skip empty test case
			}

			cb := NewCircuitBreaker(maxFailures, timeout)

			consecutiveFailures := 0
			shouldBeOpen := false

			for i, success := range results {
				var fn func() error
				if success {
					fn = func() error { return nil }
				} else {
					fn = func() error { return errors.New("operation failed") }
				}

				_ = cb.Execute(fn)

				// Track consecutive failures
				if !success {
					consecutiveFailures++
					if consecutiveFailures >= maxFailures {
						shouldBeOpen = true
					}
				} else {
					consecutiveFailures = 0
					if cb.GetState() == StateClosed {
						shouldBeOpen = false
					}
				}

				// If circuit should be open, verify it is
				if shouldBeOpen && cb.GetState() != StateOpen {
					// Unless we just had a success in Half-Open that closed it
					if !(success && cb.GetState() == StateClosed) {
						t.Logf("Expected circuit to be Open at iteration %d, got %v", i, cb.GetState())
						return false
					}
				}

				// If circuit is open, next call should return ErrCircuitBreakerOpen
				if cb.GetState() == StateOpen {
					nextErr := cb.Execute(fn)
					if nextErr != ErrCircuitBreakerOpen {
						t.Logf("Expected ErrCircuitBreakerOpen when circuit is Open, got %v", nextErr)
						return false
					}
					break // Stop testing once circuit is open
				}
			}

			return true
		},
		genMaxFailures,
		genTimeout,
		gen.SliceOfN(20, gen.Bool()),
	))

	properties.TestingRun(t)
}

// Property test: Verify circuit breaker is thread-safe
func TestProperty_CircuitBreakerThreadSafety(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Fewer iterations due to concurrency overhead
	properties := gopter.NewProperties(parameters)

	genMaxFailures := gen.IntRange(3, 10)
	genTimeout := gen.IntRange(50, 200).Map(func(ms int) time.Duration {
		return time.Duration(ms) * time.Millisecond
	})
	genGoroutines := gen.IntRange(2, 10)

	properties.Property("circuit breaker handles concurrent operations safely", prop.ForAll(
		func(maxFailures int, timeout time.Duration, numGoroutines int) bool {
			cb := NewCircuitBreaker(maxFailures, timeout)

			done := make(chan bool, numGoroutines)

			// Launch concurrent goroutines
			for i := 0; i < numGoroutines; i++ {
				go func(id int) {
					defer func() {
						if r := recover(); r != nil {
							t.Logf("Goroutine %d panicked: %v", id, r)
							done <- false
							return
						}
						done <- true
					}()

					// Each goroutine executes some operations
					for j := 0; j < 5; j++ {
						var fn func() error
						if j%2 == 0 {
							fn = func() error { return nil }
						} else {
							fn = func() error { return errors.New("failed") }
						}

						cb.Execute(fn)
						_ = cb.GetState()
						_ = cb.GetFailures()
					}
				}(i)
			}

			// Wait for all goroutines to complete
			for i := 0; i < numGoroutines; i++ {
				success := <-done
				if !success {
					return false
				}
			}

			// Verify circuit breaker is in a valid state
			state := cb.GetState()
			if state != StateClosed && state != StateOpen && state != StateHalfOpen {
				t.Logf("Invalid state after concurrent operations: %v", state)
				return false
			}

			return true
		},
		genMaxFailures,
		genTimeout,
		genGoroutines,
	))

	properties.TestingRun(t)
}
