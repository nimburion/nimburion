package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "empty URL",
			cfg: Config{
				URL:             "",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
				QueryTimeout:    10 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, _ := logger.NewZapLogger(logger.Config{
				Level:  logger.InfoLevel,
				Format: logger.JSONFormat,
			})

			_, err := NewPostgreSQLAdapter(tt.cfg, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPostgreSQLAdapter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTx(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() interface{}
		wantOk bool
	}{
		{
			name: "context without transaction",
			setup: func() interface{} {
				return nil
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a minimal test - full transaction tests are in integration tests
			// since they require a real database
		})
	}
}

func TestWithQueryTimeout_UsesConfigWhenNoDeadline(t *testing.T) {
	a := &PostgreSQLAdapter{config: Config{QueryTimeout: 2 * time.Second}}

	ctx, cancel := a.withQueryTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline from query timeout")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > 2*time.Second {
		t.Fatalf("unexpected remaining timeout: %v", remaining)
	}
}

func TestWithQueryTimeout_PreservesCallerDeadline(t *testing.T) {
	a := &PostgreSQLAdapter{config: Config{QueryTimeout: 2 * time.Second}}
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer parentCancel()

	ctx, cancel := a.withQueryTimeout(parentCtx)
	defer cancel()

	parentDeadline, _ := parentCtx.Deadline()
	gotDeadline, _ := ctx.Deadline()
	if !gotDeadline.Equal(parentDeadline) {
		t.Fatalf("expected caller deadline to be preserved, got %v want %v", gotDeadline, parentDeadline)
	}
}
