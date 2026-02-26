package testutil

import (
	"context"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// MockLogger is a test logger that captures log entries for assertion in tests.
type MockLogger struct {
	Logs []LogEntry
}

// LogEntry represents a single log entry captured by MockLogger.
type LogEntry struct {
	Level  string
	Msg    string
	Fields map[string]interface{}
}

// Debug TODO: add description
func (m *MockLogger) Debug(msg string, args ...any) {
	m.Logs = append(m.Logs, LogEntry{Level: "debug", Msg: msg, Fields: argsToMap(args)})
}

// Info TODO: add description
func (m *MockLogger) Info(msg string, args ...any) {
	m.Logs = append(m.Logs, LogEntry{Level: "info", Msg: msg, Fields: argsToMap(args)})
}

// Warn TODO: add description
func (m *MockLogger) Warn(msg string, args ...any) {
	m.Logs = append(m.Logs, LogEntry{Level: "warn", Msg: msg, Fields: argsToMap(args)})
}

// Error TODO: add description
func (m *MockLogger) Error(msg string, args ...any) {
	m.Logs = append(m.Logs, LogEntry{Level: "error", Msg: msg, Fields: argsToMap(args)})
}

// With TODO: add description
func (m *MockLogger) With(args ...any) logger.Logger {
	return m
}

// WithContext TODO: add description
func (m *MockLogger) WithContext(ctx context.Context) logger.Logger {
	return m
}

func argsToMap(args []any) map[string]interface{} {
	fields := make(map[string]interface{})
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			fields[key] = args[i+1]
		}
	}
	return fields
}
