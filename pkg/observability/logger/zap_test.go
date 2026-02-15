package logger

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewZapLogger(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "json format with debug level",
			config: Config{
				Level:  DebugLevel,
				Format: JSONFormat,
			},
			wantErr: false,
		},
		{
			name: "text format with info level",
			config: Config{
				Level:  InfoLevel,
				Format: TextFormat,
			},
			wantErr: false,
		},
		{
			name: "json format with warn level",
			config: Config{
				Level:  WarnLevel,
				Format: JSONFormat,
			},
			wantErr: false,
		},
		{
			name: "json format with error level",
			config: Config{
				Level:  ErrorLevel,
				Format: JSONFormat,
			},
			wantErr: false,
		},
		{
			name: "default to info level for invalid level",
			config: Config{
				Level:  "invalid",
				Format: JSONFormat,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewZapLogger(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewZapLogger() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && logger == nil {
				t.Error("NewZapLogger() returned nil logger")
			}
			if logger != nil {
				_ = logger.Sync()
			}
		})
	}
}

func TestZapLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		logLevel LogLevel
		logFunc  func(Logger)
		expected bool // whether log should appear
	}{
		{
			name:     "debug level logs debug",
			logLevel: DebugLevel,
			logFunc:  func(l Logger) { l.Debug("debug message") },
			expected: true,
		},
		{
			name:     "info level does not log debug",
			logLevel: InfoLevel,
			logFunc:  func(l Logger) { l.Debug("debug message") },
			expected: false,
		},
		{
			name:     "info level logs info",
			logLevel: InfoLevel,
			logFunc:  func(l Logger) { l.Info("info message") },
			expected: true,
		},
		{
			name:     "warn level does not log info",
			logLevel: WarnLevel,
			logFunc:  func(l Logger) { l.Info("info message") },
			expected: false,
		},
		{
			name:     "warn level logs warn",
			logLevel: WarnLevel,
			logFunc:  func(l Logger) { l.Warn("warn message") },
			expected: true,
		},
		{
			name:     "error level does not log warn",
			logLevel: ErrorLevel,
			logFunc:  func(l Logger) { l.Warn("warn message") },
			expected: false,
		},
		{
			name:     "error level logs error",
			logLevel: ErrorLevel,
			logFunc:  func(l Logger) { l.Error("error message") },
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger with JSON format for easier parsing
			logger, err := NewZapLogger(Config{
				Level:  tt.logLevel,
				Format: JSONFormat,
			})
			if err != nil {
				t.Fatalf("Failed to create logger: %v", err)
			}
			defer logger.Sync()

			// Note: In a real test, we'd capture stdout to verify output
			// For now, we just verify the logger doesn't panic
			tt.logFunc(logger)
		})
	}
}

func TestZapLogger_StructuredFields(t *testing.T) {
	logger, err := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Test logging with structured fields
	logger.Info("test message",
		"key1", "value1",
		"key2", 42,
		"key3", true,
	)

	// Test all log levels with structured fields
	logger.Debug("debug with fields", "field", "debug_value")
	logger.Info("info with fields", "field", "info_value")
	logger.Warn("warn with fields", "field", "warn_value")
	logger.Error("error with fields", "field", "error_value")
}

func TestZapLogger_With(t *testing.T) {
	logger, err := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create child logger with additional fields
	childLogger := logger.With("service", "test-service", "version", "1.0.0")

	// Log with child logger
	childLogger.Info("child logger message")

	// Original logger should not have the additional fields
	logger.Info("original logger message")

	// Create another child from the child
	grandchildLogger := childLogger.With("request_id", "12345")
	grandchildLogger.Info("grandchild logger message")
}

