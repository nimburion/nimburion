package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

func TestNewAdapter_Validation(t *testing.T) {
	_, err := NewAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty region")
	}
}

func TestPing_WhenClosed(t *testing.T) {
	a := &Adapter{closed: true, logger: &mockLogger{}}
	if err := a.Ping(context.Background()); err == nil {
		t.Fatal("expected error when closed")
	}
}

func TestOperations_WhenClosed(t *testing.T) {
	a := &Adapter{closed: true, logger: &mockLogger{}}

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "put item",
			run: func() error {
				_, err := a.PutItem(context.Background(), &dynamodb.PutItemInput{})
				return err
			},
		},
		{
			name: "get item",
			run: func() error {
				_, err := a.GetItem(context.Background(), &dynamodb.GetItemInput{})
				return err
			},
		},
		{
			name: "update item",
			run: func() error {
				_, err := a.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{})
				return err
			},
		},
		{
			name: "delete item",
			run: func() error {
				_, err := a.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{})
				return err
			},
		},
		{
			name: "query",
			run: func() error {
				_, err := a.Query(context.Background(), &dynamodb.QueryInput{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("expected error when closed")
			}
		})
	}
}

func TestClose_Idempotent(t *testing.T) {
	a := &Adapter{}
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
	a := &Adapter{timeout: 2 * time.Second}

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
	a := &Adapter{timeout: 2 * time.Second}
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
