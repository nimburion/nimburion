package migrate

import (
	"testing"
	"testing/fstest"
)

func TestNewSQLManagerNilDB(t *testing.T) {
	fs := fstest.MapFS{}
	_, err := NewSQLManager(nil, fs, "migrations")
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestNewSQLManagerNilFS(t *testing.T) {
	_, err := NewSQLManager(nil, nil, "migrations")
	if err == nil {
		t.Fatal("expected error for nil fs")
	}
}

func TestNewSQLManagerEmptyDir(t *testing.T) {
	fs := fstest.MapFS{}
	_, err := NewSQLManager(nil, fs, "")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestLoadMigrationsMissingUp(t *testing.T) {
	fs := fstest.MapFS{
		"migrations/001_init.down.sql": {Data: []byte("DROP TABLE users")},
	}
	_, err := loadMigrations(fs, "migrations")
	if err == nil {
		t.Fatal("expected error for missing up migration")
	}
}

func TestLoadMigrationsSuccess(t *testing.T) {
	fs := fstest.MapFS{
		"migrations/001_init.up.sql":   {Data: []byte("CREATE TABLE users")},
		"migrations/001_init.down.sql": {Data: []byte("DROP TABLE users")},
		"migrations/002_add.up.sql":    {Data: []byte("ALTER TABLE users ADD COLUMN name TEXT")},
	}
	migrations, err := loadMigrations(fs, "migrations")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}
	if migrations[0].Version != 1 || migrations[1].Version != 2 {
		t.Fatalf("unexpected versions: %v", migrations)
	}
}

func TestLoadMigrationsInvalidVersion(t *testing.T) {
	fs := fstest.MapFS{
		"migrations/abc_init.up.sql": {Data: []byte("CREATE TABLE users")},
	}
	migrations, err := loadMigrations(fs, "migrations")
	// Invalid filenames are skipped, not errors
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(migrations) != 0 {
		t.Fatalf("expected 0 migrations, got %d", len(migrations))
	}
}

func TestLoadMigrationsReadError(t *testing.T) {
	fs := fstest.MapFS{}
	_, err := loadMigrations(fs, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
