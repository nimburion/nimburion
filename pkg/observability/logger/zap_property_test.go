package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// **Validates: Requirements 12.1, 12.2, 12.3, 12.7**
// Property 11: Structured Logging Format
// For any log entry, the output should be valid JSON containing at minimum:
// timestamp, level, message, and request_id (when in request context).
func TestProperty_StructuredLoggingFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for log levels
	genLogLevel := gen.OneConstOf(DebugLevel, InfoLevel, WarnLevel, ErrorLevel)
	
	// Generator for log messages
	genMessage := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) < 200
	})
	
	// Generator for request IDs (optional)
	genRequestID := gen.OneGenOf(
		gen.Const(""), // No request ID
		gen.Identifier().Map(func(s string) string {
			return "req-" + s
		}),
	)
	
	// Generator for additional fields (simplified - just generate a count and keys/values separately)
	genFieldCount := gen.IntRange(0, 5)

	properties.Property("all log entries are valid JSON with required fields", prop.ForAll(
		func(level LogLevel, message string, requestID string, fieldCount int) bool {
			// Capture log output
			var buf bytes.Buffer
			logger := createTestLogger(&buf, level)
			
			// Create context with or without request ID
			var ctx context.Context
			if requestID != "" {
				ctx = context.WithValue(context.Background(), "request_id", requestID)
			} else {
				ctx = context.Background()
			}
			
			// Create logger with context
			ctxLogger := logger.WithContext(ctx)
			
			// Generate simple fields
			var args []interface{}
			for i := 0; i < fieldCount; i++ {
				args = append(args, "field"+string(rune('A'+i)), "value"+string(rune('A'+i)))
			}
			
			// Log at the appropriate level
			switch level {
			case DebugLevel:
				ctxLogger.Debug(message, args...)
			case InfoLevel:
				ctxLogger.Info(message, args...)
			case WarnLevel:
				ctxLogger.Warn(message, args...)
			case ErrorLevel:
				ctxLogger.Error(message, args...)
			}
			
			// Sync to ensure output is written
			if zl, ok := logger.(*ZapLogger); ok {
				_ = zl.Sync()
			}
			
			// Parse the JSON output
			output := buf.String()
			if output == "" {
				// No output means the log level filtered it out
				return true
			}
			
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
				t.Logf("Failed to parse JSON: %v\nOutput: %s", err, output)
				return false
			}
			
			// Verify required fields exist
			requiredFields := []string{"timestamp", "level", "message"}
			for _, field := range requiredFields {
				if _, ok := logEntry[field]; !ok {
					t.Logf("Missing required field: %s\nLog entry: %v", field, logEntry)
					return false
				}
			}
			
			// Verify message matches
			if logEntry["message"] != message {
				t.Logf("Message mismatch: expected %q, got %q", message, logEntry["message"])
				return false
			}
			
			// Verify level matches
			expectedLevel := string(level)
			if logEntry["level"] != expectedLevel {
				t.Logf("Level mismatch: expected %q, got %q", expectedLevel, logEntry["level"])
				return false
			}
			
			// Verify timestamp is valid ISO8601 format
			if timestamp, ok := logEntry["timestamp"].(string); ok {
				// Try multiple timestamp formats (ISO8601 can have different formats)
				formats := []string{
					time.RFC3339,
					time.RFC3339Nano,
					"2006-01-02T15:04:05.000-0700",
					"2006-01-02T15:04:05.000Z0700",
				}
				parsed := false
				for _, format := range formats {
					if _, err := time.Parse(format, timestamp); err == nil {
						parsed = true
						break
					}
				}
				if !parsed {
					t.Logf("Invalid timestamp format: %s", timestamp)
					return false
				}
			} else {
				t.Logf("Timestamp is not a string: %v", logEntry["timestamp"])
				return false
			}
			
			// Verify request_id is present when provided in context
			if requestID != "" {
				if rid, ok := logEntry["request_id"]; !ok {
					t.Logf("Missing request_id in log entry when provided in context")
					return false
				} else if rid != requestID {
					t.Logf("Request ID mismatch: expected %q, got %q", requestID, rid)
					return false
				}
			}
			
			return true
		},
		genLogLevel,
		genMessage,
		genRequestID,
		genFieldCount,
	))

	properties.TestingRun(t)
}

