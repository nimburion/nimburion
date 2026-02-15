# Metrics Package

The metrics package provides Prometheus metrics integration for the Go microservices framework.

## Features

- **HTTP Metrics**: Automatic tracking of HTTP request duration, count, and in-flight requests
- **Go Runtime Metrics**: Automatic collection of goroutines, memory usage, and GC statistics
- **Custom Metrics**: Support for registering application-specific metrics
- **Prometheus Format**: Standard Prometheus exposition format via `/metrics` endpoint

## Requirements

Implements requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7

## Usage

### Basic Setup

```go
import (
    "net/http"
    "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// Create a metrics registry
registry := metrics.NewRegistry()

// Expose metrics on management server
http.Handle("/metrics", registry.Handler())
```

### HTTP Metrics Middleware

The framework automatically records HTTP metrics when using the metrics middleware:

```go
import (
    "github.com/nimburion/nimburion/pkg/middleware"
)

// Apply metrics middleware to your router
router.Use(middleware.Metrics())
```

This tracks:
- `http_request_duration_seconds` - Histogram of request durations (labels: method, path, status)
- `http_requests_total` - Counter of total requests (labels: method, path, status)
- `http_requests_in_flight` - Gauge of current in-flight requests

### Custom Metrics

Register your own application-specific metrics:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// Create custom metrics
ordersProcessed := prometheus.NewCounter(prometheus.CounterOpts{
    Name: "orders_processed_total",
    Help: "Total number of orders processed",
})

orderValue := prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "order_value_dollars",
    Help: "Order value in dollars",
    Buckets: prometheus.LinearBuckets(0, 10, 20), // 0-200 in steps of 10
})

// Register with the metrics registry
registry := metrics.NewRegistry()
registry.MustRegister(ordersProcessed, orderValue)

// Use in your application
ordersProcessed.Inc()
orderValue.Observe(125.50)
```

### Go Runtime Metrics

The registry automatically includes Go runtime metrics:

- `go_goroutines` - Number of goroutines
- `go_threads` - Number of OS threads
- `go_memstats_*` - Memory statistics (heap, stack, GC)
- `go_gc_duration_seconds` - GC pause durations
- `process_cpu_seconds_total` - Process CPU time
- `process_resident_memory_bytes` - Process memory usage

## Default Metrics

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_request_duration_seconds` | Histogram | method, path, status | Request duration in seconds |
| `http_requests_total` | Counter | method, path, status | Total number of requests |
| `http_requests_in_flight` | Gauge | - | Current number of requests being processed |

### Runtime Metrics

All standard Go collector metrics are included automatically. See [Prometheus Go client documentation](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors) for details.

## Testing

The package includes comprehensive unit tests:

```bash
go test ./pkg/observability/metrics/...
```

## Architecture

```
pkg/observability/metrics/
├── registry.go      # Metrics registry with custom registration
├── http.go          # HTTP-specific metrics
├── registry_test.go # Unit tests for registry
└── README.md        # This file
```

## Best Practices

1. **Create one registry per application**: Use a single registry instance throughout your application
2. **Register metrics at startup**: Register all custom metrics during application initialization
3. **Use appropriate metric types**:
   - Counter: Monotonically increasing values (requests, errors)
   - Gauge: Values that can go up or down (in-flight requests, queue size)
   - Histogram: Distributions of values (request duration, response size)
   - Summary: Similar to histogram but with quantiles
4. **Choose meaningful labels**: Labels create separate time series, so use them judiciously
5. **Avoid high cardinality**: Don't use user IDs or request IDs as labels
6. **Expose on management port**: Keep metrics on the management server, not the public API

## Example: Complete Setup

```go
package main

import (
    "net/http"
    "github.com/nimburion/nimburion/pkg/observability/metrics"
    "github.com/nimburion/nimburion/pkg/middleware"
    "github.com/nimburion/nimburion/pkg/server/router"
)

func main() {
    // Create metrics registry
    metricsRegistry := metrics.NewRegistry()
    
    // Create routers
    publicRouter := router.NewRouter()
    mgmtRouter := router.NewRouter()
    
    // Apply metrics middleware to public API
    publicRouter.Use(middleware.Metrics())
    
    // Expose metrics on management server
    mgmtRouter.GET("/metrics", func(c router.Context) error {
        metricsRegistry.Handler().ServeHTTP(c.Response(), c.Request())
        return nil
    })
    
    // Start servers...
}
```
