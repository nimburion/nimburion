# Redis Cache Adapter

This package provides a Redis adapter for cache operations with connection pooling, health checks, and graceful shutdown.

## Features

- Connection pooling with configurable limits
- Health checks for Redis connectivity
- Graceful connection cleanup on shutdown
- Operation timeouts
- Key expiration (TTL) support
- Atomic operations (INCR, DECR, etc.)

## Usage

### Creating a Redis Adapter

```go
import (
    "time"
    "github.com/nimburion/nimburion/pkg/store/redis"
    "github.com/nimburion/nimburion/pkg/observability/logger"
)

cfg := redis.Config{
    URL:              "redis://localhost:6379/0",
    MaxConns:         10,
    OperationTimeout: 5 * time.Second,
}

log := logger.NewZapLogger(logger.Config{
    Level:  "info",
    Format: "json",
})

adapter, err := redis.NewRedisAdapter(cfg, log)
if err != nil {
    log.Fatal("failed to create redis adapter", "error", err)
}
defer adapter.Close()
```

### Basic Cache Operations

```go
ctx := context.Background()

// Set a value
err := adapter.Set(ctx, "user:123", "John Doe")

// Get a value
value, err := adapter.Get(ctx, "user:123")

// Delete a key
err := adapter.Delete(ctx, "user:123")
```

### Cache Operations with TTL

```go
// Set a value with 1 hour expiration
err := adapter.SetWithTTL(ctx, "session:abc", "user-data", 1*time.Hour)

// Get the value before expiration
value, err := adapter.Get(ctx, "session:abc")

// After TTL expires, Get will return "key not found" error
```

### Atomic Operations

```go
// Increment a counter
count, err := adapter.Incr(ctx, "page:views")

// Increment by specific amount
count, err := adapter.IncrBy(ctx, "page:views", 10)

// Decrement a counter
count, err := adapter.Decr(ctx, "inventory:item123")

// Decrement by specific amount
count, err := adapter.DecrBy(ctx, "inventory:item123", 5)
```

### Health Checks

```go
// Check if Redis is healthy
err := adapter.HealthCheck(ctx)
if err != nil {
    log.Error("redis is unhealthy", "error", err)
}
```

## Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| URL | string | Redis connection URL (required) | - |
| MaxConns | int | Maximum number of connections in pool | 10 |
| OperationTimeout | time.Duration | Timeout for Redis operations | 5s |

### URL Format

The Redis URL follows the standard format:
```
redis://[username:password@]host:port[/database]
```

Examples:
- `redis://localhost:6379/0` - Local Redis, database 0
- `redis://:password@localhost:6379/1` - With password, database 1
- `redis://user:pass@redis.example.com:6379/0` - With username and password

## Error Handling

The adapter returns descriptive errors for common scenarios:

- **Connection errors**: Returned during adapter creation if Redis is unreachable
- **Key not found**: Returned by `Get()` when the key doesn't exist
- **Operation errors**: Returned when Redis operations fail

Example:
```go
value, err := adapter.Get(ctx, "nonexistent")
if err != nil {
    if strings.Contains(err.Error(), "key not found") {
        // Handle missing key
    } else {
        // Handle other errors
    }
}
```

## Thread Safety

The Redis adapter is thread-safe and can be used concurrently from multiple goroutines. The underlying go-redis client handles connection pooling and synchronization automatically.

## Requirements

This adapter satisfies the following framework requirements:

- **21.1**: Provides Redis adapter for cache operations
- **21.2**: Uses factory function NewRedisAdapter(config, logger)
- **21.3**: Supports connection pooling with configurable limits
- **21.4**: Implements health checks for Redis connections
- **21.5**: Supports graceful connection cleanup on shutdown
- **21.6**: Implements operation timeouts
- **21.7**: Supports key expiration (TTL)
- **21.8**: Supports atomic operations (INCR, DECR, etc.)
