package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

func TestNewDynamoDBAdapter_Validation(t *testing.T) {
	_, err := NewDynamoDBAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty region")
	}
}

func TestPing_WhenClosed(t *testing.T) {
	a := &DynamoDBAdapter{closed: true, logger: &mockLogger{}}
	if err := a.Ping(context.Background()); err == nil {
		t.Fatal("expected error when closed")
	}
}

func TestClose_Idempotent(t *testing.T) {
	a := &DynamoDBAdapter{}
	if err := a.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("unexpected error on second close: %v", err)
	}
}

func TestIsThrottlingError(t *testing.T) {
	if IsThrottlingError(nil) {
		t.Fatal("nil error must return false")
	}
	if IsThrottlingError(errors.New("x")) {
		t.Fatal("generic error must return false")
	}
	if !IsThrottlingError(&types.ProvisionedThroughputExceededException{}) {
		t.Fatal("expected throttling error to be detected")
	}
}

func TestWithOperationTimeout_UsesAdapterTimeoutWhenNoDeadline(t *testing.T) {
	a := &DynamoDBAdapter{timeout: 2 * time.Second}

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
	a := &DynamoDBAdapter{timeout: 2 * time.Second}
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