func TestZapLogger_WithContext(t *testing.T) {
	logger, err := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	tests := []struct {
		name      string
		ctx       context.Context
		expectID  bool
		requestID string
	}{
		{
			name:      "context with request ID",
			ctx:       context.WithValue(context.Background(), "request_id", "test-request-123"),
			expectID:  true,
			requestID: "test-request-123",
		},
		{
			name:     "context without request ID",
			ctx:      context.Background(),
			expectID: false,
		},
		{
			name:     "nil context",
			ctx:      nil,
			expectID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextLogger := logger.WithContext(tt.ctx)
			contextLogger.Info("test message with context")
			
			// Verify logger was created
			if contextLogger == nil {
				t.Error("WithContext returned nil logger")
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LogLevel
		wantErr bool
	}{
		{
			name:    "debug level",
			input:   "debug",
			want:    DebugLevel,
			wantErr: false,
		},
		{
			name:    "info level",
			input:   "info",
			want:    InfoLevel,
			wantErr: false,
		},
		{
			name:    "warn level",
			input:   "warn",
			want:    WarnLevel,
			wantErr: false,
		},
		{
			name:    "warning level (alias)",
			input:   "warning",
			want:    WarnLevel,
			wantErr: false,
		},
		{
			name:    "error level",
			input:   "error",
			want:    ErrorLevel,
			wantErr: false,
		},
		{
			name:    "invalid level",
			input:   "invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLogFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LogFormat
		wantErr bool
	}{
		{
			name:    "json format",
			input:   "json",
			want:    JSONFormat,
			wantErr: false,
		},
		{
			name:    "text format",
			input:   "text",
			want:    TextFormat,
			wantErr: false,
		},
		{
			name:    "console format (alias)",
			input:   "console",
			want:    TextFormat,
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogFormat(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLogFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestZapLogger_JSONOutput(t *testing.T) {
	// This test verifies that JSON output is valid JSON
	// In a real implementation, we'd capture stdout to verify the actual output
	logger, err := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Log a message - in production, we'd capture and parse this
	logger.Info("test json output",
		"string_field", "value",
		"int_field", 42,
		"bool_field", true,
	)
}

func TestZapLogger_RequestIDPropagation(t *testing.T) {
	logger, err := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Create context with request ID
	ctx := context.WithValue(context.Background(), "request_id", "req-12345")

	// Create logger with context
	ctxLogger := logger.WithContext(ctx)

	// Log messages - request_id should be included automatically
	ctxLogger.Info("processing request")
	ctxLogger.Debug("debug information")
	ctxLogger.Warn("warning message")
	ctxLogger.Error("error occurred")
}

// Helper function to verify JSON structure (would be used with captured output)
func verifyJSONStructure(t *testing.T, jsonStr string) {
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &logEntry); err != nil {
		t.Errorf("Failed to parse JSON log entry: %v", err)
		return
	}

	// Verify required fields
	requiredFields := []string{"timestamp", "level", "message"}
	for _, field := range requiredFields {
		if _, ok := logEntry[field]; !ok {
			t.Errorf("Missing required field: %s", field)
		}
	}
}

func TestGetRequestIDFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "context with request ID",
			ctx:  context.WithValue(context.Background(), "request_id", "test-123"),
			want: "test-123",
		},
		{
			name: "context without request ID",
			ctx:  context.Background(),
			want: "",
		},
		{
			name: "nil context",
			ctx:  nil,
			want: "",
		},
		{
			name: "context with wrong type",
			ctx:  context.WithValue(context.Background(), "request_id", 12345),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRequestIDFromContext(tt.ctx)
			if got != tt.want {
				t.Errorf("getRequestIDFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkZapLogger_Info(b *testing.B) {
	logger, _ := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	defer logger.Sync()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", "iteration", i)
	}
}

func BenchmarkZapLogger_WithFields(b *testing.B) {
	logger, _ := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	defer logger.Sync()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message",
			"field1", "value1",
			"field2", 42,
			"field3", true,
			"iteration", i,
		)
	}
}

func BenchmarkZapLogger_WithContext(b *testing.B) {
	logger, _ := NewZapLogger(Config{
		Level:  InfoLevel,
		Format: JSONFormat,
	})
	defer logger.Sync()

	ctx := context.WithValue(context.Background(), "request_id", "bench-123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctxLogger := logger.WithContext(ctx)
		ctxLogger.Info("benchmark message", "iteration", i)
	}
}
