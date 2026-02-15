package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockChecker is a mock implementation of Checker for testing
type mockChecker struct {
	name   string
	result CheckResult
	delay  time.Duration
}

func (m *mockChecker) Check(ctx context.Context) CheckResult {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.result
}

func (m *mockChecker) Name() string {
	return m.name
}

// TestNewRegistry tests registry creation
func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	
	if registry.checkers == nil {
		t.Error("Registry checkers map is nil")
	}
	
	if len(registry.List()) != 0 {
		t.Errorf("New registry should have 0 checkers, got %d", len(registry.List()))
	}
}

// TestRegistry_Register tests registering health checks
func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	
	checker1 := &mockChecker{
		name: "test-checker-1",
		result: CheckResult{
			Name:   "test-checker-1",
			Status: StatusHealthy,
		},
	}
	
	checker2 := &mockChecker{
		name: "test-checker-2",
		result: CheckResult{
			Name:   "test-checker-2",
			Status: StatusHealthy,
		},
	}
	
	// Register first checker
	registry.Register(checker1)
	
	names := registry.List()
	if len(names) != 1 {
		t.Errorf("Expected 1 checker, got %d", len(names))
	}
	
	// Register second checker
	registry.Register(checker2)
	
	names = registry.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 checkers, got %d", len(names))
	}
	
	// Register checker with same name (should replace)
	checker1Updated := &mockChecker{
		name: "test-checker-1",
		result: CheckResult{
			Name:   "test-checker-1",
			Status: StatusUnhealthy,
		},
	}
	
	registry.Register(checker1Updated)
	
	names = registry.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 checkers after replacement, got %d", len(names))
	}
}

// TestRegistry_RegisterFunc tests registering function-based health checks
func TestRegistry_RegisterFunc(t *testing.T) {
	registry := NewRegistry()
	
	checkFunc := func(ctx context.Context) CheckResult {
		return CheckResult{
			Name:   "func-checker",
			Status: StatusHealthy,
		}
	}
	
	registry.RegisterFunc("func-checker", checkFunc)
	
	names := registry.List()
	if len(names) != 1 {
		t.Errorf("Expected 1 checker, got %d", len(names))
	}
	
	if names[0] != "func-checker" {
		t.Errorf("Expected checker name 'func-checker', got '%s'", names[0])
	}
}

// TestRegistry_Unregister tests unregistering health checks
func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()
	
	checker := &mockChecker{
		name: "test-checker",
		result: CheckResult{
			Name:   "test-checker",
			Status: StatusHealthy,
		},
	}
	
	registry.Register(checker)
	
	if len(registry.List()) != 1 {
		t.Errorf("Expected 1 checker before unregister, got %d", len(registry.List()))
	}
	
	registry.Unregister("test-checker")
	
	if len(registry.List()) != 0 {
		t.Errorf("Expected 0 checkers after unregister, got %d", len(registry.List()))
	}
	
	// Unregister non-existent checker (should not panic)
	registry.Unregister("non-existent")
}

// TestRegistry_Check_AllHealthy tests checking when all checks are healthy
func TestRegistry_Check_AllHealthy(t *testing.T) {
	registry := NewRegistry()
	
	registry.Register(&mockChecker{
		name: "checker-1",
		result: CheckResult{
			Name:   "checker-1",
			Status: StatusHealthy,
		},
	})
	
	registry.Register(&mockChecker{
		name: "checker-2",
		result: CheckResult{
			Name:   "checker-2",
			Status: StatusHealthy,
		},
	})
	
	ctx := context.Background()
	result := registry.Check(ctx)
	
	if result.Status != StatusHealthy {
		t.Errorf("Expected overall status to be healthy, got %s", result.Status)
	}
	
	if len(result.Checks) != 2 {
		t.Errorf("Expected 2 check results, got %d", len(result.Checks))
	}
	
	if !result.IsHealthy() {
		t.Error("IsHealthy() should return true when status is healthy")
	}
}

