package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithTimeout_Success(t *testing.T) {
	ctx := context.Background()
	timeout := 100 * time.Millisecond

	fn := func(ctx context.Context) error {
		// Fast operation
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	err := WithTimeout(ctx, timeout, fn)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestWithTimeout_Timeout(t *testing.T) {
	ctx := context.Background()
	timeout := 50 * time.Millisecond

	fn := func(ctx context.Context) error {
		// Slow operation that exceeds timeout
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	err := WithTimeout(ctx, timeout, fn)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestWithTimeout_FunctionError(t *testing.T) {
	ctx := context.Background()
	timeout := 100 * time.Millisecond

	expectedErr := errors.New("function error")
	fn := func(ctx context.Context) error {
		return expectedErr
	}

	err := WithTimeout(ctx, timeout, fn)
	if err != expectedErr {
		t.Errorf("expected function error, got %v", err)
	}
}

func TestWithTimeout_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	timeout := 100 * time.Millisecond

	fn := func(ctx context.Context) error {
		// Wait for context cancellation
		<-ctx.Done()
		return ctx.Err()
	}

	// Cancel the context immediately
	cancel()

	err := WithTimeout(ctx, timeout, fn)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestWithTimeout_RespectsFunctionContext(t *testing.T) {
	ctx := context.Background()
	timeout := 100 * time.Millisecond

	fn := func(ctx context.Context) error {
		// Check if context has deadline
		deadline, ok := ctx.Deadline()
		if !ok {
			return errors.New("context should have deadline")
		}

		// Verify deadline is approximately correct
		expectedDeadline := time.Now().Add(timeout)
		diff := deadline.Sub(expectedDeadline)
		if diff < -10*time.Millisecond || diff > 10*time.Millisecond {
			return errors.New("deadline is not approximately correct")
		}

		return nil
	}

	err := WithTimeout(ctx, timeout, fn)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTimeoutFunc_Execute(t *testing.T) {
	tf := NewTimeoutFunc(100 * time.Millisecond)
	ctx := context.Background()

	fn := func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	err := tf.Execute(ctx, fn)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTimeoutFunc_ExecuteTimeout(t *testing.T) {
	tf := NewTimeoutFunc(50 * time.Millisecond)
	ctx := context.Background()

	fn := func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	err := tf.Execute(ctx, fn)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestTimeoutFunc_ExecuteWithCustomTimeout(t *testing.T) {
	tf := NewTimeoutFunc(100 * time.Millisecond)
	ctx := context.Background()

	fn := func(ctx context.Context) error {
		time.Sleep(75 * time.Millisecond)
		return nil
	}

	// Use custom timeout that's shorter than default
	err := tf.ExecuteWithCustomTimeout(ctx, 50*time.Millisecond, fn)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout with custom timeout, got %v", err)
	}

	// Use custom timeout that's longer than operation
	err = tf.ExecuteWithCustomTimeout(ctx, 150*time.Millisecond, fn)
	if err != nil {
		t.Errorf("expected no error with longer custom timeout, got %v", err)
	}
}

func TestWithTimeout_ZeroTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := 0 * time.Millisecond

	fn := func(ctx context.Context) error {
		return nil
	}

	// Zero timeout should timeout immediately
	err := WithTimeout(ctx, timeout, fn)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout with zero timeout, got %v", err)
	}
}

func TestWithTimeout_NegativeTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := -1 * time.Millisecond

	fn := func(ctx context.Context) error {
		return nil
	}

	// Negative timeout should timeout immediately
	err := WithTimeout(ctx, timeout, fn)
	if err != ErrTimeout {
		t.Errorf("expected ErrTimeout with negative timeout, got %v", err)
	}
}
