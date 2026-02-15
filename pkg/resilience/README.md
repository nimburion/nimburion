# Resilience Package

The resilience package provides patterns for building fault-tolerant and resilient microservices. It includes implementations of the Circuit Breaker pattern and timeout enforcement utilities.

## Features

- **Circuit Breaker**: Prevents cascading failures by failing fast when error rates exceed thresholds
- **Timeout Enforcement**: Ensures operations complete within specified time limits using context-based cancellation

## Circuit Breaker

The Circuit Breaker pattern prevents cascading failures by tracking failure rates and temporarily blocking requests when failures exceed a threshold.

### States

- **Closed**: Normal operation, all requests pass through
- **Open**: Failure threshold exceeded, requests fail immediately
- **Half-Open**: Testing recovery, limited requests allowed through

### Usage

```go
import "github.com/nimburion/nimburion/pkg/resilience"

// Create a circuit breaker with max 3 failures and 5 second timeout
cb := resilience.NewCircuitBreaker(3, 5*time.Second)

// Execute an operation
err := cb.Execute(func() error {
    // Your operation here
    return doSomething()
})

if err == resilience.ErrCircuitBreakerOpen {
    // Circuit is open, handle accordingly
}
```

### State Transitions

1. **Closed → Open**: When consecutive failures reach `maxFailures`
2. **Open → Half-Open**: After `timeout` duration elapses
3. **Half-Open → Closed**: When a request succeeds
4. **Half-Open → Open**: When a request fails

### Methods

- `Execute(fn func() error) error`: Execute a function with circuit breaker protection
- `GetState() State`: Get the current circuit breaker state
- `GetFailures() int`: Get the current failure count
- `Reset()`: Manually reset the circuit breaker to closed state

## Timeout Enforcement

Timeout enforcement ensures operations complete within specified time limits using Go's context package.

### Usage

#### Simple Timeout

```go
import (
    "context"
    "time"
    "github.com/nimburion/nimburion/pkg/resilience"
)

ctx := context.Background()
timeout := 5 * time.Second

err := resilience.WithTimeout(ctx, timeout, func(ctx context.Context) error {
    // Your operation here - must respect ctx.Done()
    return doSomethingWithContext(ctx)
})

if err == resilience.ErrTimeout {
    // Operation timed out
}
```

#### Reusable Timeout Function

```go
// Create a timeout function with default timeout
tf := resilience.NewTimeoutFunc(5 * time.Second)

// Execute with default timeout
err := tf.Execute(ctx, func(ctx context.Context) error {
    return doSomething(ctx)
})

// Execute with custom timeout
err = tf.ExecuteWithCustomTimeout(ctx, 10*time.Second, func(ctx context.Context) error {
    return doSomethingElse(ctx)
})
```

### Best Practices

1. **Always respect context cancellation**: Operations should check `ctx.Done()` and return promptly
2. **Use appropriate timeouts**: Set timeouts based on expected operation duration plus buffer
3. **Handle timeout errors**: Distinguish between timeout errors and other errors
4. **Propagate context**: Pass the context to all downstream operations

## Combining Patterns

Circuit breakers and timeouts work well together:

```go
cb := resilience.NewCircuitBreaker(3, 5*time.Second)
tf := resilience.NewTimeoutFunc(2 * time.Second)

err := cb.Execute(func() error {
    return tf.Execute(ctx, func(ctx context.Context) error {
        return doSomething(ctx)
    })
})
```

This ensures:
1. Operations timeout if they take too long
2. Circuit opens if too many operations fail (including timeouts)
3. System fails fast when circuit is open

## Error Types

- `ErrCircuitBreakerOpen`: Returned when circuit breaker is in open state
- `ErrTimeout`: Returned when an operation exceeds its timeout

## Thread Safety

Both Circuit Breaker and Timeout utilities are thread-safe and can be used concurrently from multiple goroutines.

## Testing

The package includes comprehensive unit tests and property-based tests:

```bash
# Run all tests
go test ./pkg/resilience/...

# Run only unit tests
go test ./pkg/resilience/... -run 'Test[^P]'

# Run only property tests
go test ./pkg/resilience/... -run 'Property'
```

## Requirements

This package implements:
- Requirements 32.1-32.8 (Timeout Configuration)
- Requirements 33.1-33.8 (Circuit Breaker Pattern)

## Examples

See the test files for comprehensive examples:
- `circuitbreaker_test.go`: Unit test examples
- `circuitbreaker_property_test.go`: Property-based test examples
- `timeout_test.go`: Timeout unit test examples
- `timeout_property_test.go`: Timeout property-based test examples
