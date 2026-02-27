package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestPingChecker tests the ping checker implementation
func TestPingChecker(t *testing.T) {
	checker := NewPingChecker("ping")
	
	if checker.Name() != "ping" {
		t.Errorf("Expected name 'ping', got '%s'", checker.Name())
	}
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}
	
	if result.Name != "ping" {
		t.Errorf("Expected result name 'ping', got '%s'", result.Name)
	}
	
	if result.Message == "" {
		t.Error("Expected message to be set")
	}
}

// TestCompositeChecker tests the composite checker implementation
func TestCompositeChecker(t *testing.T) {
	t.Run("all sub-checks healthy", func(t *testing.T) {
		checker1 := &mockChecker{
			name: "sub-check-1",
			result: CheckResult{
				Name:   "sub-check-1",
				Status: StatusHealthy,
			},
		}
		
		checker2 := &mockChecker{
			name: "sub-check-2",
			result: CheckResult{
				Name:   "sub-check-2",
				Status: StatusHealthy,
			},
		}
		
		composite := NewCompositeChecker("composite", checker1, checker2)
		
		ctx := context.Background()
		result := composite.Check(ctx)
		
		if result.Status != StatusHealthy {
			t.Errorf("Expected status healthy, got %s", result.Status)
		}
		
		if result.Name != "composite" {
			t.Errorf("Expected name 'composite', got '%s'", result.Name)
		}
		
		if composite.Name() != "composite" {
			t.Errorf("Expected Name() to return 'composite', got '%s'", composite.Name())
		}
	})
	
	t.Run("one sub-check unhealthy", func(t *testing.T) {
		checker1 := &mockChecker{
			name: "sub-check-1",
			result: CheckResult{
				Name:   "sub-check-1",
				Status: StatusHealthy,
			},
		}
		
		checker2 := &mockChecker{
			name: "sub-check-2",
			result: CheckResult{
				Name:   "sub-check-2",
				Status: StatusUnhealthy,
				Error:  "failed",
			},
		}
		
		composite := NewCompositeChecker("composite", checker1, checker2)
		
		ctx := context.Background()
		result := composite.Check(ctx)
		
		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy, got %s", result.Status)
		}
		
		if result.Error == "" {
			t.Error("Expected error message to be set")
		}
	})
	
	t.Run("one sub-check degraded", func(t *testing.T) {
		checker1 := &mockChecker{
			name: "sub-check-1",
			result: CheckResult{
				Name:   "sub-check-1",
				Status: StatusHealthy,
			},
		}
		
		checker2 := &mockChecker{
			name: "sub-check-2",
			result: CheckResult{
				Name:   "sub-check-2",
				Status: StatusDegraded,
			},
		}
		
		composite := NewCompositeChecker("composite", checker1, checker2)
		
		ctx := context.Background()
		result := composite.Check(ctx)
		
		if result.Status != StatusDegraded {
			t.Errorf("Expected status degraded, got %s", result.Status)
		}
	})
	
	t.Run("unhealthy takes precedence over degraded", func(t *testing.T) {
		checker1 := &mockChecker{
			name: "sub-check-1",
			result: CheckResult{
				Name:   "sub-check-1",
				Status: StatusDegraded,
			},
		}
		
		checker2 := &mockChecker{
			name: "sub-check-2",
			result: CheckResult{
				Name:   "sub-check-2",
				Status: StatusUnhealthy,
				Error:  "failed",
			},
		}
		
		composite := NewCompositeChecker("composite", checker1, checker2)
		
		ctx := context.Background()
		result := composite.Check(ctx)
		
		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy (takes precedence), got %s", result.Status)
		}
	})
	
	t.Run("empty composite", func(t *testing.T) {
		composite := NewCompositeChecker("composite")
		
		ctx := context.Background()
		result := composite.Check(ctx)
		
		if result.Status != StatusHealthy {
			t.Errorf("Expected empty composite to be healthy, got %s", result.Status)
		}
	})
}

// TestCustomChecker tests the custom checker implementation
func TestCustomChecker(t *testing.T) {
	t.Run("healthy check", func(t *testing.T) {
		checkFunc := func(ctx context.Context) (Status, string, error) {
			return StatusHealthy, "All good", nil
		}
		
		checker := NewCustomChecker("custom", checkFunc)
		
		if checker.Name() != "custom" {
			t.Errorf("Expected name 'custom', got '%s'", checker.Name())
		}
		
		ctx := context.Background()
		result := checker.Check(ctx)
		
		if result.Status != StatusHealthy {
			t.Errorf("Expected status healthy, got %s", result.Status)
		}
		
		if result.Message != "All good" {
			t.Errorf("Expected message 'All good', got '%s'", result.Message)
		}
		
		if result.Error != "" {
			t.Errorf("Expected no error, got '%s'", result.Error)
		}
	})
	
	t.Run("unhealthy check with error", func(t *testing.T) {
		checkFunc := func(ctx context.Context) (Status, string, error) {
			return StatusUnhealthy, "Something wrong", errors.New("connection failed")
		}
		
		checker := NewCustomChecker("custom", checkFunc)
		
		ctx := context.Background()
		result := checker.Check(ctx)
		
		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy, got %s", result.Status)
		}
		
		if result.Message != "Something wrong" {
			t.Errorf("Expected message 'Something wrong', got '%s'", result.Message)
		}
		
		if result.Error == "" {
			t.Error("Expected error to be set")
		}
	})
	
	t.Run("degraded check", func(t *testing.T) {
		checkFunc := func(ctx context.Context) (Status, string, error) {
			return StatusDegraded, "Partially available", nil
		}
		
		checker := NewCustomChecker("custom", checkFunc)
		
		ctx := context.Background()
		result := checker.Check(ctx)
		
		if result.Status != StatusDegraded {
			t.Errorf("Expected status degraded, got %s", result.Status)
		}
		
		if result.Message != "Partially available" {
			t.Errorf("Expected message 'Partially available', got '%s'", result.Message)
		}
	})
}

