package logger

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger is a Logger implementation using uber-go/zap for structured logging.
type ZapLogger struct {
	logger *zap.Logger
	sugar  *zap.SugaredLogger
}

// LogLevel represents the logging level
type LogLevel string

// Log level constants
const (
	// DebugLevel enables debug and above logs
	DebugLevel LogLevel = "debug"
	// InfoLevel enables info and above logs
	InfoLevel LogLevel = "info"
	// WarnLevel enables warning and above logs
	WarnLevel LogLevel = "warn"
	// ErrorLevel enables error logs only
	ErrorLevel LogLevel = "error"
)

// LogFormat represents the output format for logs
type LogFormat string

// Log format constants
const (
	// JSONFormat outputs structured JSON logs
	JSONFormat LogFormat = "json"
	// TextFormat outputs human-readable text logs
	TextFormat LogFormat = "text"
)

// Config holds configuration for the logger
type Config struct {
	Level  LogLevel
	Format LogFormat
}

// DefaultConfig returns the default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	}
}

// NewZapLogger creates a new ZapLogger with the specified configuration.
// It supports both JSON and text (console) output formats and configurable log levels.
func NewZapLogger(cfg Config) (*ZapLogger, error) {
	// Parse log level
	var level zapcore.Level
	switch cfg.Level {
	case DebugLevel:
		level = zapcore.DebugLevel
	case InfoLevel:
		level = zapcore.InfoLevel
	case WarnLevel:
		level = zapcore.WarnLevel
	case ErrorLevel:
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	// Configure encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create encoder based on format
	var encoder zapcore.Encoder
	if cfg.Format == JSONFormat {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &ZapLogger{
		logger: logger,
		sugar:  logger.Sugar(),
	}, nil
}

// Debug logs a debug-level message with optional key-value pairs
func (l *ZapLogger) Debug(msg string, args ...any) {
	l.sugar.Debugw(msg, args...)
}

// Info logs an info-level message with optional key-value pairs
func (l *ZapLogger) Info(msg string, args ...any) {
	l.sugar.Infow(msg, args...)
}

// Warn logs a warning-level message with optional key-value pairs
func (l *ZapLogger) Warn(msg string, args ...any) {
	l.sugar.Warnw(msg, args...)
}

// Error logs an error-level message with optional key-value pairs
func (l *ZapLogger) Error(msg string, args ...any) {
	l.sugar.Errorw(msg, args...)
}

// With creates a child logger with additional key-value pairs that will be
// included in all subsequent log entries
func (l *ZapLogger) With(args ...any) Logger {
	return &ZapLogger{
		logger: l.logger,
		sugar:  l.sugar.With(args...),
	}
}

// WithContext creates a child logger that extracts request ID from context.
// If a request ID is found in the context, it will be included in all log entries.
func (l *ZapLogger) WithContext(ctx context.Context) Logger {
	// Extract request ID from context if present
	if requestID := getRequestIDFromContext(ctx); requestID != "" {
		return l.With("request_id", requestID)
	}
	return l
}

// getRequestIDFromContext extracts the request ID from the context.
// This function looks for the request ID under the "request_id" key.
func getRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}

	return ""
}

// Sync flushes any buffered log entries. Applications should call this before exiting.
func (l *ZapLogger) Sync() error {
	return l.logger.Sync()
}

// ParseLogLevel converts a string to a LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch level {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	default:
		return "", fmt.Errorf("invalid log level: %s", level)
	}
}

// ParseLogFormat converts a string to a LogFormat
func ParseLogFormat(format string) (LogFormat, error) {
	switch format {
	case "json":
		return JSONFormat, nil
	case "text", "console":
		return TextFormat, nil
	default:
		return "", fmt.Errorf("invalid log format: %s", format)
	}
}
