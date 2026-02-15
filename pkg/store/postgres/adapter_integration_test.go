package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPostgreSQLAdapter_Integration tests the PostgreSQL adapter with a real database
// using testcontainers.
//
// **Validates: Requirements 16.1-16.8, 38.1**
func TestPostgreSQLAdapter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Create logger
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test adapter creation
	t.Run("CreateAdapter", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Verify connection pool settings
		stats := adapter.DB().Stats()
		if stats.MaxOpenConnections != cfg.MaxOpenConns {
			t.Errorf("Expected MaxOpenConnections=%d, got %d", cfg.MaxOpenConns, stats.MaxOpenConnections)
		}
	})

	// Test connection and ping
	t.Run("ConnectionAndPing", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Test ping
		if err := adapter.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Test health check
		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("Health check failed: %v", err)
		}
	})

	// Test graceful shutdown
	t.Run("GracefulShutdown", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}

		// Close adapter
		if err := adapter.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Verify connection is closed
		if err := adapter.Ping(ctx); err == nil {
			t.Error("Expected ping to fail after close, but it succeeded")
		}
	})

	// Test transaction commit
	t.Run("TransactionCommit", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Create test table
		_, err = adapter.DB().ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS test_commit (
				id SERIAL PRIMARY KEY,
				value TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create test table: %v", err)
		}
		defer adapter.DB().ExecContext(ctx, "DROP TABLE IF EXISTS test_commit")

		// Execute transaction
		err = adapter.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := adapter.ExecContext(txCtx, "INSERT INTO test_commit (value) VALUES ($1)", "test_value")
			return err
		})
		if err != nil {
			t.Fatalf("Transaction failed: %v", err)
		}

		// Verify data was committed
		var count int
		err = adapter.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM test_commit WHERE value = $1", "test_value").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 row, got %d", count)
		}
	})

	// Test transaction rollback
	t.Run("TransactionRollback", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Create test table
		_, err = adapter.DB().ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS test_rollback (
				id SERIAL PRIMARY KEY,
				value TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create test table: %v", err)
		}
		defer adapter.DB().ExecContext(ctx, "DROP TABLE IF EXISTS test_rollback")

		// Execute transaction that should rollback
		err = adapter.WithTransaction(ctx, func(txCtx context.Context) error {
			_, err := adapter.ExecContext(txCtx, "INSERT INTO test_rollback (value) VALUES ($1)", "should_rollback")
			if err != nil {
				return err
			}
			// Return error to trigger rollback
			return context.Canceled
		})

		// Transaction should have failed
		if err == nil {
			t.Fatal("Expected transaction to fail, but it succeeded")
		}

		// Verify data was NOT committed
		var count int
		err = adapter.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM test_rollback WHERE value = $1", "should_rollback").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 rows (rollback), got %d", count)
		}
	})

	// Test nested transaction context
	t.Run("NestedTransactionContext", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Create test table
		_, err = adapter.DB().ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS test_nested (
				id SERIAL PRIMARY KEY,
				value TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create test table: %v", err)
		}
		defer adapter.DB().ExecContext(ctx, "DROP TABLE IF EXISTS test_nested")

		// Execute transaction with nested operations
		err = adapter.WithTransaction(ctx, func(txCtx context.Context) error {
			// First insert
			_, err := adapter.ExecContext(txCtx, "INSERT INTO test_nested (value) VALUES ($1)", "first")
			if err != nil {
				return err
			}

			// Nested operation using same transaction context
			_, err = adapter.ExecContext(txCtx, "INSERT INTO test_nested (value) VALUES ($1)", "second")
			return err
		})
		if err != nil {
			t.Fatalf("Transaction failed: %v", err)
		}

		// Verify both inserts were committed
		var count int
		err = adapter.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM test_nested").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 rows, got %d", count)
		}
	})

	// Test query operations
	t.Run("QueryOperations", func(t *testing.T) {
		cfg := Config{
			URL:             connStr,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			QueryTimeout:    10 * time.Second,
		}

		adapter, err := NewPostgreSQLAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Create test table
		_, err = adapter.DB().ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS test_query (
				id SERIAL PRIMARY KEY,
				value TEXT
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create test table: %v", err)
		}
		defer adapter.DB().ExecContext(ctx, "DROP TABLE IF EXISTS test_query")

		// Insert test data
		_, err = adapter.ExecContext(ctx, "INSERT INTO test_query (value) VALUES ($1), ($2), ($3)", "a", "b", "c")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		// Test QueryContext
		rows, err := adapter.QueryContext(ctx, "SELECT value FROM test_query ORDER BY value")
		if err != nil {
			t.Fatalf("QueryContext failed: %v", err)
		}
		defer rows.Close()

		values := []string{}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				t.Fatalf("Failed to scan row: %v", err)
			}
			values = append(values, value)
		}

		expected := []string{"a", "b", "c"}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected value[%d]=%s, got %s", i, expected[i], v)
			}
		}

		// Test QueryRowContext
		var count int
		err = adapter.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_query").Scan(&count)
		if err != nil {
			t.Fatalf("QueryRowContext failed: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected count=3, got %d", count)
		}
	})
}
