package mysql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

func TestNewMySQLAdapter_Validation(t *testing.T) {
	_, err := NewMySQLAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestGetTx(t *testing.T) {
	ctx := context.Background()
	if tx, ok := GetTx(ctx); ok || tx != nil {
		t.Fatal("expected no tx in plain context")
	}

	// sqlmock requires explicit expectation for transaction begin.
	expectDB, expect, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer expectDB.Close()
	expect.ExpectBegin()
	tx, err := expectDB.Begin()
	if err != nil {
		t.Fatalf("begin tx error: %v", err)
	}
	ctx = context.WithValue(ctx, txContextKey, tx)
	got, ok := GetTx(ctx)
	if !ok || got == nil {
		t.Fatal("expected tx from context")
	}
	_ = tx.Rollback()
	if err := expect.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations: %v", err)
	}
}

func TestClosePreventsSubsequentOperations(t *testing.T) {
	db, expectations, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()
	expectations.ExpectClose()

	a := &MySQLAdapter{db: db, logger: &mockLogger{}}
	if err := a.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	_, err = a.ExecContext(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("expected error after close")
	}
	if err := expectations.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations: %v", err)
	}
}

func TestWithTransaction_RollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	a := &MySQLAdapter{db: db, logger: &mockLogger{}}

	mock.ExpectBegin()
	mock.ExpectRollback()

	txErr := errors.New("tx failed")
	err = a.WithTransaction(context.Background(), func(ctx context.Context) error {
		if _, ok := GetTx(ctx); !ok {
			t.Fatal("expected tx in context")
		}
		return txErr
	})
	if !errors.Is(err, txErr) {
		t.Fatalf("expected tx error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations: %v", err)
	}
}

func TestWithTransaction_CommitOnSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	a := &MySQLAdapter{db: db, logger: &mockLogger{}}

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = a.WithTransaction(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations: %v", err)
	}
}

func TestQueryRowContext_UsesDBWhenNoTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	defer db.Close()

	a := &MySQLAdapter{db: db, logger: &mockLogger{}, config: Config{QueryTimeout: 2 * time.Second}}
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))

	var v int
	if err := a.QueryRowContext(context.Background(), "SELECT 1").Scan(&v); err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected 1, got %d", v)
	}
}

func TestWithQueryTimeout_UsesConfigWhenNoDeadline(t *testing.T) {
	a := &MySQLAdapter{config: Config{QueryTimeout: 2 * time.Second}}

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
	a := &MySQLAdapter{config: Config{QueryTimeout: 2 * time.Second}}
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

var _ *sql.DB
