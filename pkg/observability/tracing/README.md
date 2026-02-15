# Tracing Package

The tracing package provides OpenTelemetry distributed tracing support for the Go microservices framework. It enables end-to-end request tracing across services, databases, and message brokers.

## Features

- **OTLP Exporter**: Exports traces to OpenTelemetry collectors
- **Configurable Sampling**: Control trace sampling rate to manage overhead
- **Context Propagation**: Automatic trace context propagation across service boundaries
- **HTTP Tracing**: Middleware for tracing HTTP requests
- **Database Tracing**: Utilities for tracing database operations
- **Messaging Tracing**: Utilities for tracing message broker operations
- **Cache Tracing**: Utilities for tracing cache operations

## Requirements

This package implements the following requirements:
- 14.1: OpenTelemetry tracing support
- 14.2: Trace context propagation across service boundaries
- 14.3: Spans for HTTP requests
- 14.4: Spans for database operations
- 14.5: Spans for message broker operations
- 14.6: Request ID in span attributes
- 14.7: Configurable trace sampling rates

## Usage

### Initialize Tracer Provider

```go
import (
    "context"
    "github.com/nimburion/nimburion/pkg/observability/tracing"
)

func main() {
    ctx := context.Background()
    
    // Create tracer provider
    tracerProvider, err := tracing.NewTracerProvider(ctx, tracing.TracerConfig{
        ServiceName:    "my-service",
        ServiceVersion: "1.0.0",
        Environment:    "production",
        Endpoint:       "localhost:4317",
        SampleRate:     0.1, // Sample 10% of traces
        Enabled:        true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer tracerProvider.Shutdown(ctx)
    
    // Tracer provider is now set globally
}
```

### HTTP Tracing Middleware

```go
import (
    "github.com/nimburion/nimburion/pkg/middleware"
    "github.com/nimburion/nimburion/pkg/server/router"
)

func setupRouter(r router.Router) {
    // Add tracing middleware
    r.Use(middleware.Tracing(middleware.TracingConfig{
        TracerName: "my-service-http",
    }))
    
    // Add routes
    r.GET("/users", handleGetUsers)
}
```

### Database Operation Tracing

```go
import (
    "context"
    "github.com/nimburion/nimburion/pkg/observability/tracing"
)

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    // Start database span
    ctx, span := tracing.StartDatabaseSpan(ctx, tracing.SpanOperationDBQuery,
        tracing.WithDBSystem("postgresql"),
        tracing.WithDBTable("users"),
        tracing.WithDBStatement("SELECT * FROM users WHERE id = $1"),
    )
    defer span.End()
    
    // Execute query
    var user User
    err := r.db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = $1", id).Scan(&user)
    if err != nil {
        tracing.RecordError(span, err)
        return nil, err
    }
    
    tracing.RecordSuccess(span)
    return &user, nil
}
```

### Message Broker Operation Tracing

```go
import (
    "context"
    "github.com/nimburion/nimburion/pkg/observability/tracing"
)

func (p *KafkaProducer) Publish(ctx context.Context, topic string, message []byte) error {
    // Start messaging span
    ctx, span := tracing.StartMessagingSpan(ctx, tracing.SpanOperationMsgPublish,
        tracing.WithMessagingSystem("kafka"),
        tracing.WithMessagingDestination(topic),
        tracing.WithMessagingPayloadSize(len(message)),
    )
    defer span.End()
    
    // Publish message
    err := p.writer.WriteMessages(ctx, kafka.Message{
        Topic: topic,
        Value: message,
    })
    if err != nil {
        tracing.RecordError(span, err)
        return err
    }
    
    tracing.RecordSuccess(span)
    return nil
}
```

### Cache Operation Tracing

```go
import (
    "context"
    "github.com/nimburion/nimburion/pkg/observability/tracing"
)

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
    // Start cache span
    ctx, span := tracing.StartCacheSpan(ctx, tracing.SpanOperationCacheGet,
        tracing.WithCacheSystem("redis"),
        tracing.WithCacheKey(key),
    )
    defer span.End()
    
    // Get value from cache
    val, err := c.client.Get(ctx, key).Result()
    if err == redis.Nil {
        // Cache miss
        span.SetAttributes(attribute.Bool("cache.hit", false))
        return "", nil
    } else if err != nil {
        tracing.RecordError(span, err)
        return "", err
    }
    
    // Cache hit
    span.SetAttributes(attribute.Bool("cache.hit", true))
    tracing.RecordSuccess(span)
    return val, nil
}
```

### Propagating Trace Context

When making HTTP calls to other services, propagate the trace context:

```go
import (
    "context"
    "net/http"
    "github.com/nimburion/nimburion/pkg/middleware"
)

func callExternalService(ctx context.Context, url string) error {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return err
    }
    
    // Propagate trace context in headers
    headers := make(map[string]string)
    middleware.PropagateTraceContext(ctx, headers)
    for k, v := range headers {
        req.Header.Set(k, v)
    }
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

## Configuration

### Environment Variables

- `APP_TRACING_ENABLED`: Enable/disable tracing (default: true)
- `APP_TRACING_ENDPOINT`: OTLP collector endpoint (default: localhost:4317)
- `APP_TRACING_SAMPLE_RATE`: Trace sampling rate 0.0-1.0 (default: 0.1)

### Sampling Strategies

The framework uses trace ID ratio-based sampling:
- `1.0`: Sample all traces (100%)
- `0.1`: Sample 10% of traces
- `0.01`: Sample 1% of traces
- `0.0`: Disable sampling (no traces)

For production environments, start with a low sampling rate (0.01-0.1) and adjust based on traffic volume and observability needs.

## Best Practices

1. **Always defer span.End()**: Ensure spans are closed even if errors occur
2. **Record errors**: Use `tracing.RecordError()` to capture error details
3. **Add meaningful attributes**: Include relevant context in span attributes
4. **Use appropriate span kinds**: Client for outgoing calls, Server for incoming requests
5. **Propagate context**: Always pass context through function calls
6. **Sample appropriately**: Balance observability needs with performance overhead

## Integration with Other Components

The tracing package integrates seamlessly with:
- **Logger**: Request IDs are included in both logs and traces
- **Metrics**: Traces can be correlated with metrics using exemplars
- **Middleware**: Automatic tracing for all HTTP requests

## Performance Considerations

- Tracing adds minimal overhead when properly configured
- Use sampling to control the volume of traces
- OTLP exporter batches spans for efficient export
- Spans are exported asynchronously to avoid blocking requests
- Consider using tail-based sampling for more sophisticated strategies
