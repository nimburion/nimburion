package logger

import (
	"context"
	"sync"
	"sync/atomic"
)

// AsyncConfig configures the async logger wrapper.
type AsyncConfig struct {
	Enabled      bool
	QueueSize    int
	WorkerCount  int
	DropWhenFull bool
}

type logLevel int

const (
	logLevelDebug logLevel = iota
	logLevelInfo
	logLevelWarn
	logLevelError
)

type asyncEntry struct {
	base  Logger
	level logLevel
	msg   string
	args  []any
}

type asyncDispatcher struct {
	entries      chan asyncEntry
	dropWhenFull bool
	wg           sync.WaitGroup
	stopOnce     sync.Once
	stopped      atomic.Bool
}

// AsyncLogger queues log entries and writes them through worker goroutines.
type AsyncLogger struct {
	base       Logger
	dispatcher *asyncDispatcher
}

// WrapAsync wraps a logger with async dispatch when enabled.
func WrapAsync(base Logger, cfg AsyncConfig) Logger {
	if !cfg.Enabled {
		return base
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 1024
	}
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 1
	}

	dispatcher := &asyncDispatcher{
		entries:      make(chan asyncEntry, queueSize),
		dropWhenFull: cfg.DropWhenFull,
	}
	for i := 0; i < workerCount; i++ {
		dispatcher.wg.Add(1)
		go func() {
			defer dispatcher.wg.Done()
			for entry := range dispatcher.entries {
				switch entry.level {
				case logLevelDebug:
					entry.base.Debug(entry.msg, entry.args...)
				case logLevelInfo:
					entry.base.Info(entry.msg, entry.args...)
				case logLevelWarn:
					entry.base.Warn(entry.msg, entry.args...)
				case logLevelError:
					entry.base.Error(entry.msg, entry.args...)
				}
			}
		}()
	}

	return &AsyncLogger{
		base:       base,
		dispatcher: dispatcher,
	}
}

// Debug logs a debug-level message asynchronously.
func (l *AsyncLogger) Debug(msg string, args ...any) {
	l.enqueue(logLevelDebug, msg, args...)
}

// Info logs an info-level message asynchronously.
func (l *AsyncLogger) Info(msg string, args ...any) {
	l.enqueue(logLevelInfo, msg, args...)
}

// Warn logs a warn-level message asynchronously.
func (l *AsyncLogger) Warn(msg string, args ...any) {
	l.enqueue(logLevelWarn, msg, args...)
}

// Error logs an error-level message asynchronously.
func (l *AsyncLogger) Error(msg string, args ...any) {
	l.enqueue(logLevelError, msg, args...)
}

// With returns a new logger with additional fields.
func (l *AsyncLogger) With(args ...any) Logger {
	return &AsyncLogger{
		base:       l.base.With(args...),
		dispatcher: l.dispatcher,
	}
}

// WithContext returns a new logger with the given context.
func (l *AsyncLogger) WithContext(ctx context.Context) Logger {
	return &AsyncLogger{
		base:       l.base.WithContext(ctx),
		dispatcher: l.dispatcher,
	}
}

// Close drains the queue and stops async workers.
func (l *AsyncLogger) Close() {
	l.dispatcher.stop()
}

func (l *AsyncLogger) enqueue(level logLevel, msg string, args ...any) {
	if l.dispatcher.stopped.Load() {
		l.logNow(level, msg, args...)
		return
	}

	entry := asyncEntry{
		base:  l.base,
		level: level,
		msg:   msg,
		args:  args,
	}

	if l.dispatcher.dropWhenFull {
		select {
		case l.dispatcher.entries <- entry:
		default:
		}
		return
	}

	l.dispatcher.entries <- entry
}

func (l *AsyncLogger) logNow(level logLevel, msg string, args ...any) {
	switch level {
	case logLevelDebug:
		l.base.Debug(msg, args...)
	case logLevelInfo:
		l.base.Info(msg, args...)
	case logLevelWarn:
		l.base.Warn(msg, args...)
	case logLevelError:
		l.base.Error(msg, args...)
	}
}

func (d *asyncDispatcher) stop() {
	d.stopOnce.Do(func() {
		d.stopped.Store(true)
		close(d.entries)
		d.wg.Wait()
	})
}