// slowCheckable is a mock that simulates slow health checks
type slowCheckable struct {
	delay time.Duration
}

func (s *slowCheckable) HealthCheck(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.delay):
		return nil
	}
}

// TestAdapterChecker_Timeout tests that adapter checker respects timeout
func TestAdapterChecker_Timeout(t *testing.T) {
	// Create an adapter that takes longer than the timeout
	slowAdapter := &slowCheckable{
		delay: 200 * time.Millisecond,
	}
	
	checker := NewAdapterChecker("slow-adapter", slowAdapter, 50*time.Millisecond)
	
	ctx := context.Background()
	start := time.Now()
	result := checker.Check(ctx)
	duration := time.Since(start)
	
	// Check should timeout and return unhealthy
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected status unhealthy due to timeout, got %s", result.Status)
	}
	
	// Duration should be close to timeout, not the full 200ms
	if duration > 100*time.Millisecond {
		t.Errorf("Check took too long: %v, expected ~50ms", duration)
	}
	
	if result.Error == "" {
		t.Error("Expected error message for timeout")
	}
}

// TestCheckerFunc tests the CheckerFunc type
func TestCheckerFunc(t *testing.T) {
	checkFunc := CheckerFunc(func(ctx context.Context) CheckResult {
		return CheckResult{
			Name:   "test",
			Status: StatusHealthy,
		}
	})
	
	ctx := context.Background()
	result := checkFunc.Check(ctx)
	
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}
	
	if checkFunc.Name() != "anonymous" {
		t.Errorf("Expected name 'anonymous', got '%s'", checkFunc.Name())
	}
}

// TestCheckResult_Timestamp tests that check results include timestamps
func TestCheckResult_Timestamp(t *testing.T) {
	checker := NewPingChecker("ping")
	
	ctx := context.Background()
	before := time.Now()
	result := checker.Check(ctx)
	after := time.Now()
	
	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("Timestamp %v is outside expected range [%v, %v]", result.Timestamp, before, after)
	}
}

// TestCheckResult_Duration tests that check results include duration
func TestCheckResult_Duration(t *testing.T) {
	adapter := &mockCheckable{err: nil}
	adapterChecker := NewAdapterChecker("test", adapter, 5*time.Second)
	
	ctx := context.Background()
	result := adapterChecker.Check(ctx)
	
	if result.Duration <= 0 {
		t.Errorf("Expected positive duration, got %v", result.Duration)
	}
}
// TestNewDatabaseChecker tests the database checker convenience function
func TestNewDatabaseChecker(t *testing.T) {
	adapter := &mockCheckable{err: nil}
	checker := NewDatabaseChecker("postgres", adapter)

	if checker.Name() != "postgres" {
		t.Errorf("Expected name 'postgres', got '%s'", checker.Name())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}

	if result.Name != "postgres" {
		t.Errorf("Expected result name 'postgres', got '%s'", result.Name)
	}
}

// TestNewCacheChecker tests the cache checker convenience function
func TestNewCacheChecker(t *testing.T) {
	adapter := &mockCheckable{err: nil}
	checker := NewCacheChecker("redis", adapter)

	if checker.Name() != "redis" {
		t.Errorf("Expected name 'redis', got '%s'", checker.Name())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}

	if result.Name != "redis" {
		t.Errorf("Expected result name 'redis', got '%s'", result.Name)
	}
}

// TestNewMessageBrokerChecker tests the message broker checker convenience function
func TestNewMessageBrokerChecker(t *testing.T) {
	adapter := &mockCheckable{err: nil}
	checker := NewMessageBrokerChecker("kafka", adapter)

	if checker.Name() != "kafka" {
		t.Errorf("Expected name 'kafka', got '%s'", checker.Name())
	}

	ctx := context.Background()
	result := checker.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}

	if result.Name != "kafka" {
		t.Errorf("Expected result name 'kafka', got '%s'", result.Name)
	}
}

// TestDependencyCheckers_WithFailures tests dependency checkers with failures
func TestDependencyCheckers_WithFailures(t *testing.T) {
	t.Run("database failure", func(t *testing.T) {
		adapter := &mockCheckable{err: errors.New("connection refused")}
		checker := NewDatabaseChecker("postgres", adapter)

		ctx := context.Background()
		result := checker.Check(ctx)

		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy, got %s", result.Status)
		}

		if result.Error == "" {
			t.Error("Expected error message to be set")
		}
	})

	t.Run("cache failure", func(t *testing.T) {
		adapter := &mockCheckable{err: errors.New("redis unavailable")}
		checker := NewCacheChecker("redis", adapter)

		ctx := context.Background()
		result := checker.Check(ctx)

		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy, got %s", result.Status)
		}

		if result.Error == "" {
			t.Error("Expected error message to be set")
		}
	})

	t.Run("message broker failure", func(t *testing.T) {
		adapter := &mockCheckable{err: errors.New("kafka broker down")}
		checker := NewMessageBrokerChecker("kafka", adapter)

		ctx := context.Background()
		result := checker.Check(ctx)

		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy, got %s", result.Status)
		}

		if result.Error == "" {
			t.Error("Expected error message to be set")
		}
	})
}

