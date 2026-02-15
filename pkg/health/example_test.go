package health_test

import (
	"context"
	"fmt"
	"time"

	"github.com/nimburion/nimburion/pkg/health"
)

// mockDatabase simulates a database adapter with health check support
type mockDatabase struct {
	connected bool
}

func (db *mockDatabase) HealthCheck(ctx context.Context) error {
	if !db.connected {
		return fmt.Errorf("database not connected")
	}
	return nil
}

// mockCache simulates a cache adapter with health check support
type mockCache struct {
	available bool
}

func (c *mockCache) HealthCheck(ctx context.Context) error {
	if !c.available {
		return fmt.Errorf("cache unavailable")
	}
	return nil
}

// Example_basicUsage demonstrates basic health check registry usage
func Example_basicUsage() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Register a simple ping check (always healthy)
	registry.Register(health.NewPingChecker("liveness"))

	// Run all health checks
	ctx := context.Background()
	result := registry.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result.Status)
	fmt.Printf("Number of Checks: %d\n", len(result.Checks))
	fmt.Printf("Is Healthy: %v\n", result.IsHealthy())

	// Output:
	// Overall Status: healthy
	// Number of Checks: 1
	// Is Healthy: true
}

// Example_adapterChecks demonstrates registering adapter health checks
func Example_adapterChecks() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Create mock adapters
	db := &mockDatabase{connected: true}
	cache := &mockCache{available: true}

	// Register adapter health checks
	registry.Register(health.NewAdapterChecker("database", db, 5*time.Second))
	registry.Register(health.NewAdapterChecker("cache", cache, 5*time.Second))

	// Run all health checks
	ctx := context.Background()
	result := registry.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result.Status)
	fmt.Printf("Number of Checks: %d\n", len(result.Checks))

	// Output:
	// Overall Status: healthy
	// Number of Checks: 2
}

// Example_customCheck demonstrates registering a custom health check
func Example_customCheck() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Register a custom health check using a function
	registry.RegisterFunc("disk-space", func(ctx context.Context) health.CheckResult {
		// Simulate checking disk space
		freeSpacePercent := 75

		if freeSpacePercent < 10 {
			return health.CheckResult{
				Name:      "disk-space",
				Status:    health.StatusUnhealthy,
				Error:     "disk space critically low",
				Timestamp: time.Now(),
			}
		} else if freeSpacePercent < 20 {
			return health.CheckResult{
				Name:      "disk-space",
				Status:    health.StatusDegraded,
				Message:   "disk space running low",
				Timestamp: time.Now(),
			}
		}

		return health.CheckResult{
			Name:      "disk-space",
			Status:    health.StatusHealthy,
			Message:   fmt.Sprintf("%d%% free", freeSpacePercent),
			Timestamp: time.Now(),
		}
	})

	// Run all health checks
	ctx := context.Background()
	result := registry.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result.Status)

	// Output:
	// Overall Status: healthy
}

// Example_compositeCheck demonstrates using composite health checks
func Example_compositeCheck() {
	// Create individual checkers
	db := &mockDatabase{connected: true}
	cache := &mockCache{available: true}

	dbChecker := health.NewAdapterChecker("database", db, 5*time.Second)
	cacheChecker := health.NewAdapterChecker("cache", cache, 5*time.Second)

	// Create a composite checker that combines them
	composite := health.NewCompositeChecker("data-layer", dbChecker, cacheChecker)

	// Create registry and register the composite
	registry := health.NewRegistry()
	registry.Register(composite)

	// Run all health checks
	ctx := context.Background()
	result := registry.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result.Status)

	// Output:
	// Overall Status: healthy
}

// Example_checkOne demonstrates checking a specific health check
func Example_checkOne() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Register multiple checks
	registry.Register(health.NewPingChecker("liveness"))

	db := &mockDatabase{connected: true}
	registry.Register(health.NewAdapterChecker("database", db, 5*time.Second))

	// Check only the database
	ctx := context.Background()
	result, err := registry.CheckOne(ctx, "database")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Check Name: %s\n", result.Name)
	fmt.Printf("Status: %s\n", result.Status)

	// Output:
	// Check Name: database
	// Status: healthy
}

// Example_listChecks demonstrates listing registered health checks
func Example_listChecks() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Register multiple checks
	registry.Register(health.NewPingChecker("liveness"))

	db := &mockDatabase{connected: true}
	registry.Register(health.NewAdapterChecker("database", db, 5*time.Second))

	cache := &mockCache{available: true}
	registry.Register(health.NewAdapterChecker("cache", cache, 5*time.Second))

	// List all registered checks
	checks := registry.List()

	fmt.Printf("Number of registered checks: %d\n", len(checks))

	// Output:
	// Number of registered checks: 3
}

// Example_unhealthyCheck demonstrates handling unhealthy checks
func Example_unhealthyCheck() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Register a healthy check
	healthyDB := &mockDatabase{connected: true}
	registry.Register(health.NewAdapterChecker("database", healthyDB, 5*time.Second))

	// Register an unhealthy check
	unhealthyCache := &mockCache{available: false}
	registry.Register(health.NewAdapterChecker("cache", unhealthyCache, 5*time.Second))

	// Run all health checks
	ctx := context.Background()
	result := registry.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result.Status)
	fmt.Printf("Is Healthy: %v\n", result.IsHealthy())

	// Check individual results
	for _, check := range result.Checks {
		if check.Status == health.StatusUnhealthy {
			fmt.Printf("Unhealthy Check: %s - %s\n", check.Name, check.Error)
		}
	}

	// Output:
	// Overall Status: unhealthy
	// Is Healthy: false
	// Unhealthy Check: cache - cache unavailable
}

// Example_dependencyChecks demonstrates using convenience functions for dependency health checks
func Example_dependencyChecks() {
	// Create a new health check registry
	registry := health.NewRegistry()

	// Create mock adapters
	db := &mockDatabase{connected: true}
	cache := &mockCache{available: true}

	// Mock message broker (using database as placeholder for example)
	broker := &mockDatabase{connected: true}

	// Register dependency health checks using convenience functions
	registry.Register(health.NewDatabaseChecker("postgres", db))
	registry.Register(health.NewCacheChecker("redis", cache))
	registry.Register(health.NewMessageBrokerChecker("kafka", broker))

	// Run all health checks
	ctx := context.Background()
	result := registry.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result.Status)
	fmt.Printf("Number of Checks: %d\n", len(result.Checks))

	// Output:
	// Overall Status: healthy
	// Number of Checks: 3
}
