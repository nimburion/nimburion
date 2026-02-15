package logger_test

import (
	"context"
	"fmt"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

func ExampleNewZapLogger() {
	// Create a logger with JSON format and info level
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	// Log a simple message
	log.Info("application started")

	// Log with structured fields
	log.Info("user logged in",
		"user_id", "12345",
		"username", "john.doe",
		"ip_address", "192.168.1.1",
	)
}

func ExampleZapLogger_With() {
	log, _ := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	defer log.Sync()

	// Create a child logger with service context
	serviceLogger := log.With(
		"service", "user-service",
		"version", "1.0.0",
	)

	// All logs from serviceLogger will include service and version
	serviceLogger.Info("processing request")
	serviceLogger.Warn("slow query detected", "duration_ms", 1500)
}

func ExampleZapLogger_WithContext() {
	log, _ := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	defer log.Sync()

	// Create a context with request ID (typically from middleware)
	ctx := context.WithValue(context.Background(), "request_id", "req-abc-123")

	// Create a logger that includes the request ID
	requestLogger := log.WithContext(ctx)

	// All logs will automatically include the request_id
	requestLogger.Info("handling request")
	requestLogger.Info("database query executed", "rows", 42)
	requestLogger.Info("request completed", "status", 200)
}

func ExampleParseLogLevel() {
	// Parse log level from string (e.g., from environment variable)
	level, err := logger.ParseLogLevel("debug")
	if err != nil {
		fmt.Printf("Invalid log level: %v\n", err)
		return
	}

	log, _ := logger.NewZapLogger(logger.Config{
		Level:  level,
		Format: logger.JSONFormat,
	})
	defer log.Sync()

	log.Debug("this debug message will be visible")
}

func ExampleParseLogFormat() {
	// Parse log format from string (e.g., from environment variable)
	format, err := logger.ParseLogFormat("json")
	if err != nil {
		fmt.Printf("Invalid log format: %v\n", err)
		return
	}

	log, _ := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: format,
	})
	defer log.Sync()

	log.Info("structured JSON output")
}