// TestRegistry_Check_OneUnhealthy tests checking when one check is unhealthy
func TestRegistry_Check_OneUnhealthy(t *testing.T) {
	registry := NewRegistry()
	
	registry.Register(&mockChecker{
		name: "healthy-checker",
		result: CheckResult{
			Name:   "healthy-checker",
			Status: StatusHealthy,
		},
	})
	
	registry.Register(&mockChecker{
		name: "unhealthy-checker",
		result: CheckResult{
			Name:   "unhealthy-checker",
			Status: StatusUnhealthy,
			Error:  "connection failed",
		},
	})
	
	ctx := context.Background()
	result := registry.Check(ctx)
	
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected overall status to be unhealthy, got %s", result.Status)
	}
	
	if len(result.Checks) != 2 {
		t.Errorf("Expected 2 check results, got %d", len(result.Checks))
	}
	
	if result.IsHealthy() {
		t.Error("IsHealthy() should return false when status is unhealthy")
	}
}

// TestRegistry_Check_Degraded tests checking when one check is degraded
func TestRegistry_Check_Degraded(t *testing.T) {
	registry := NewRegistry()
	
	registry.Register(&mockChecker{
		name: "healthy-checker",
		result: CheckResult{
			Name:   "healthy-checker",
			Status: StatusHealthy,
		},
	})
	
	registry.Register(&mockChecker{
		name: "degraded-checker",
		result: CheckResult{
			Name:   "degraded-checker",
			Status: StatusDegraded,
		},
	})
	
	ctx := context.Background()
	result := registry.Check(ctx)
	
	if result.Status != StatusDegraded {
		t.Errorf("Expected overall status to be degraded, got %s", result.Status)
	}
	
	if result.IsHealthy() {
		t.Error("IsHealthy() should return false when status is degraded")
	}
}

// TestRegistry_Check_UnhealthyTakesPrecedence tests that unhealthy status takes precedence over degraded
func TestRegistry_Check_UnhealthyTakesPrecedence(t *testing.T) {
	registry := NewRegistry()
	
	registry.Register(&mockChecker{
		name: "degraded-checker",
		result: CheckResult{
			Name:   "degraded-checker",
			Status: StatusDegraded,
		},
	})
	
	registry.Register(&mockChecker{
		name: "unhealthy-checker",
		result: CheckResult{
			Name:   "unhealthy-checker",
			Status: StatusUnhealthy,
		},
	})
	
	ctx := context.Background()
	result := registry.Check(ctx)
	
	if result.Status != StatusUnhealthy {
		t.Errorf("Expected overall status to be unhealthy (takes precedence), got %s", result.Status)
	}
}

// TestRegistry_Check_EmptyRegistry tests checking an empty registry
func TestRegistry_Check_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()
	
	ctx := context.Background()
	result := registry.Check(ctx)
	
	if result.Status != StatusHealthy {
		t.Errorf("Expected empty registry to be healthy, got %s", result.Status)
	}
	
	if len(result.Checks) != 0 {
		t.Errorf("Expected 0 check results, got %d", len(result.Checks))
	}
}

// TestRegistry_Check_Concurrent tests that checks run concurrently
func TestRegistry_Check_Concurrent(t *testing.T) {
	registry := NewRegistry()
	
	// Add checkers with delays
	delay := 100 * time.Millisecond
	
	for i := 0; i < 3; i++ {
		registry.Register(&mockChecker{
			name:  "slow-checker",
			delay: delay,
			result: CheckResult{
				Name:   "slow-checker",
				Status: StatusHealthy,
			},
		})
	}
	
	ctx := context.Background()
	start := time.Now()
	result := registry.Check(ctx)
	duration := time.Since(start)
	
	// If checks run concurrently, total time should be ~delay, not 3*delay
	maxExpectedDuration := delay + 50*time.Millisecond // Add buffer for overhead
	
	if duration > maxExpectedDuration {
		t.Errorf("Checks appear to run sequentially. Duration: %v, expected < %v", duration, maxExpectedDuration)
	}
	
	if result.Status != StatusHealthy {
		t.Errorf("Expected overall status to be healthy, got %s", result.Status)
	}
}

// TestRegistry_CheckOne tests checking a specific health check
func TestRegistry_CheckOne(t *testing.T) {
	registry := NewRegistry()
	
	registry.Register(&mockChecker{
		name: "checker-1",
		result: CheckResult{
			Name:   "checker-1",
			Status: StatusHealthy,
		},
	})
	
	registry.Register(&mockChecker{
		name: "checker-2",
		result: CheckResult{
			Name:   "checker-2",
			Status: StatusUnhealthy,
		},
	})
	
	ctx := context.Background()
	
	// Check existing checker
	result, err := registry.CheckOne(ctx, "checker-1")
	if err != nil {
		t.Errorf("CheckOne() returned unexpected error: %v", err)
	}
	
	if result.Name != "checker-1" {
		t.Errorf("Expected result name 'checker-1', got '%s'", result.Name)
	}
	
	if result.Status != StatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}
	
	// Check non-existent checker
	_, err = registry.CheckOne(ctx, "non-existent")
	if err == nil {
		t.Error("CheckOne() should return error for non-existent checker")
	}
}

