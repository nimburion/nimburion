# Logger Package

The logger package provides a structured logging interface and implementation using [uber-go/zap](https://github.com/uber-go/zap) for high-performance, structured logging throughout the Go microservices framework.

## Features

- **Structured Logging**: All log entries are structured with key-value pairs for easy parsing and querying
- **Multiple Log Levels**: Support for Debug, Info, Warn, and Error levels with configurable filtering
- **Multiple Output Formats**: JSON format for production and text format for development
- **Context-Aware Logging**: Automatic extraction of request IDs from context
- **Child Loggers**: Create child loggers with additional fields that persist across log entries
- **High Performance**: Built on uber-go/zap for minimal performance overhead

## Requirements

This implementation satisfies the following framework requirements:

- **Requirement 12.1**: Uses structured logging (uber-go/zap)
- **Requirement 12.2**: Includes Request_ID in all log entries (via WithContext)
- **Requirement 12.3**: Includes timestamp in all log entries
- **Requirement 12.4**: Supports configurable log levels (debug, info, warn, error)
- **Requirement 12.7**: Supports JSON log output format

## Usage

### Basic Usage

```go
import "github.com/nimburion/nimburion/pkg/observability/logger"

// Create a logger
log, err := logger.NewZapLogger(logger.Config{
    Level:  logger.InfoLevel,
    Format: logger.JSONFormat,
})
if err != nil {
    panic(err)
}
defer log.Sync() // Flush any buffered log entries

// Log messages
log.Info("application started")
log.Debug("debug information")
log.Warn("warning message")
log.Error("error occurred")
```

### Structured Fields

Add structured fields to log entries for better observability:

```go
log.Info("user logged in",
    "user_id", "12345",
    "username", "john.doe",
    "ip_address", "192.168.1.1",
)
```

Output (JSON format):
```json
{
  "level": "info",
  "timestamp": "2026-02-14T14:26:13.482+0100",
  "caller": "main.go:42",
  "message": "user logged in",
  "user_id": "12345",
  "username": "john.doe",
  "ip_address": "192.168.1.1"
}
```

### Child Loggers

Create child loggers with persistent fields:

```go
// Create a child logger with service context
serviceLogger := log.With(
    "service", "user-service",
    "version", "1.0.0",
)

// All logs from serviceLogger will include service and version
serviceLogger.Info("processing request")
serviceLogger.Warn("slow query detected", "duration_ms", 1500)
```

### Context-Aware Logging

Automatically include request IDs from context:

```go
// Create a context with request ID (typically from middleware)
ctx := context.WithValue(context.Background(), "request_id", "req-abc-123")

// Create a logger that includes the request ID
requestLogger := log.WithContext(ctx)

// All logs will automatically include the request_id
requestLogger.Info("handling request")
requestLogger.Info("database query executed", "rows", 42)
```

Output:
```json
{
  "level": "info",
  "timestamp": "2026-02-14T14:26:13.482+0100",
  "message": "handling request",
  "request_id": "req-abc-123"
}
```

### Configuration from Environment

Parse configuration from environment variables:

```go
import "os"

// Get log level from environment
levelStr := os.Getenv("LOG_LEVEL")
if levelStr == "" {
    levelStr = "info"
}
level, err := logger.ParseLogLevel(levelStr)
if err != nil {
    level = logger.InfoLevel
}

// Get log format from environment
formatStr := os.Getenv("LOG_FORMAT")
if formatStr == "" {
    formatStr = "json"
}
format, err := logger.ParseLogFormat(formatStr)
if err != nil {
    format = logger.JSONFormat
}

// Create logger with environment configuration
log, err := logger.NewZapLogger(logger.Config{
    Level:  level,
    Format: format,
})
```

## Configuration

### Log Levels

- `logger.DebugLevel`: Detailed information for debugging
- `logger.InfoLevel`: General informational messages
- `logger.WarnLevel`: Warning messages for potentially harmful situations
- `logger.ErrorLevel`: Error messages for serious problems

### Log Formats

- `logger.JSONFormat`: Structured JSON output (recommended for production)
- `logger.TextFormat`: Human-readable console output (recommended for development)

## Interface

The `Logger` interface defines the contract for all logger implementations:

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    With(args ...any) Logger
    WithContext(ctx context.Context) Logger
}
```

This interface allows for easy mocking in tests and potential alternative implementations.

## Best Practices

1. **Always call Sync()**: Ensure buffered log entries are flushed before application exit
   ```go
   defer log.Sync()
   ```

2. **Use structured fields**: Prefer structured fields over string formatting
   ```go
   // Good
   log.Info("user created", "user_id", userID, "email", email)
   
   // Avoid
   log.Info(fmt.Sprintf("user created: %s (%s)", userID, email))
   ```

3. **Create child loggers for components**: Use `With()` to add component context
   ```go
   dbLogger := log.With("component", "database")
   cacheLogger := log.With("component", "cache")
   ```

4. **Use WithContext in HTTP handlers**: Automatically include request IDs
   ```go
   func handler(w http.ResponseWriter, r *http.Request) {
       logger := log.WithContext(r.Context())
       logger.Info("handling request")
   }
   ```

5. **Choose appropriate log levels**:
   - Debug: Detailed diagnostic information
   - Info: Normal application flow
   - Warn: Unexpected but recoverable situations
   - Error: Errors that need attention

## Performance

The logger is built on uber-go/zap, which is designed for high performance:

- Zero allocation in most cases
- Structured logging with minimal overhead
- Efficient JSON encoding
- Suitable for high-throughput applications

## Testing

The package includes comprehensive tests covering:

- Logger creation with different configurations
- Log level filtering
- Structured field logging
- Child logger creation
- Context-aware logging
- Configuration parsing
- Benchmark tests for performance validation

Run tests:
```bash
go test ./pkg/observability/logger/...
```

Run benchmarks:
```bash
go test -bench=. ./pkg/observability/logger/...
```
