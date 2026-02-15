package mongodb

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

func TestNewMongoDBAdapter_Validation(t *testing.T) {
	_, err := NewMongoDBAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty URL and database")
	}

	_, err = NewMongoDBAdapter(Config{URL: "mongodb://localhost:27017"}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty database")
	}
}

func TestPing_WhenClosed(t *testing.T) {
	a := &MongoDBAdapter{closed: true}
	if err := a.Ping(context.Background()); err == nil {
		t.Fatal("expected error when adapter is closed")
	}
}

func TestClose_IdempotentWhenAlreadyClosed(t *testing.T) {
	a := &MongoDBAdapter{closed: true}
	if err := a.Close(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestWithOperationTimeout_UsesAdapterTimeoutWhenNoDeadline(t *testing.T) {
	a := &MongoDBAdapter{timeout: 2 * time.Second}

	ctx, cancel := a.withOperationTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline from operation timeout")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > 2*time.Second {
		t.Fatalf("unexpected remaining timeout: %v", remaining)
	}
}

func TestWithOperationTimeout_PreservesCallerDeadline(t *testing.T) {
	a := &MongoDBAdapter{timeout: 2 * time.Second}
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer parentCancel()

	ctx, cancel := a.withOperationTimeout(parentCtx)
	defer cancel()

	parentDeadline, _ := parentCtx.Deadline()
	gotDeadline, _ := ctx.Deadline()
	if !gotDeadline.Equal(parentDeadline) {
		t.Fatalf("expected caller deadline to be preserved, got %v want %v", gotDeadline, parentDeadline)
	}
}
