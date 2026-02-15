package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// PostgreSQLAdapter provides PostgreSQL database connectivity with connection pooling
type PostgreSQLAdapter struct {
	db     *sql.DB
	logger logger.Logger
	config Config
}

// Config holds PostgreSQL connection configuration
type Config struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	QueryTimeout    time.Duration
}

// NewPostgreSQLAdapter creates a new PostgreSQL adapter with connection pooling
func NewPostgreSQLAdapter(cfg Config, log logger.Logger) (*PostgreSQLAdapter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	// Open database connection
	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("PostgreSQL connection established",
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
		"conn_max_lifetime", cfg.ConnMaxLifetime,
		"conn_max_idle_time", cfg.ConnMaxIdleTime,
	)

	return &PostgreSQLAdapter{
		db:     db,
		logger: log,
		config: cfg,
	}, nil
}

// DB returns the underlying *sql.DB for direct access when needed
func (a *PostgreSQLAdapter) DB() *sql.DB {
	return a.db
}

// Ping verifies the database connection is alive
func (a *PostgreSQLAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// HealthCheck verifies the database connection is healthy with a timeout
func (a *PostgreSQLAdapter) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := a.db.PingContext(ctx); err != nil {
		a.logger.Error("PostgreSQL health check failed", "error", err)
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

// Close gracefully closes the database connection
func (a *PostgreSQLAdapter) Close() error {
	a.logger.Info("closing PostgreSQL connection")

	if err := a.db.Close(); err != nil {
		a.logger.Error("failed to close PostgreSQL connection", "error", err)
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	a.logger.Info("PostgreSQL connection closed successfully")
	return nil
}

// WithTransaction executes the given function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (a *PostgreSQLAdapter) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// Begin transaction
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Handle panic during transaction
	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic
			if rbErr := tx.Rollback(); rbErr != nil {
				a.logger.Error("failed to rollback transaction after panic",
					"panic", p,
					"rollback_error", rbErr,
				)
			}
			// Re-panic after rollback
			panic(p)
		}
	}()

	// Store transaction in context for nested operations
	txCtx := context.WithValue(ctx, txContextKey, tx)

	// Execute function within transaction
	if err := fn(txCtx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			a.logger.Error("failed to rollback transaction",
				"original_error", err,
				"rollback_error", rbErr,
			)
			return fmt.Errorf("failed to rollback transaction: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// txContextKey is the key used to store transactions in context
type contextKey string

const txContextKey contextKey = "tx"

// GetTx extracts a transaction from the context, if present
// This allows nested operations to use the same transaction
func GetTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txContextKey).(*sql.Tx)
	return tx, ok
}

// ExecContext executes a query with the transaction from context if available
// Otherwise uses the regular database connection
func (a *PostgreSQLAdapter) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	queryCtx, cancel := a.withQueryTimeout(ctx)
	defer cancel()
	if tx, ok := GetTx(ctx); ok {
		return tx.ExecContext(queryCtx, query, args...)
	}
	return a.db.ExecContext(queryCtx, query, args...)
}

// QueryContext executes a query with the transaction from context if available
// Otherwise uses the regular database connection
func (a *PostgreSQLAdapter) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	queryCtx, cancel := a.withQueryTimeout(ctx)
	defer cancel()
	if tx, ok := GetTx(ctx); ok {
		return tx.QueryContext(queryCtx, query, args...)
	}
	return a.db.QueryContext(queryCtx, query, args...)
}

// QueryRowContext executes a query that returns a single row with the transaction from context if available
// Otherwise uses the regular database connection
func (a *PostgreSQLAdapter) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	queryCtx, cancel := a.withQueryTimeout(ctx)
	defer cancel()
	if tx, ok := GetTx(ctx); ok {
		return tx.QueryRowContext(queryCtx, query, args...)
	}
	return a.db.QueryRowContext(queryCtx, query, args...)
}

func (a *PostgreSQLAdapter) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.config.QueryTimeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.config.QueryTimeout)
}
