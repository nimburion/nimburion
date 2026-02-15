package logger

import (
	"context"
	"sync"
	"testing"
	"time"
)

type testLogger struct {
	mu    sync.Mutex
	logs  []string
	withs [][]any
}

func (l *testLogger) Debug(msg string, args ...any) { l.append(msg) }
func (l *testLogger) Info(msg string, args ...any)  { l.append(msg) }
func (l *testLogger) Warn(msg string, args ...any)  { l.append(msg) }
func (l *testLogger) Error(msg string, args ...any) { l.append(msg) }

func (l *testLogger) With(args ...any) Logger {
	l.mu.Lock()
	l.withs = append(l.withs, args)
	l.mu.Unlock()
	return l
}

func (l *testLogger) WithContext(context.Context) Logger { return l }

func (l *testLogger) append(msg string) {
	l.mu.Lock()
	l.logs = append(l.logs, msg)
	l.mu.Unlock()
}

func TestWrapAsync_DisabledReturnsBase(t *testing.T) {
	base := &testLogger{}
	wrapped := WrapAsync(base, AsyncConfig{Enabled: false})
	if wrapped != base {
		t.Fatalf("expected base logger when disabled")
	}
}

func TestWrapAsync_EmitsLogs(t *testing.T) {
	base := &testLogger{}
	wrapped := WrapAsync(base, AsyncConfig{
		Enabled:      true,
		QueueSize:    16,
		WorkerCount:  1,
		DropWhenFull: false,
	})

	async, ok := wrapped.(*AsyncLogger)
	if !ok {
		t.Fatalf("expected async logger type")
	}

	wrapped.Info("first")
	wrapped.Error("second")
	async.Close()

	base.mu.Lock()
	defer base.mu.Unlock()
	if len(base.logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(base.logs))
	}
}

func TestWrapAsync_DropWhenFull(t *testing.T) {
	base := &testLogger{}
	wrapped := WrapAsync(base, AsyncConfig{
		Enabled:      true,
		QueueSize:    1,
		WorkerCount:  1,
		DropWhenFull: true,
	})

	for i := 0; i < 200; i++ {
		wrapped.Info("line")
	}

	time.Sleep(50 * time.Millisecond)
	wrapped.(*AsyncLogger).Close()

	base.mu.Lock()
	defer base.mu.Unlock()
	if len(base.logs) == 0 {
		t.Fatalf("expected at least one log to be processed")
	}
	if len(base.logs) >= 200 {
		t.Fatalf("expected some logs to be dropped when queue is full")
	}
}
