package health

import (
	"context"
	"fmt"
	"time"
)

// Checkable is an interface for components that support health checks
type Checkable interface {
	HealthCheck(ctx context.Context) error
}

// AdapterChecker creates a health checker for any component that implements Checkable
type AdapterChecker struct {
	name    string
	adapter Checkable
	timeout time.Duration
}

// NewAdapterChecker creates a new health checker for an adapter
func NewAdapterChecker(name string, adapter Checkable, timeout time.Duration) *AdapterChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	
	return &AdapterChecker{
		name:    name,
		adapter: adapter,
		timeout: timeout,
	}
}

// Check performs the health check on the adapter
func (c *AdapterChecker) Check(ctx context.Context) CheckResult {
	start := time.Now()
	
	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	
	// Perform health check
	err := c.adapter.HealthCheck(checkCtx)
	duration := time.Since(start)
	
	if err != nil {
		return CheckResult{
			Name:      c.name,
			Status:    StatusUnhealthy,
			Error:     err.Error(),
			Timestamp: time.Now(),
			Duration:  duration,
		}
	}
	
	return CheckResult{
		Name:      c.name,
		Status:    StatusHealthy,
		Message:   "OK",
		Timestamp: time.Now(),
		Duration:  duration,
	}
}

// Name returns the name of the health check
func (c *AdapterChecker) Name() string {
	return c.name
}

// PingChecker creates a simple health checker that always returns healthy
// Useful for liveness checks
type PingChecker struct {
	name string
}

// NewPingChecker creates a new ping checker
func NewPingChecker(name string) *PingChecker {
	return &PingChecker{
		name: name,
	}
}

// Check always returns healthy status
func (c *PingChecker) Check(ctx context.Context) CheckResult {
	return CheckResult{
		Name:      c.name,
		Status:    StatusHealthy,
		Message:   "Service is alive",
		Timestamp: time.Now(),
		Duration:  0,
	}
}

// Name returns the name of the health check
func (c *PingChecker) Name() string {
	return c.name
}

// CompositeChecker combines multiple checkers into one
// All sub-checks must pass for the composite check to be healthy
type CompositeChecker struct {
	name     string
	checkers []Checker
}

// NewCompositeChecker creates a new composite checker
func NewCompositeChecker(name string, checkers ...Checker) *CompositeChecker {
	return &CompositeChecker{
		name:     name,
		checkers: checkers,
	}
}

// Check runs all sub-checks and aggregates the results
func (c *CompositeChecker) Check(ctx context.Context) CheckResult {
	start := time.Now()
	status := StatusHealthy
	var errors []string
	
	for _, checker := range c.checkers {
		result := checker.Check(ctx)
		
		if result.Status == StatusUnhealthy {
			status = StatusUnhealthy
			if result.Error != "" {
				errors = append(errors, fmt.Sprintf("%s: %s", result.Name, result.Error))
			}
		} else if result.Status == StatusDegraded && status == StatusHealthy {
			status = StatusDegraded
		}
	}
	
	result := CheckResult{
		Name:      c.name,
		Status:    status,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
	
	if len(errors) > 0 {
		result.Error = fmt.Sprintf("sub-checks failed: %v", errors)
	} else if status == StatusHealthy {
		result.Message = "All sub-checks passed"
	}
	
	return result
}

// Name returns the name of the health check
func (c *CompositeChecker) Name() string {
	return c.name
}

// CustomChecker allows creating a health checker from a custom function
type CustomChecker struct {
	name      string
	checkFunc func(ctx context.Context) (Status, string, error)
}

// NewCustomChecker creates a new custom health checker
// The checkFunc should return (status, message, error)
func NewCustomChecker(name string, checkFunc func(ctx context.Context) (Status, string, error)) *CustomChecker {
	return &CustomChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Check executes the custom check function
func (c *CustomChecker) Check(ctx context.Context) CheckResult {
	start := time.Now()
	
	status, message, err := c.checkFunc(ctx)
	
	result := CheckResult{
		Name:      c.name,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
	
	if err != nil {
		result.Error = err.Error()
	}
	
	return result
}

// Name returns the name of the health check
func (c *CustomChecker) Name() string {
	return c.name
}
// NewDatabaseChecker creates a health checker for a database adapter
// This is a convenience function for creating an AdapterChecker with database-specific defaults
func NewDatabaseChecker(name string, db Checkable) *AdapterChecker {
	return NewAdapterChecker(name, db, 5*time.Second)
}

// NewCacheChecker creates a health checker for a cache adapter
// This is a convenience function for creating an AdapterChecker with cache-specific defaults
func NewCacheChecker(name string, cache Checkable) *AdapterChecker {
	return NewAdapterChecker(name, cache, 3*time.Second)
}

// NewMessageBrokerChecker creates a health checker for a message broker adapter
// This is a convenience function for creating an AdapterChecker with message broker-specific defaults
func NewMessageBrokerChecker(name string, broker Checkable) *AdapterChecker {
	return NewAdapterChecker(name, broker, 5*time.Second)
}

