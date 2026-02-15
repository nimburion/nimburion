package resilience

import (
	"context"
	"errors"
	"time"
)

// ErrTimeout is returned when an operation exceeds its timeout
var ErrTimeout = errors.New("operation timed out")

// WithTimeout executes the given function with a timeout
// If the function does not complete within the timeout duration, it returns ErrTimeout
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Channel to receive the result
	done := make(chan error, 1)

	// Execute the function in a goroutine
	go func() {
		done <- fn(timeoutCtx)
	}()

	// Wait for either completion or timeout
	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return ErrTimeout
		}
		return timeoutCtx.Err()
	}
}

// TimeoutFunc wraps a function to enforce a timeout
type TimeoutFunc struct {
	timeout time.Duration
}

// NewTimeoutFunc creates a new timeout function wrapper
func NewTimeoutFunc(timeout time.Duration) *TimeoutFunc {
	return &TimeoutFunc{
		timeout: timeout,
	}
}

// Execute runs the given function with the configured timeout
func (tf *TimeoutFunc) Execute(ctx context.Context, fn func(context.Context) error) error {
	return WithTimeout(ctx, tf.timeout, fn)
}

// ExecuteWithCustomTimeout runs the given function with a custom timeout
func (tf *TimeoutFunc) ExecuteWithCustomTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	return WithTimeout(ctx, timeout, fn)
}
