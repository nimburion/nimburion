# Health Check Package

The health check package provides a flexible and extensible system for monitoring the health of microservice components and dependencies.

## Features

- **Registry-based**: Central registry for managing multiple health checks
- **Concurrent execution**: All health checks run concurrently for fast results
- **Flexible checkers**: Support for adapter checks, custom checks, composite checks, and more
- **Status levels**: Healthy, Unhealthy, and Degraded status support
- **Aggregation**: Automatic aggregation of multiple check results
- **Context-aware**: All checks support context for timeouts and cancellation
- **Type-safe**: Strongly typed interfaces and results

## Core Concepts

### Health Status

Three status levels are supported:

- `StatusHealthy`: Component is fully operational
- `StatusDegraded`: Component is operational but with reduced functionality
- `StatusUnhealthy`: Component is not operational

### Check Result

Every health check returns a `CheckResult` containing:

- `Name`: Identifier for the check
- `Status`: Health status (healthy, unhealthy, degraded)
- `Message`: Optional human-readable message
- `Error`: Optional error message when unhealthy
- `Timestamp`: When the check was performed
- `Duration`: How long the check took
- `Metadata`: Optional additional context

### Checker Interface

All health checks implement the `Checker` interface:

```go
type Checker interface {
    Check(ctx context.Context) CheckResult
    Name() string
}
```

## Usage

### Basic Setup

```go
import "github.com/nimburion/nimburion/pkg/health"

// Create a registry
registry := health.NewRegistry()

// Register a simple liveness check
registry.Register(health.NewPingChecker("liveness"))

// Run all checks
ctx := context.Background()
result := registry.Check(ctx)

if result.IsHealthy() {
    fmt.Println("All systems operational")
}
```

### Adapter Health Checks

For components that implement the `HealthCheckable` interface:

```go
// Assuming db implements HealthCheck(ctx context.Context) error
dbChecker := health.NewAdapterChecker("database", db, 5*time.Second)
registry.Register(dbChecker)

// Assuming cache implements HealthCheck(ctx context.Context) error
cacheChecker := health.NewAdapterChecker("cache", cache, 5*time.Second)
registry.Register(cacheChecker)
```

### Dependency Health Checks

Convenience functions for common dependency types with sensible defaults:

```go
// Database health check (5 second timeout)
dbChecker := health.NewDatabaseChecker("postgres", postgresAdapter)
registry.Register(dbChecker)

// Cache health check (3 second timeout)
cacheChecker := health.NewCacheChecker("redis", redisAdapter)
registry.Register(cacheChecker)

// Message broker health check (5 second timeout)
brokerChecker := health.NewMessageBrokerChecker("kafka", kafkaAdapter)
registry.Register(brokerChecker)
```

These convenience functions are equivalent to using `NewAdapterChecker` with pre-configured timeouts appropriate for each dependency type.

### Custom Health Checks

Register custom logic using functions:

```go
registry.RegisterFunc("disk-space", func(ctx context.Context) health.CheckResult {
    freeSpace := checkDiskSpace() // Your custom logic
    
    if freeSpace < 10 {
        return health.CheckResult{
            Name:      "disk-space",
            Status:    health.StatusUnhealthy,
            Error:     "disk space critically low",
            Timestamp: time.Now(),
        }
    }
    
    return health.CheckResult{
        Name:      "disk-space",
        Status:    health.StatusHealthy,
        Message:   fmt.Sprintf("%d%% free", freeSpace),
        Timestamp: time.Now(),
    }
})
```

### Composite Checks

Combine multiple checks into one:

```go
dbChecker := health.NewAdapterChecker("database", db, 5*time.Second)
cacheChecker := health.NewAdapterChecker("cache", cache, 5*time.Second)

// Create a composite that checks both
dataLayer := health.NewCompositeChecker("data-layer", dbChecker, cacheChecker)
registry.Register(dataLayer)
```

### Check Individual Components

```go
// Check only the database
result, err := registry.CheckOne(ctx, "database")
if err != nil {
    log.Printf("Check not found: %v", err)
}

if result.Status == health.StatusHealthy {
    fmt.Println("Database is healthy")
}
```

### List Registered Checks

```go
checks := registry.List()
fmt.Printf("Registered checks: %v\n", checks)
```

## Built-in Checkers

### PingChecker

Always returns healthy - useful for liveness probes:

```go
checker := health.NewPingChecker("liveness")
```

### AdapterChecker

Wraps any component implementing `HealthCheckable`:

```go
checker := health.NewAdapterChecker("database", db, 5*time.Second)
```

### Dependency Checkers

Convenience functions for common dependencies:

```go
// Database checker with 5 second timeout
dbChecker := health.NewDatabaseChecker("postgres", postgresAdapter)

// Cache checker with 3 second timeout
cacheChecker := health.NewCacheChecker("redis", redisAdapter)

// Message broker checker with 5 second timeout
brokerChecker := health.NewMessageBrokerChecker("kafka", kafkaAdapter)
```

### CompositeChecker

Combines multiple checkers:

```go
checker := health.NewCompositeChecker("services", checker1, checker2, checker3)
```

### CustomChecker

For advanced custom logic:

```go
checker := health.NewCustomChecker("custom", func(ctx context.Context) (health.Status, string, error) {
    // Your custom logic
    if someCondition {
        return health.StatusHealthy, "All good", nil
    }
    return health.StatusUnhealthy, "Problem detected", errors.New("details")
})
```

## HTTP Integration

Typical usage in HTTP handlers:

```go
// Liveness endpoint - always returns 200 if service is running
func livenessHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// Readiness endpoint - checks dependencies
func readinessHandler(registry *health.Registry) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        result := registry.Check(r.Context())
        
        if result.IsHealthy() {
            w.WriteHeader(http.StatusOK)
        } else {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        
        json.NewEncoder(w).Encode(result)
    }
}
```

## Best Practices

1. **Separate liveness and readiness**: Use `PingChecker` for liveness, dependency checks for readiness
2. **Set appropriate timeouts**: Health checks should be fast (< 5 seconds)
3. **Use meaningful names**: Check names should clearly identify the component
4. **Include metadata**: Add relevant context to help with debugging
5. **Run checks concurrently**: The registry handles this automatically
6. **Handle context cancellation**: Respect context timeouts in custom checks
7. **Log failures**: Include error details in check results
8. **Monitor check duration**: Use the duration field to identify slow checks

## Thread Safety

The registry is thread-safe and can be safely accessed from multiple goroutines. All operations (Register, Unregister, Check, CheckOne, List) use appropriate locking.

## Performance

- Health checks run concurrently using goroutines
- Registry operations use read-write locks for efficiency
- Check results include timing information for monitoring

## Requirements Validation

This implementation validates:

- **Requirement 30.7**: Support custom health check registration ✓
- **Requirement 30.7**: Aggregate health check results ✓
- **Requirement 30.5**: Check all critical dependencies ✓
- **Requirement 30.6**: Return HTTP 503 when dependencies are unhealthy ✓

## Examples

See `example_test.go` for complete working examples of all features.
