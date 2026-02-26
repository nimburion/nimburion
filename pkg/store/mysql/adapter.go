package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// MySQLAdapter provides MySQL connectivity with pooled connections.
type MySQLAdapter struct {
	db     *sql.DB
	logger logger.Logger
	config Config
}

// Config holds MySQL configuration.
type Config struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	QueryTimeout    time.Duration
}

// Cosa fa: inizializza un adapter MySQL con validazione e ping iniziale.
// Cosa NON fa: non esegue migrazioni schema né provisioning database.
// Esempio minimo: adapter, err := mysql.NewMySQLAdapter(cfg, log)
func NewMySQLAdapter(cfg Config, log logger.Logger) (*MySQLAdapter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	db, err := sql.Open("mysql", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping mysql database: %w", err)
	}

	log.Info("MySQL connection established",
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
		"conn_max_lifetime", cfg.ConnMaxLifetime,
		"conn_max_idle_time", cfg.ConnMaxIdleTime,
	)

	return &MySQLAdapter{db: db, logger: log, config: cfg}, nil
}

// DB TODO: add description
func (a *MySQLAdapter) DB() *sql.DB {
	return a.db
}

// Ping performs a basic connectivity check to verify the service is reachable.
func (a *MySQLAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// HealthCheck verifies the component is operational and can perform its intended function.
func (a *MySQLAdapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.db.PingContext(hcCtx); err != nil {
		a.logger.Error("MySQL health check failed", "error", err)
		return fmt.Errorf("mysql health check failed: %w", err)
	}
	return nil
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (a *MySQLAdapter) Close() error {
	a.logger.Info("closing MySQL connection")
	if err := a.db.Close(); err != nil {
		a.logger.Error("failed to close MySQL connection", "error", err)
		return fmt.Errorf("failed to close mysql connection: %w", err)
	}
	a.logger.Info("MySQL connection closed successfully")
	return nil
}

type contextKey string

const txContextKey contextKey = "mysql_tx"

// GetTx TODO: add description
func GetTx(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txContextKey).(*sql.Tx)
	return tx, ok
}

// Cosa fa: esegue fn in transazione con commit/rollback automatici.
// Cosa NON fa: non gestisce retry applicativi su deadlock o timeout.
// Esempio minimo: err := adapter.WithTransaction(ctx, func(txCtx context.Context) error { return nil })
func (a *MySQLAdapter) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				a.logger.Error("failed to rollback transaction after panic", "panic", p, "rollback_error", rbErr)
			}
			panic(p)
		}
	}()

	txCtx := context.WithValue(ctx, txContextKey, tx)
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("failed to rollback transaction: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// ExecContext TODO: add description
func (a *MySQLAdapter) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	queryCtx, cancel := a.withQueryTimeout(ctx)
	defer cancel()
	if tx, ok := GetTx(ctx); ok {
		return tx.ExecContext(queryCtx, query, args...)
	}
	return a.db.ExecContext(queryCtx, query, args...)
}

// QueryContext TODO: add description
func (a *MySQLAdapter) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	queryCtx, cancel := a.withQueryTimeout(ctx)
	defer cancel()
	if tx, ok := GetTx(ctx); ok {
		return tx.QueryContext(queryCtx, query, args...)
	}
	return a.db.QueryContext(queryCtx, query, args...)
}

// QueryRowContext TODO: add description
func (a *MySQLAdapter) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	queryCtx, cancel := a.withQueryTimeout(ctx)
	defer cancel()
	if tx, ok := GetTx(ctx); ok {
		return tx.QueryRowContext(queryCtx, query, args...)
	}
	return a.db.QueryRowContext(queryCtx, query, args...)
}

func (a *MySQLAdapter) withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.config.QueryTimeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.config.QueryTimeout)
}
