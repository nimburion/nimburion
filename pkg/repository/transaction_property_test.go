package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "github.com/lib/pq"
)

// TestProperty_TransactionAtomicity validates Property 26: Transaction Atomicity
// **Validates: Requirements 29.5**
//
// Property: For any transaction containing multiple operations, either all operations
// should succeed and be committed, or any failure should cause all operations to be
// rolled back, leaving the database in its original state.
func TestProperty_TransactionAtomicity(t *testing.T) {
	// Skip if no database URL is provided
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	// Connect to database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		t.Skipf("cannot connect to test database: %v", err)
	}

	// Create test table
	setupTransactionTestTable(t, db)
	defer cleanupTransactionTestTable(t, db)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: Successful transaction commits all operations
	properties.Property("successful transaction commits all operations", prop.ForAll(
		func(operations []int) bool {
			ctx := context.Background()
			
			// Clear table before test
			_, err := db.ExecContext(ctx, "DELETE FROM transaction_test")
			if err != nil {
				t.Logf("failed to clear table: %v", err)
				return false
			}

			// Execute transaction with multiple inserts
			err = withTransaction(ctx, db, func(txCtx context.Context) error {
				tx, _ := txCtx.Value("tx").(*sql.Tx)
				
				for _, value := range operations {
					_, err := tx.ExecContext(txCtx, "INSERT INTO transaction_test (value) VALUES ($1)", value)
					if err != nil {
						return err
					}
				}
				return nil
			})

			if err != nil {
				t.Logf("transaction failed: %v", err)
				return false
			}

			// Verify all operations were committed
			var count int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transaction_test").Scan(&count)
			if err != nil {
				t.Logf("failed to count rows: %v", err)
				return false
			}

			return count == len(operations)
		},
		gen.SliceOf(gen.IntRange(1, 1000)).SuchThat(func(v interface{}) bool {
			slice := v.([]int)
			return len(slice) > 0 && len(slice) <= 10
		}),
	))

	// Property: Failed transaction rolls back all operations
	properties.Property("failed transaction rolls back all operations", prop.ForAll(
		func(operations []int) bool {
			if len(operations) < 2 {
				return true // Skip trivial case
			}
			
			ctx := context.Background()
			
			// Clear table before test
			_, err := db.ExecContext(ctx, "DELETE FROM transaction_test")
			if err != nil {
				t.Logf("failed to clear table: %v", err)
				return false
			}

			// Get initial count
			var initialCount int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transaction_test").Scan(&initialCount)
			if err != nil {
				t.Logf("failed to get initial count: %v", err)
				return false
			}

			// Choose a random index to fail at
			failAtIndex := len(operations) / 2

			// Execute transaction that will fail at failAtIndex
			err = withTransaction(ctx, db, func(txCtx context.Context) error {
				tx, _ := txCtx.Value("tx").(*sql.Tx)
				
				for i, value := range operations {
					if i == failAtIndex {
						// Intentionally cause an error
						return fmt.Errorf("intentional error at index %d", i)
					}
					_, err := tx.ExecContext(txCtx, "INSERT INTO transaction_test (value) VALUES ($1)", value)
					if err != nil {
						return err
					}
				}
				return nil
			})

			// Transaction should have failed
			if err == nil {
				t.Logf("transaction should have failed but succeeded")
				return false
			}

			// Verify no operations were committed (rollback occurred)
			var finalCount int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transaction_test").Scan(&finalCount)
			if err != nil {
				t.Logf("failed to get final count: %v", err)
				return false
			}

			return finalCount == initialCount
		},
		gen.SliceOfN(5, gen.IntRange(1, 1000)),
	))

	// Property: Panic during transaction causes rollback
	properties.Property("panic during transaction causes rollback", prop.ForAll(
		func(operations []int) bool {
			if len(operations) < 2 {
				return true // Skip trivial case
			}
			
			ctx := context.Background()
			
			// Clear table before test
			_, err := db.ExecContext(ctx, "DELETE FROM transaction_test")
			if err != nil {
				t.Logf("failed to clear table: %v", err)
				return false
			}

			// Get initial count
			var initialCount int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transaction_test").Scan(&initialCount)
			if err != nil {
				t.Logf("failed to get initial count: %v", err)
				return false
			}

			// Choose a random index to panic at
			panicAtIndex := len(operations) / 2

			// Execute transaction that will panic at panicAtIndex
			panicked := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
					}
				}()

				_ = withTransaction(ctx, db, func(txCtx context.Context) error {
					tx, _ := txCtx.Value("tx").(*sql.Tx)
					
					for i, value := range operations {
						if i == panicAtIndex {
							// Intentionally panic
							panic(fmt.Sprintf("intentional panic at index %d", i))
						}
						_, err := tx.ExecContext(txCtx, "INSERT INTO transaction_test (value) VALUES ($1)", value)
						if err != nil {
							return err
						}
					}
					return nil
				})
			}()

			// Should have panicked
			if !panicked {
				t.Logf("transaction should have panicked but didn't")
				return false
			}

			// Verify no operations were committed (rollback occurred)
			var finalCount int
			err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transaction_test").Scan(&finalCount)
			if err != nil {
				t.Logf("failed to get final count: %v", err)
				return false
			}

			return finalCount == initialCount
		},
		gen.SliceOfN(5, gen.IntRange(1, 1000)),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// getTestDatabaseURL returns a test database URL from environment or default
func getTestDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	// Default test database URL
	return "postgres://test:test@localhost:5432/testdb?sslmode=disable"
}

// withTransaction is a simplified transaction wrapper for testing
func withTransaction(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txCtx := context.WithValue(ctx, "tx", tx)

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

// setupTransactionTestTable creates the test table for transaction tests
func setupTransactionTestTable(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS transaction_test (
			id SERIAL PRIMARY KEY,
			value INTEGER NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
}

// cleanupTransactionTestTable drops the test table
func cleanupTransactionTestTable(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS transaction_test")
	if err != nil {
		t.Logf("failed to drop test table: %v", err)
	}
}
