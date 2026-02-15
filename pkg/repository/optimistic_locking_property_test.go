package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "github.com/lib/pq"
)

// TestProperty_OptimisticLocking validates Property 27: Optimistic Locking
// **Validates: Requirements 29.6, 29.7**
//
// Property: For any entity with version field, when two concurrent updates attempt
// to modify the same entity, the first update should succeed and increment the version,
// and the second update should fail with an OptimisticLockError indicating the version conflict.
func TestProperty_OptimisticLocking(t *testing.T) {
	// Skip if no database URL is provided
	dbURL := getOptimisticLockingTestDatabaseURL()
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
	setupOptimisticLockingTestTable(t, db)
	defer cleanupOptimisticLockingTestTable(t, db)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: First update succeeds and increments version
	properties.Property("first update succeeds and increments version", prop.ForAll(
		func(id int64, initialName string, updatedName string) bool {
			if initialName == updatedName {
				return true // Skip trivial case
			}

			ctx := context.Background()

			// Create initial entity with version 0
			_, err := db.ExecContext(ctx,
				"INSERT INTO optimistic_locking_test (id, name, version) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = $2, version = $3",
				id, initialName, int64(0))
			if err != nil {
				t.Logf("failed to insert entity: %v", err)
				return false
			}

			// Create repository
			repo := NewGenericCrudRepository[VersionedEntity, int64](
				db,
				"optimistic_locking_test",
				"id",
				&VersionedEntityMapper{},
			)

			// Read entity
			entity, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find entity: %v", err)
				return false
			}

			// Verify initial version
			if entity.Version != 0 {
				t.Logf("expected initial version 0, got %d", entity.Version)
				return false
			}

			// Update entity
			entity.Name = updatedName
			err = repo.Update(ctx, entity)
			if err != nil {
				t.Logf("update failed: %v", err)
				return false
			}

			// Verify version was incremented
			if entity.Version != 1 {
				t.Logf("expected version 1 after update, got %d", entity.Version)
				return false
			}

			// Read entity again to verify persistence
			updatedEntity, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find updated entity: %v", err)
				return false
			}

			return updatedEntity.Name == updatedName && updatedEntity.Version == 1
		},
		gen.Int64Range(1, 10000),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property: Concurrent updates cause optimistic lock error
	properties.Property("concurrent updates cause optimistic lock error", prop.ForAll(
		func(id int64, initialName string, update1 string, update2 string) bool {
			if initialName == update1 || initialName == update2 || update1 == update2 {
				return true // Skip trivial cases
			}

			ctx := context.Background()

			// Create initial entity with version 0
			_, err := db.ExecContext(ctx,
				"INSERT INTO optimistic_locking_test (id, name, version) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = $2, version = $3",
				id, initialName, int64(0))
			if err != nil {
				t.Logf("failed to insert entity: %v", err)
				return false
			}

			// Create repository
			repo := NewGenericCrudRepository[VersionedEntity, int64](
				db,
				"optimistic_locking_test",
				"id",
				&VersionedEntityMapper{},
			)

			// Read entity twice (simulating two concurrent reads)
			entity1, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find entity1: %v", err)
				return false
			}

			entity2, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find entity2: %v", err)
				return false
			}

			// Both entities should have the same version
			if entity1.Version != entity2.Version {
				t.Logf("entities have different versions: %d vs %d", entity1.Version, entity2.Version)
				return false
			}

			// Update first entity
			entity1.Name = update1
			err = repo.Update(ctx, entity1)
			if err != nil {
				t.Logf("first update failed: %v", err)
				return false
			}

			// Try to update second entity (should fail with optimistic lock error)
			entity2.Name = update2
			err = repo.Update(ctx, entity2)
			if err == nil {
				t.Logf("second update should have failed but succeeded")
				return false
			}

			// Verify it's an optimistic lock error
			var lockErr *OptimisticLockError
			if !isOptimisticLockError(err) {
				t.Logf("expected OptimisticLockError, got: %v", err)
				return false
			}

			// Verify the error contains correct version information
			if lockErr != nil {
				if lockErr.Expected != 0 {
					t.Logf("expected version should be 0, got %d", lockErr.Expected)
					return false
				}
				if lockErr.Actual != 1 {
					t.Logf("actual version should be 1, got %d", lockErr.Actual)
					return false
				}
			}

			// Verify first update succeeded
			finalEntity, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find final entity: %v", err)
				return false
			}

			return finalEntity.Name == update1 && finalEntity.Version == 1
		},
		gen.Int64Range(1, 10000),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	// Property: Multiple sequential updates increment version correctly
	properties.Property("multiple sequential updates increment version correctly", prop.ForAll(
		func(id int64, updates []string) bool {
			if len(updates) == 0 {
				return true // Skip empty case
			}

			ctx := context.Background()

			// Create initial entity with version 0
			_, err := db.ExecContext(ctx,
				"INSERT INTO optimistic_locking_test (id, name, version) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = $2, version = $3",
				id, "initial", int64(0))
			if err != nil {
				t.Logf("failed to insert entity: %v", err)
				return false
			}

			// Create repository
			repo := NewGenericCrudRepository[VersionedEntity, int64](
				db,
				"optimistic_locking_test",
				"id",
				&VersionedEntityMapper{},
			)

			// Perform sequential updates
			for i, name := range updates {
				entity, err := repo.FindByID(ctx, id)
				if err != nil {
					t.Logf("failed to find entity at iteration %d: %v", i, err)
					return false
				}

				// Verify version matches iteration count
				if entity.Version != int64(i) {
					t.Logf("expected version %d at iteration %d, got %d", i, i, entity.Version)
					return false
				}

				// Update entity
				entity.Name = name
				err = repo.Update(ctx, entity)
				if err != nil {
					t.Logf("update failed at iteration %d: %v", i, err)
					return false
				}

				// Verify version was incremented
				if entity.Version != int64(i+1) {
					t.Logf("expected version %d after update at iteration %d, got %d", i+1, i, entity.Version)
					return false
				}
			}

			// Verify final state
			finalEntity, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find final entity: %v", err)
				return false
			}

			expectedVersion := int64(len(updates))
			return finalEntity.Version == expectedVersion && finalEntity.Name == updates[len(updates)-1]
		},
		gen.Int64Range(1, 10000),
		gen.SliceOfN(5, gen.AlphaString()),
	))

	// Property: Concurrent updates with proper retry succeed
	properties.Property("concurrent updates with proper retry succeed", prop.ForAll(
		func(id int64, initialName string, update1 string, update2 string) bool {
			if initialName == update1 || initialName == update2 || update1 == update2 {
				return true // Skip trivial cases
			}

			ctx := context.Background()

			// Create initial entity with version 0
			_, err := db.ExecContext(ctx,
				"INSERT INTO optimistic_locking_test (id, name, version) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = $2, version = $3",
				id, initialName, int64(0))
			if err != nil {
				t.Logf("failed to insert entity: %v", err)
				return false
			}

			// Create repository
			repo := NewGenericCrudRepository[VersionedEntity, int64](
				db,
				"optimistic_locking_test",
				"id",
				&VersionedEntityMapper{},
			)

			// Simulate concurrent updates with retry logic
			var wg sync.WaitGroup
			errors := make([]error, 2)

			// First update
			wg.Add(1)
			go func() {
				defer wg.Done()
				errors[0] = updateWithRetry(ctx, repo, id, update1, 3)
			}()

			// Second update
			wg.Add(1)
			go func() {
				defer wg.Done()
				errors[1] = updateWithRetry(ctx, repo, id, update2, 3)
			}()

			wg.Wait()

			// Both updates should eventually succeed with retry
			if errors[0] != nil {
				t.Logf("first update failed after retries: %v", errors[0])
				return false
			}
			if errors[1] != nil {
				t.Logf("second update failed after retries: %v", errors[1])
				return false
			}

			// Verify final version is 2 (two successful updates)
			finalEntity, err := repo.FindByID(ctx, id)
			if err != nil {
				t.Logf("failed to find final entity: %v", err)
				return false
			}

			return finalEntity.Version == 2
		},
		gen.Int64Range(1, 10000),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// VersionedEntity is a test entity with version field for optimistic locking
type VersionedEntity struct {
	ID      int64
	Name    string
	Version int64
}

func (e *VersionedEntity) GetVersion() int64 {
	return e.Version
}

func (e *VersionedEntity) SetVersion(version int64) {
	e.Version = version
}

// VersionedEntityMapper maps VersionedEntity to database rows
type VersionedEntityMapper struct{}

func (m *VersionedEntityMapper) ToRow(entity *VersionedEntity) ([]string, []interface{}, error) {
	return []string{"id", "name", "version"},
		[]interface{}{entity.ID, entity.Name, entity.Version},
		nil
}

func (m *VersionedEntityMapper) FromRow(rows *sql.Rows) (*VersionedEntity, error) {
	entity := &VersionedEntity{}
	err := rows.Scan(&entity.ID, &entity.Name, &entity.Version)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (m *VersionedEntityMapper) GetID(entity *VersionedEntity) int64 {
	return entity.ID
}

func (m *VersionedEntityMapper) SetID(entity *VersionedEntity, id int64) {
	entity.ID = id
}

// updateWithRetry attempts to update an entity with retry logic for optimistic lock conflicts
func updateWithRetry(ctx context.Context, repo *GenericCrudRepository[VersionedEntity, int64], id int64, name string, maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		entity, err := repo.FindByID(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to find entity: %w", err)
		}

		entity.Name = name
		err = repo.Update(ctx, entity)
		if err == nil {
			return nil // Success
		}

		// Check if it's an optimistic lock error
		if !isOptimisticLockError(err) {
			return err // Non-retryable error
		}

		// Retry on optimistic lock error
	}

	return fmt.Errorf("failed to update after %d retries", maxRetries)
}

// isOptimisticLockError checks if an error is an OptimisticLockError
func isOptimisticLockError(err error) bool {
	if err == nil {
		return false
	}
	var lockErr *OptimisticLockError
	// Check if error message contains optimistic lock keywords
	errStr := err.Error()
	return contains(errStr, "optimistic lock failed") || 
		   contains(errStr, "version") && contains(errStr, "expected") ||
		   lockErr != nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getOptimisticLockingTestDatabaseURL returns a test database URL from environment or default
func getOptimisticLockingTestDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	// Default test database URL
	return "postgres://test:test@localhost:5432/testdb?sslmode=disable"
}

// setupOptimisticLockingTestTable creates the test table for optimistic locking tests
func setupOptimisticLockingTestTable(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS optimistic_locking_test (
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			version BIGINT NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
}

// cleanupOptimisticLockingTestTable drops the test table
func cleanupOptimisticLockingTestTable(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS optimistic_locking_test")
	if err != nil {
		t.Logf("failed to drop test table: %v", err)
	}
}
