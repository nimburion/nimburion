package logger

import (
	"context"
)

// Logger defines the interface for structured logging throughout the framework.
// All log methods accept a message string followed by key-value pairs for structured fields.
type Logger interface {
	// Debug logs a debug-level message with optional key-value pairs
	Debug(msg string, args ...any)
	
	// Info logs an info-level message with optional key-value pairs
	Info(msg string, args ...any)
	
	// Warn logs a warning-level message with optional key-value pairs
	Warn(msg string, args ...any)
	
	// Error logs an error-level message with optional key-value pairs
	Error(msg string, args ...any)
	
	// With creates a child logger with additional key-value pairs that will be
	// included in all subsequent log entries
	With(args ...any) Logger
	
	// WithContext creates a child logger that extracts request ID from context
	WithContext(ctx context.Context) Logger
}
