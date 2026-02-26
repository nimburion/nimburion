package migrate

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestRunWithDBContextNilBuilder(t *testing.T) {
	err := RunWithDBContext(context.Background(), &sql.DB{}, "migrations", "test", "up", 1, 10*time.Second, testLogger{}, nil)
	if err == nil {
		t.Fatal("expected error for nil builder")
	}
}

func TestRunWithDBContextNilDB(t *testing.T) {
	builder := func(context.Context, *sql.DB, string) (Operations, error) {
		return defaultOperations(), nil
	}
	err := RunWithDBContext(context.Background(), nil, "migrations", "test", "up", 1, 10*time.Second, testLogger{}, builder)
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestRunWithSQLDriverNilBuilder(t *testing.T) {
	err := RunWithSQLDriver("sqlite3", ":memory:", "migrations", "test", "up", 1, 10*time.Second, testLogger{}, nil)
	if err == nil {
		t.Fatal("expected error for nil builder")
	}
}

func TestRunWithSQLDriverEmptyDriver(t *testing.T) {
	builder := func(*sql.DB, string) (Operations, error) {
		return defaultOperations(), nil
	}
	err := RunWithSQLDriver("", ":memory:", "migrations", "test", "up", 1, 10*time.Second, testLogger{}, builder)
	if err == nil {
		t.Fatal("expected error for empty driver")
	}
}

func TestRunWithSQLDriverContextNilBuilder(t *testing.T) {
	err := RunWithSQLDriverContext(context.Background(), "sqlite3", ":memory:", "migrations", "test", "up", 1, 10*time.Second, testLogger{}, nil)
	if err == nil {
		t.Fatal("expected error for nil builder")
	}
}

func TestRunWithSQLDriverContextEmptyDriver(t *testing.T) {
	builder := func(context.Context, *sql.DB, string) (Operations, error) {
		return defaultOperations(), nil
	}
	err := RunWithSQLDriverContext(context.Background(), "", ":memory:", "migrations", "test", "up", 1, 10*time.Second, testLogger{}, builder)
	if err == nil {
		t.Fatal("expected error for empty driver")
	}
}

func TestRunWithSQLDriverInvalidDriver(t *testing.T) {
	builder := func(*sql.DB, string) (Operations, error) {
		return defaultOperations(), nil
	}
	err := RunWithSQLDriver("invalid_driver", "invalid_dsn", "migrations", "test", "up", 1, 10*time.Second, testLogger{}, builder)
	if err == nil {
		t.Fatal("expected error for invalid driver")
	}
}
