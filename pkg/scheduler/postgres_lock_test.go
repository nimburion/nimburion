package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPostgresLockProvider_Acquire(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	provider, err := newPostgresLockProviderWithDB(db, PostgresLockProviderConfig{
		Table:            "nimburion_scheduler_locks",
		OperationTimeout: time.Second,
	}, &schedulerTestLogger{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM upsert\\)").
		WithArgs("task-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	lease, acquired, err := provider.Acquire(context.Background(), "task-1", time.Second)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock acquired")
	}
	if lease == nil || strings.TrimSpace(lease.Token) == "" {
		t.Fatal("expected non-empty lease token")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestPostgresLockProvider_RenewAndRelease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	provider, err := newPostgresLockProviderWithDB(db, PostgresLockProviderConfig{
		Table:            "nimburion_scheduler_locks",
		OperationTimeout: time.Second,
	}, &schedulerTestLogger{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	lease := &LockLease{Key: "task-1", Token: "token-1"}

	mock.ExpectExec("UPDATE nimburion_scheduler_locks SET expires_at=\\$3, updated_at=NOW\\(\\) WHERE lock_key=\\$1 AND token=\\$2 AND expires_at > NOW\\(\\)").
		WithArgs("task-1", "token-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := provider.Renew(context.Background(), lease, time.Second); err != nil {
		t.Fatalf("renew: %v", err)
	}

	mock.ExpectExec("DELETE FROM nimburion_scheduler_locks WHERE lock_key=\\$1 AND token=\\$2").
		WithArgs("task-1", "token-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := provider.Release(context.Background(), lease); err != nil {
		t.Fatalf("release: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestPostgresLockProvider_RejectsInvalidTableName(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	_, err = newPostgresLockProviderWithDB(db, PostgresLockProviderConfig{
		Table: "invalid-table-name",
	}, &schedulerTestLogger{})
	if err == nil {
		t.Fatal("expected invalid table name error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestPostgresLockProvider_RenewRejectsMissingLeaseWithTypedConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	provider, err := newPostgresLockProviderWithDB(db, PostgresLockProviderConfig{
		Table:            "nimburion_scheduler_locks",
		OperationTimeout: time.Second,
	}, &schedulerTestLogger{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	lease := &LockLease{Key: "task-1", Token: "token-1"}
	mock.ExpectExec("UPDATE nimburion_scheduler_locks SET expires_at=\\$3, updated_at=NOW\\(\\) WHERE lock_key=\\$1 AND token=\\$2 AND expires_at > NOW\\(\\)").
		WithArgs("task-1", "token-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = provider.Renew(context.Background(), lease, time.Second)
	if err == nil {
		t.Fatal("expected renew rejection error")
	}
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestPostgresLockProvider_ReleaseRejectsMissingLeaseWithTypedConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	provider, err := newPostgresLockProviderWithDB(db, PostgresLockProviderConfig{
		Table:            "nimburion_scheduler_locks",
		OperationTimeout: time.Second,
	}, &schedulerTestLogger{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	lease := &LockLease{Key: "task-1", Token: "token-1"}
	mock.ExpectExec("DELETE FROM nimburion_scheduler_locks WHERE lock_key=\\$1 AND token=\\$2").
		WithArgs("task-1", "token-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = provider.Release(context.Background(), lease)
	if err == nil {
		t.Fatal("expected release rejection error")
	}
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}