// createTestLogger creates a logger that writes to the provided buffer
func createTestLogger(w io.Writer, level LogLevel) Logger {
	// Parse log level
	var zapLevel zapcore.Level
	switch level {
	case DebugLevel:
		zapLevel = zapcore.DebugLevel
	case InfoLevel:
		zapLevel = zapcore.InfoLevel
	case WarnLevel:
		zapLevel = zapcore.WarnLevel
	case ErrorLevel:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Configure encoder for JSON output
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

	encoder := zapcore.NewJSONEncoder(encoderConfig)
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(w),
		zapLevel,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &ZapLogger{
		logger: logger,
		sugar:  logger.Sugar(),
	}
}

// shouldLogAppear determines if a log at logLevel should appear when logger is configured at configLevel
func TestProperty_JSONOutputAlwaysParseable(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	genMessage := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) < 200
	})

	properties.Property("JSON output is always parseable", prop.ForAll(
		func(message string) bool {
			var buf bytes.Buffer
			logger := createTestLogger(&buf, InfoLevel)
			
			logger.Info(message)
			
			if zl, ok := logger.(*ZapLogger); ok {
				_ = zl.Sync()
			}
			
			output := buf.String()
			if output == "" {
				return true
			}
			
			var logEntry map[string]interface{}
			return json.Unmarshal([]byte(output), &logEntry) == nil
		},
		genMessage,
	))

	properties.TestingRun(t)
}

// Property test: Verify log level filtering works correctly
// **Validates: Requirements 12.4**
func TestProperty_LogLevelFiltering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	genConfigLevel := gen.OneConstOf(DebugLevel, InfoLevel, WarnLevel, ErrorLevel)
	genLogLevel := gen.OneConstOf(DebugLevel, InfoLevel, WarnLevel, ErrorLevel)
	genMessage := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) < 100
	})

	properties.Property("log level filtering works correctly", prop.ForAll(
		func(configLevel LogLevel, logLevel LogLevel, message string) bool {
			var buf bytes.Buffer
			logger := createTestLogger(&buf, configLevel)
			
			// Log at the specified level
			switch logLevel {
			case DebugLevel:
				logger.Debug(message)
			case InfoLevel:
				logger.Info(message)
			case WarnLevel:
				logger.Warn(message)
			case ErrorLevel:
				logger.Error(message)
			}
			
			if zl, ok := logger.(*ZapLogger); ok {
				_ = zl.Sync()
			}
			
			output := buf.String()
			
			// Determine if log should appear based on level hierarchy
			shouldAppear := shouldLogAppear(configLevel, logLevel)
			
			hasOutput := output != ""
			
			if shouldAppear != hasOutput {
				t.Logf("Level filtering failed: config=%s, log=%s, shouldAppear=%v, hasOutput=%v",
					configLevel, logLevel, shouldAppear, hasOutput)
				return false
			}
			
			return true
		},
		genConfigLevel,
		genLogLevel,
		genMessage,
	))

	properties.TestingRun(t)
}

// shouldLogAppear determines if a log at logLevel should appear when logger is configured at configLevel
func shouldLogAppear(configLevel, logLevel LogLevel) bool {
	levels := map[LogLevel]int{
		DebugLevel: 0,
		InfoLevel:  1,
		WarnLevel:  2,
		ErrorLevel: 3,
	}
	
	return levels[logLevel] >= levels[configLevel]
}

// Benchmark property-based test execution
func BenchmarkProperty_StructuredLoggingFormat(b *testing.B) {
	// Redirect stdout to discard during benchmark
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	for i := 0; i < b.N; i++ {
		t := &testing.T{}
		TestProperty_StructuredLoggingFormat(t)
	}
}
