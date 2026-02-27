package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var migrationNamePattern = regexp.MustCompile(`^(\d+)_([a-zA-Z0-9_\-]+)\.(up|down)\.sql$`)

// Migration represents a database migration with up and down SQL scripts.
type Migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
}

// SQLManager manages database migrations with support for up/down operations and status tracking.
type SQLManager struct {
	db         *sql.DB
	migrations []Migration
}

// NewSQLManager creates a new SQLManager instance.
func NewSQLManager(db *sql.DB, migrationFiles fs.FS, migrationsDir string) (*SQLManager, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if migrationFiles == nil {
		return nil, fmt.Errorf("migration files filesystem is required")
	}
	if strings.TrimSpace(migrationsDir) == "" {
		return nil, fmt.Errorf("migration directory is required")
	}

	migrations, err := loadMigrations(migrationFiles, migrationsDir)
	if err != nil {
		return nil, err
	}

	return &SQLManager{db: db, migrations: migrations}, nil
}

// Up applies all pending migrations in order.
func (m *SQLManager) Up(ctx context.Context) (int, error) {
	if err := m.ensureMetadataTable(ctx); err != nil {
		return 0, err
	}

	applied, err := m.appliedSet(ctx)
	if err != nil {
		return 0, err
	}

	appliedCount := 0
	for _, migration := range m.migrations {
		if _, already := applied[migration.Version]; already {
			continue
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return appliedCount, fmt.Errorf("begin migration transaction %d: %w", migration.Version, err)
		}

		if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
			_ = tx.Rollback()
			return appliedCount, fmt.Errorf("apply migration %d_%s: %w", migration.Version, migration.Name, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, NOW())`, migration.Version); err != nil {
			_ = tx.Rollback()
			return appliedCount, fmt.Errorf("record migration %d: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return appliedCount, fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}

		appliedCount++
	}

	return appliedCount, nil
}

// Down rolls back the specified number of migrations.
func (m *SQLManager) Down(ctx context.Context, steps int) (int, error) {
	if steps <= 0 {
		steps = 1
	}
	if err := m.ensureMetadataTable(ctx); err != nil {
		return 0, err
	}

	applied, err := m.appliedVersionsDesc(ctx)
	if err != nil {
		return 0, err
	}
	if len(applied) == 0 {
		return 0, nil
	}

	if steps > len(applied) {
		steps = len(applied)
	}

	reverted := 0
	for _, version := range applied[:steps] {
		migration, ok := m.migrationByVersion(version)
		if !ok {
			return reverted, fmt.Errorf("migration definition not found for applied version %d", version)
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return reverted, fmt.Errorf("begin rollback transaction %d: %w", version, err)
		}

		if strings.TrimSpace(migration.DownSQL) == "" {
			_ = tx.Rollback()
			return reverted, fmt.Errorf("down migration missing for version %d", version)
		}

		if _, err := tx.ExecContext(ctx, migration.DownSQL); err != nil {
			_ = tx.Rollback()
			return reverted, fmt.Errorf("rollback migration %d_%s: %w", migration.Version, migration.Name, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
			_ = tx.Rollback()
			return reverted, fmt.Errorf("delete migration record %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return reverted, fmt.Errorf("commit rollback %d: %w", version, err)
		}

		reverted++
	}

	return reverted, nil
}

// Status returns the HTTP status code that was written, or 0 if not yet written.
func (m *SQLManager) Status(ctx context.Context) (*Status, error) {
	if err := m.ensureMetadataTable(ctx); err != nil {
		return nil, err
	}

	appliedSet, err := m.appliedSet(ctx)
	if err != nil {
		return nil, err
	}

	appliedVersions := make([]int64, 0, len(appliedSet))
	for version := range appliedSet {
		appliedVersions = append(appliedVersions, version)
	}
	sort.Slice(appliedVersions, func(i, j int) bool {
		return appliedVersions[i] < appliedVersions[j]
	})

	pending := make([]PendingMigration, 0)
	for _, migration := range m.migrations {
		if _, exists := appliedSet[migration.Version]; !exists {
			pending = append(pending, PendingMigration{Version: migration.Version, Name: migration.Name})
		}
	}

	return &Status{AppliedVersions: appliedVersions, Pending: pending}, nil
}

func (m *SQLManager) ensureMetadataTable(ctx context.Context) error {
	query := `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version BIGINT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`
	if _, err := m.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func (m *SQLManager) appliedSet(ctx context.Context) (map[int64]struct{}, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration version: %w", err)
		}
		versions[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return versions, nil
}

func (m *SQLManager) appliedVersionsDesc(ctx context.Context) ([]int64, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations descending: %w", err)
	}
	defer rows.Close()

	versions := make([]int64, 0)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration version: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations descending: %w", err)
	}
	return versions, nil
}

func (m *SQLManager) migrationByVersion(version int64) (Migration, bool) {
	for _, migration := range m.migrations {
		if migration.Version == version {
			return migration, true
		}
	}
	return Migration{}, false
}

func loadMigrations(migrationFiles fs.FS, migrationsDir string) ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migration files: %w", err)
	}

	type partial struct {
		version int64
		name    string
		up      string
		down    string
	}
	byVersion := make(map[int64]*partial)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		matches := migrationNamePattern.FindStringSubmatch(name)
		if len(matches) != 4 {
			continue
		}

		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", matches[1], err)
		}
		migrationName := matches[2]
		direction := matches[3]

		payload, err := fs.ReadFile(migrationFiles, migrationsDir+"/"+name)
		if err != nil {
			return nil, fmt.Errorf("read migration file %q: %w", name, err)
		}

		if _, ok := byVersion[version]; !ok {
			byVersion[version] = &partial{version: version, name: migrationName}
		}

		if direction == "up" {
			byVersion[version].up = string(payload)
		} else {
			byVersion[version].down = string(payload)
		}
	}

	versions := make([]int64, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})

	migrations := make([]Migration, 0, len(versions))
	for _, version := range versions {
		item := byVersion[version]
		if strings.TrimSpace(item.up) == "" {
			return nil, fmt.Errorf("missing up migration for version %d", version)
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    item.name,
			UpSQL:   item.up,
			DownSQL: item.down,
		})
	}

	return migrations, nil
}
