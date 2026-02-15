# PostgreSQL Adapter

The PostgreSQL adapter provides a production-ready database connection with connection pooling, health checks, graceful shutdown, and transaction management.

## Features

- **Connection Pooling**: Configurable connection pool with max open/idle connections and connection lifetime
- **Health Checks**: Built-in health check support for orchestrators (Kubernetes, Docker)
- **Transaction Management**: Full transaction support with automatic commit/rollback
- **Graceful Shutdown**: Proper connection cleanup on application shutdown
- **Context Support**: Transaction context propagation for nested operations
- **Query Timeouts**: Configurable timeouts for all database operations

## Usage

### Basic Setup

```go
import (
    "github.com/nimburion/nimburion/pkg/store/postgres"
    "github.com/nimburion/nimburion/pkg/observability/logger"
)

// Create logger
log, _ := logger.NewZapLogger(logger.Config{
    Level:  logger.InfoLevel,
    Format: logger.JSONFormat,
})

// Configure adapter
cfg := postgres.Config{
    URL:             "postgres://user:password@localhost:5432/mydb?sslmode=disable",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
    QueryTimeout:    10 * time.Second,
}

// Create adapter
adapter, err := postgres.NewPostgreSQLAdapter(cfg, log)
if err != nil {
    log.Fatal("Failed to create adapter", "error", err)
}
defer adapter.Close()
```

### Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `URL` | string | required | PostgreSQL connection string |
| `MaxOpenConns` | int | 25 | Maximum number of open connections |
| `MaxIdleConns` | int | 5 | Maximum number of idle connections |
| `ConnMaxLifetime` | duration | 5m | Maximum connection lifetime |
| `QueryTimeout` | duration | 10s | Default query timeout |

### Environment Variables

Configuration can be provided via environment variables with the `APP_DB_` prefix:

```bash
APP_DB_URL=postgres://user:password@localhost:5432/mydb
APP_DB_MAX_OPEN_CONNS=25
APP_DB_MAX_IDLE_CONNS=5
APP_DB_CONN_MAX_LIFETIME=5m
APP_DB_QUERY_TIMEOUT=10s
```

### Health Checks

The adapter provides a health check method for use with orchestrators:

```go
ctx := context.Background()
if err := adapter.HealthCheck(ctx); err != nil {
    log.Error("Database health check failed", "error", err)
}
```

### Transactions

The adapter supports transactions with automatic commit/rollback:

```go
ctx := context.Background()

err := adapter.WithTransaction(ctx, func(txCtx context.Context) error {
    // Execute queries within transaction
    _, err := adapter.ExecContext(txCtx, 
        "INSERT INTO users (name, email) VALUES ($1, $2)", 
        "John Doe", "john@example.com")
    if err != nil {
        return err // Triggers rollback
    }
    
    // More operations...
    
    return nil // Triggers commit
})

if err != nil {
    log.Error("Transaction failed", "error", err)
}
```

### Nested Operations

Transaction context is automatically propagated to nested operations:

```go
err := adapter.WithTransaction(ctx, func(txCtx context.Context) error {
    // First operation
    _, err := adapter.ExecContext(txCtx, "INSERT INTO orders (...) VALUES (...)")
    if err != nil {
        return err
    }
    
    // Nested operation using same transaction
    _, err = adapter.ExecContext(txCtx, "INSERT INTO order_items (...) VALUES (...)")
    return err
})
```

### Query Operations

The adapter provides context-aware query methods:

```go
// Execute query
result, err := adapter.ExecContext(ctx, 
    "UPDATE users SET status = $1 WHERE id = $2", 
    "active", userID)

// Query multiple rows
rows, err := adapter.QueryContext(ctx, 
    "SELECT id, name FROM users WHERE status = $1", 
    "active")
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        return err
    }
    // Process row...
}

// Query single row
var count int
err = adapter.QueryRowContext(ctx, 
    "SELECT COUNT(*) FROM users").Scan(&count)
```

### Direct Database Access

For advanced use cases, you can access the underlying `*sql.DB`:

```go
db := adapter.DB()
// Use standard database/sql methods
```

## Testing

The package includes three types of tests:

### Unit Tests

Basic validation tests that don't require a database:

```bash
go test ./pkg/store/postgres -short
```

### Property Tests

Property-based tests that verify the adapter contract:

```bash
go test ./pkg/store/postgres -run TestProperty14
```

### Integration Tests

Full integration tests using testcontainers (requires Docker):

```bash
go test ./pkg/store/postgres -run TestPostgreSQLAdapter_Integration
```

## Requirements

- Go 1.25.x or later
- PostgreSQL 12 or later
- Docker (for integration tests only)

## Dependencies

- `github.com/lib/pq` - PostgreSQL driver
- `github.com/testcontainers/testcontainers-go` - Integration testing (test only)

## Best Practices

1. **Always use context**: Pass context to all query methods for proper timeout and cancellation support
2. **Use transactions**: Wrap related operations in transactions to ensure atomicity
3. **Handle errors**: Always check and handle errors from database operations
4. **Close connections**: Use `defer adapter.Close()` to ensure proper cleanup
5. **Monitor health**: Implement health check endpoints using `adapter.HealthCheck()`
6. **Configure pools**: Tune connection pool settings based on your workload

## Error Handling

The adapter returns descriptive errors for common failure scenarios:

- **Connection failures**: "failed to open database", "failed to ping database"
- **Transaction failures**: "failed to begin transaction", "failed to commit transaction"
- **Health check failures**: "database health check failed"

All errors are wrapped with context using `fmt.Errorf` with `%w` for proper error chain inspection.

## Performance Considerations

- **Connection pooling**: Properly configured pools reduce connection overhead
- **Connection lifetime**: Set `ConnMaxLifetime` to prevent stale connections
- **Query timeouts**: Use `QueryTimeout` to prevent long-running queries
- **Prepared statements**: Consider using prepared statements for frequently executed queries

## See Also

- [Repository Interfaces](../../repository/README.md)
- [Configuration Management](../../config/README.md)
- [Structured Logging](../../observability/logger/README.md)
