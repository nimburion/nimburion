package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	Name      string                 `json:"name"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Checker is the interface that health check implementations must satisfy
type Checker interface {
	// Check performs the health check and returns the result
	Check(ctx context.Context) CheckResult
	
	// Name returns the name of the health check
	Name() string
}

// CheckerFunc is a function type that implements the Checker interface
type CheckerFunc func(ctx context.Context) CheckResult

// Check implements the Checker interface
func (f CheckerFunc) Check(ctx context.Context) CheckResult {
	return f(ctx)
}

// Name returns a default name for function-based checkers
func (f CheckerFunc) Name() string {
	return "anonymous"
}

// Registry manages a collection of health checks
type Registry struct {
	checkers map[string]Checker
	mu       sync.RWMutex
}

// NewRegistry creates a new health check registry
func NewRegistry() *Registry {
	return &Registry{
		checkers: make(map[string]Checker),
	}
}

// Register adds a health check to the registry
// If a checker with the same name already exists, it will be replaced
func (r *Registry) Register(checker Checker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.checkers[checker.Name()] = checker
}

// RegisterFunc registers a function-based health check with a given name
func (r *Registry) RegisterFunc(name string, checkFunc func(ctx context.Context) CheckResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.checkers[name] = &namedChecker{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Unregister removes a health check from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.checkers, name)
}

// Check runs all registered health checks and returns aggregated results
// If any check fails, the overall status is unhealthy
func (r *Registry) Check(ctx context.Context) AggregatedResult {
	r.mu.RLock()
	checkers := make([]Checker, 0, len(r.checkers))
	for _, checker := range r.checkers {
		checkers = append(checkers, checker)
	}
	r.mu.RUnlock()
	
	start := time.Now()
	results := make([]CheckResult, 0, len(checkers))
	overallStatus := StatusHealthy
	
	// Run all checks concurrently
	resultsChan := make(chan CheckResult, len(checkers))
	var wg sync.WaitGroup
	
	for _, checker := range checkers {
		wg.Add(1)
		go func(c Checker) {
			defer wg.Done()
			result := c.Check(ctx)
			resultsChan <- result
		}(checker)
	}
	
	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		results = append(results, result)
		
		// Determine overall status
		if result.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if result.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}
	
	return AggregatedResult{
		Status:    overallStatus,
		Checks:    results,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
}

// CheckOne runs a specific health check by name
func (r *Registry) CheckOne(ctx context.Context, name string) (CheckResult, error) {
	r.mu.RLock()
	checker, exists := r.checkers[name]
	r.mu.RUnlock()
	
	if !exists {
		return CheckResult{}, fmt.Errorf("health check not found: %s", name)
	}
	
	return checker.Check(ctx), nil
}

// List returns the names of all registered health checks
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.checkers))
	for name := range r.checkers {
		names = append(names, name)
	}
	
	return names
}

// AggregatedResult represents the aggregated result of all health checks
type AggregatedResult struct {
	Status    Status        `json:"status"`
	Checks    []CheckResult `json:"checks"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// IsHealthy returns true if the overall status is healthy
func (r AggregatedResult) IsHealthy() bool {
	return r.Status == StatusHealthy
}

// namedChecker wraps a function-based checker with a name
type namedChecker struct {
	name      string
	checkFunc func(ctx context.Context) CheckResult
}

func (c *namedChecker) Check(ctx context.Context) CheckResult {
	return c.checkFunc(ctx)
}

func (c *namedChecker) Name() string {
	return c.name
}