// TestRegistry_List tests listing registered health checks
func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()
	
	expectedNames := []string{"checker-1", "checker-2", "checker-3"}
	
	for _, name := range expectedNames {
		registry.Register(&mockChecker{
			name: name,
			result: CheckResult{
				Name:   name,
				Status: StatusHealthy,
			},
		})
	}
	
	names := registry.List()
	
	if len(names) != len(expectedNames) {
		t.Errorf("Expected %d names, got %d", len(expectedNames), len(names))
	}
	
	// Check all expected names are present
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}
	
	for _, expected := range expectedNames {
		if !nameMap[expected] {
			t.Errorf("Expected name '%s' not found in list", expected)
		}
	}
}

// TestRegistry_Check_ContextCancellation tests that context cancellation is respected
func TestRegistry_Check_ContextCancellation(t *testing.T) {
	registry := NewRegistry()
	
	// Add a checker that respects context
	registry.RegisterFunc("context-aware", func(ctx context.Context) CheckResult {
		select {
		case <-ctx.Done():
			return CheckResult{
				Name:   "context-aware",
				Status: StatusUnhealthy,
				Error:  ctx.Err().Error(),
			}
		case <-time.After(100 * time.Millisecond):
			return CheckResult{
				Name:   "context-aware",
				Status: StatusHealthy,
			}
		}
	})
	
	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	result := registry.Check(ctx)
	
	// The check should complete even with cancelled context
	// (individual checkers decide how to handle cancellation)
	if len(result.Checks) != 1 {
		t.Errorf("Expected 1 check result, got %d", len(result.Checks))
	}
}

// TestAggregatedResult_IsHealthy tests the IsHealthy helper method
func TestAggregatedResult_IsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected bool
	}{
		{
			name:     "healthy status",
			status:   StatusHealthy,
			expected: true,
		},
		{
			name:     "unhealthy status",
			status:   StatusUnhealthy,
			expected: false,
		},
		{
			name:     "degraded status",
			status:   StatusDegraded,
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggregatedResult{
				Status: tt.status,
			}
			
			if result.IsHealthy() != tt.expected {
				t.Errorf("IsHealthy() = %v, expected %v for status %s", result.IsHealthy(), tt.expected, tt.status)
			}
		})
	}
}

// mockHealthCheckable is a mock implementation of HealthCheckable for testing
type mockHealthCheckable struct {
	err error
}

func (m *mockHealthCheckable) HealthCheck(ctx context.Context) error {
	return m.err
}

// TestAdapterChecker tests the adapter checker implementation
func TestAdapterChecker(t *testing.T) {
	t.Run("healthy adapter", func(t *testing.T) {
		adapter := &mockHealthCheckable{err: nil}
		checker := NewAdapterChecker("test-adapter", adapter, 5*time.Second)
		
		ctx := context.Background()
		result := checker.Check(ctx)
		
		if result.Status != StatusHealthy {
			t.Errorf("Expected status healthy, got %s", result.Status)
		}
		
		if result.Name != "test-adapter" {
			t.Errorf("Expected name 'test-adapter', got '%s'", result.Name)
		}
		
		if checker.Name() != "test-adapter" {
			t.Errorf("Expected Name() to return 'test-adapter', got '%s'", checker.Name())
		}
	})
	
	t.Run("unhealthy adapter", func(t *testing.T) {
		adapter := &mockHealthCheckable{err: errors.New("connection failed")}
		checker := NewAdapterChecker("test-adapter", adapter, 5*time.Second)
		
		ctx := context.Background()
		result := checker.Check(ctx)
		
		if result.Status != StatusUnhealthy {
			t.Errorf("Expected status unhealthy, got %s", result.Status)
		}
		
		if result.Error == "" {
			t.Error("Expected error message to be set")
		}
	})
	
	t.Run("default timeout", func(t *testing.T) {
		adapter := &mockHealthCheckable{err: nil}
		checker := NewAdapterChecker("test-adapter", adapter, 0)
		
		if checker.timeout != 5*time.Second {
			t.Errorf("Expected default timeout 5s, got %v", checker.timeout)
		}
	})
}
