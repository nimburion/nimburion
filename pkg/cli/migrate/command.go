package migrate

import (
	"context"
	"database/sql"
	"github.com/nimburion/nimburion/pkg/migrate"
	"io/fs"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// PendingMigration contains an unapplied migration entry for status output.
type PendingMigration = migrate.PendingMigration

// Status is the normalized migration status used by the CLI helper.
type Status = migrate.Status

// Operations defines service-specific migration hooks.
type Operations = migrate.Operations

// Options configures migration command behavior.
type Options = migrate.Options

// Migration describes one up/down migration pair loaded from files.
type Migration = migrate.Migration

// SQLManager executes SQL migration operations.
type SQLManager = migrate.SQLManager

// OperationsBuilder builds migrate Operations using an opened SQL database handle.
type OperationsBuilder = migrate.OperationsBuilder

// Run executes migrate subcommands using shared parsing, timeout and logging.
func Run(args []string, opts Options, ops Operations) error {
	return migrate.Run(args, opts, ops)
}

// RunParsed executes a parsed migration command.
func RunParsed(subcommand string, steps int, opts Options, ops Operations) error {
	return migrate.RunParsed(subcommand, steps, opts, ops)
}

// ParseArgs parses [up|down|status] [steps], defaulting to "up".
func ParseArgs(args []string) (string, int, error) {
	return migrate.ParseArgs(args)
}

// NewSQLManager creates a SQL manager backed by migration files.
func NewSQLManager(db *sql.DB, migrationFiles fs.FS, migrationsDir string) (*SQLManager, error) {
	return migrate.NewSQLManager(db, migrationFiles, migrationsDir)
}

// RunWithDBContext builds operations from an existing DB handle and runs the migrate command.
func RunWithDBContext(
	ctx context.Context,
	sqlDB *sql.DB,
	path string,
	serviceName string,
	direction string,
	steps int,
	timeout time.Duration,
	log logger.Logger,
	builder func(context.Context, *sql.DB, string) (Operations, error),
) error {
	return migrate.RunWithDBContext(ctx, sqlDB, path, serviceName, direction, steps, timeout, log, builder)
}

// RunWithSQLDriver opens the database, builds operations, and runs the migrate command.
func RunWithSQLDriver(
	driverName string,
	dbURL string,
	path string,
	serviceName string,
	direction string,
	steps int,
	timeout time.Duration,
	log logger.Logger,
	builder OperationsBuilder,
) error {
	return migrate.RunWithSQLDriver(driverName, dbURL, path, serviceName, direction, steps, timeout, log, builder)
}

// RunWithSQLDriverContext is like RunWithSQLDriver but allows supplying a context for builder.
func RunWithSQLDriverContext(
	ctx context.Context,
	driverName string,
	dbURL string,
	path string,
	serviceName string,
	direction string,
	steps int,
	timeout time.Duration,
	log logger.Logger,
	builder func(context.Context, *sql.DB, string) (Operations, error),
) error {
	return migrate.RunWithSQLDriverContext(ctx, driverName, dbURL, path, serviceName, direction, steps, timeout, log, builder)
}
