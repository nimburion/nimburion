package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// OperationsBuilder builds migrate Operations using an opened SQL database handle.
type OperationsBuilder func(db *sql.DB, path string) (Operations, error)

// RunWithDBContext builds operations from an existing DB handle and runs the migrate command.
func RunWithDBContext(ctx context.Context, sqlDB *sql.DB, path, serviceName, direction string, steps int, timeout time.Duration, log logger.Logger, builder func(context.Context, *sql.DB, string) (Operations, error)) error {
	if builder == nil {
		return fmt.Errorf("operations builder is required")
	}
	if sqlDB == nil {
		return fmt.Errorf("database handle is required")
	}

	ops, err := builder(ctx, sqlDB, path)
	if err != nil {
		return err
	}

	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return RunParsed(direction, steps, Options{
		ServiceName: serviceName,
		Path:        path,
		Timeout:     timeout,
		Logger:      log,
	}, ops)
}

// RunWithSQLDriver opens the database, builds operations, and runs the migrate command.
func RunWithSQLDriver(driverName, dbURL, path, serviceName, direction string, steps int, timeout time.Duration, log logger.Logger, builder OperationsBuilder) error {
	if builder == nil {
		return fmt.Errorf("operations builder is required")
	}
	if driverName == "" {
		return fmt.Errorf("driver name is required")
	}

	sqlDB, err := sql.Open(driverName, dbURL)
	if err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return RunWithDBContext(context.Background(), sqlDB, path, serviceName, direction, steps, timeout, log, func(_ context.Context, db *sql.DB, p string) (Operations, error) {
		return builder(db, p)
	})
}

// RunWithSQLDriverContext is like RunWithSQLDriver but allows supplying a context for builder.
func RunWithSQLDriverContext(ctx context.Context, driverName, dbURL, path, serviceName, direction string, steps int, timeout time.Duration, log logger.Logger, builder func(context.Context, *sql.DB, string) (Operations, error)) error {
	if builder == nil {
		return fmt.Errorf("operations builder is required")
	}
	if driverName == "" {
		return fmt.Errorf("driver name is required")
	}

	sqlDB, err := sql.Open(driverName, dbURL)
	if err != nil {
		return fmt.Errorf("open database connection: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return RunWithDBContext(ctx, sqlDB, path, serviceName, direction, steps, timeout, log, builder)
}
