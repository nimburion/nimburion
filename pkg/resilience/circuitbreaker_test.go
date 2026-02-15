package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	if cb.GetState() != StateClosed {
		t.Errorf("expected initial state to be Closed, got %v", cb.GetState())
	}

	if cb.GetFailures() != 0 {
		t.Errorf("expected initial failures to be 0, got %d", cb.GetFailures())
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	failingFn := func() error {
		return errors.New("operation failed")
	}

	// Execute failing function 3 times
	for i := 0; i < 3; i++ {
		err := cb.Execute(failingFn)
		if err == nil {
			t.Error("expected error from failing function")
		}
	}

	// Circuit should now be open
	if cb.GetState() != StateOpen {
		t.Errorf("expected state to be Open after %d failures, got %v", 3, cb.GetState())
	}

	// Next execution should fail immediately
	err := cb.Execute(failingFn)
	if err != ErrCircuitBreakerOpen {
		t.Errorf("expected ErrCircuitBreakerOpen, got %v", err)
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	timeout := 50 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	failingFn := func() error {
		return errors.New("operation failed")
	}

	// Trigger circuit to open
	cb.Execute(failingFn)
	cb.Execute(failingFn)

	if cb.GetState() != StateOpen {
		t.Errorf("expected state to be Open, got %v", cb.GetState())
	}

	// Wait for timeout
	time.Sleep(timeout + 10*time.Millisecond)

	// Next execution should transition to half-open
	successFn := func() error {
		return nil
	}

	err := cb.Execute(successFn)
	if err != nil {
		t.Errorf("expected successful execution in half-open state, got error: %v", err)
	}

	// Should now be closed
	if cb.GetState() != StateClosed {
		t.Errorf("expected state to be Closed after successful half-open execution, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	timeout := 50 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	failingFn := func() error {
		return errors.New("operation failed")
	}

	// Open the circuit
	cb.Execute(failingFn)
	cb.Execute(failingFn)

	// Wait for timeout to transition to half-open
	time.Sleep(timeout + 10*time.Millisecond)

	successFn := func() error {
		return nil
	}

	// Successful execution in half-open should close the circuit
	err := cb.Execute(successFn)
	if err != nil {
		t.Errorf("expected successful execution, got error: %v", err)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state to be Closed, got %v", cb.GetState())
	}

	// Failures should be reset
	if cb.GetFailures() != 0 {
		t.Errorf("expected failures to be reset to 0, got %d", cb.GetFailures())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	timeout := 50 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	failingFn := func() error {
		return errors.New("operation failed")
	}

	// Open the circuit
	cb.Execute(failingFn)
	cb.Execute(failingFn)

	// Wait for timeout to transition to half-open
	time.Sleep(timeout + 10*time.Millisecond)

	// Failure in half-open should reopen the circuit
	err := cb.Execute(failingFn)
	if err == nil {
		t.Error("expected error from failing function")
	}

	if cb.GetState() != StateOpen {
		t.Errorf("expected state to be Open after failure in half-open, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	failingFn := func() error {
		return errors.New("operation failed")
	}

	successFn := func() error {
		return nil
	}

	// Execute failing function twice
	cb.Execute(failingFn)
	cb.Execute(failingFn)

	if cb.GetFailures() != 2 {
		t.Errorf("expected 2 failures, got %d", cb.GetFailures())
	}

	// Execute successful function
	err := cb.Execute(successFn)
	if err != nil {
		t.Errorf("expected successful execution, got error: %v", err)
	}

	// Failures should be reset
	if cb.GetFailures() != 0 {
		t.Errorf("expected failures to be reset to 0, got %d", cb.GetFailures())
	}

	// Circuit should still be closed
	if cb.GetState() != StateClosed {
		t.Errorf("expected state to be Closed, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	failingFn := func() error {
		return errors.New("operation failed")
	}

	// Open the circuit
	cb.Execute(failingFn)
	cb.Execute(failingFn)

	if cb.GetState() != StateOpen {
		t.Errorf("expected state to be Open, got %v", cb.GetState())
	}

	// Reset the circuit breaker
	cb.Reset()

	if cb.GetState() != StateClosed {
		t.Errorf("expected state to be Closed after reset, got %v", cb.GetState())
	}

	if cb.GetFailures() != 0 {
		t.Errorf("expected failures to be 0 after reset, got %d", cb.GetFailures())
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
