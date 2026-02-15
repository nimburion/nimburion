package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Validates: Requirements 29.1**
//
// Property 22: Generic CRUD Operations
//
// For any type T and ID type, the generic CRUD repository should support
// Create (insert new entity), Read (retrieve by ID), Update (modify existing entity),
// and Delete (remove entity) operations consistently across all adapter implementations.

func TestProperty_GenericCRUDOperations(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Create then Read returns same entity", prop.ForAll(
		func(id int64, name string, status string) bool {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			entity := &TestEntity{
				ID:      id,
				Name:    name,
				Status:  status,
				Version: 0,
			}

			// Mock Create
			mock.ExpectExec("INSERT INTO test_entities").
				WithArgs(id, name, status, int64(0)).
				WillReturnResult(sqlmock.NewResult(1, 1))

			// Mock Read
			rows := sqlmock.NewRows([]string{"id", "name", "status", "version"}).
				AddRow(id, name, status, int64(0))
			mock.ExpectQuery("SELECT \\* FROM test_entities WHERE id = \\$1").
				WithArgs(id).
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Create entity
			if err := repo.Create(context.Background(), entity); err != nil {
				t.Logf("Create failed: %v", err)
				return false
			}

			// Read entity back
			retrieved, err := repo.FindByID(context.Background(), id)
			if err != nil {
				t.Logf("FindByID failed: %v", err)
				return false
			}

			// Verify entity matches
			return retrieved.ID == entity.ID &&
				retrieved.Name == entity.Name &&
				retrieved.Status == entity.Status &&
				retrieved.Version == entity.Version
		},
		gen.Int64(),
		gen.AlphaString(),
		gen.OneConstOf("active", "inactive", "pending"),
	))

	properties.Property("Update modifies entity correctly", prop.ForAll(
		func(id int64, originalName string, updatedName string) bool {
			if originalName == updatedName {
				return true // Skip trivial case
			}

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			entity := &TestEntityNoVersion{
				ID:     id,
				Name:   originalName,
				Status: "active",
			}

			// Mock Update
			mock.ExpectExec("UPDATE test_entities SET").
				WithArgs(id, updatedName, "active", id).
				WillReturnResult(sqlmock.NewResult(0, 1))

			repo := NewGenericCrudRepository[TestEntityNoVersion, int64](
				db,
				"test_entities",
				"id",
				&TestEntityNoVersionMapper{},
			)

			// Update entity
			entity.Name = updatedName
			if err := repo.Update(context.Background(), entity); err != nil {
				t.Logf("Update failed: %v", err)
				return false
			}

			return true
		},
		gen.Int64(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.Property("Delete removes entity", prop.ForAll(
		func(id int64) bool {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Mock Delete
			mock.ExpectExec("DELETE FROM test_entities WHERE id = \\$1").
				WithArgs(id).
				WillReturnResult(sqlmock.NewResult(0, 1))

			// Mock FindByID after delete (should return no rows)
			rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
			mock.ExpectQuery("SELECT \\* FROM test_entities WHERE id = \\$1").
				WithArgs(id).
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Delete entity
			if err := repo.Delete(context.Background(), id); err != nil {
				t.Logf("Delete failed: %v", err)
				return false
			}

			// Verify entity is gone
			_, err = repo.FindByID(context.Background(), id)
			return err == sql.ErrNoRows
		},
		gen.Int64(),
	))

	properties.Property("FindByID returns error for non-existent entity", prop.ForAll(
		func(id int64) bool {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Mock FindByID with no results
			rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
			mock.ExpectQuery("SELECT \\* FROM test_entities WHERE id = \\$1").
				WithArgs(id).
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Try to find non-existent entity
			_, err = repo.FindByID(context.Background(), id)
			return err == sql.ErrNoRows
		},
		gen.Int64(),
	))

	properties.Property("Create with nil entity returns error", prop.ForAll(
		func() bool {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Try to create nil entity
			err = repo.Create(context.Background(), nil)
			return err != nil
		},
	))

	properties.Property("Update with nil entity returns error", prop.ForAll(
		func() bool {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			repo := NewGenericCrudRepository[TestEntityNoVersion, int64](
				db,
				"test_entities",
				"id",
				&TestEntityNoVersionMapper{},
			)

			// Try to update nil entity
			err = repo.Update(context.Background(), nil)
			return err != nil
		},
	))

	properties.Property("Count returns correct number of entities", prop.ForAll(
		func(count int) bool {
			if count < 0 {
				count = 0
			}
			if count > 1000 {
				count = 1000 // Limit for reasonable test
			}

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Mock Count
			rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(count))
			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM test_entities").
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Count entities
			result, err := repo.Count(context.Background(), Filter{})
			if err != nil {
				t.Logf("Count failed: %v", err)
				return false
			}

			return result == int64(count)
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// **Validates: Requirements 29.2**
//
// Property 23: Pagination Consistency
//
// For any repository query with pagination parameters (page, page_size), the results
// should return exactly page_size items (or fewer on last page), and consecutive pages
// should not overlap or skip items.

func TestProperty_PaginationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("pagination returns correct page size", prop.ForAll(
		func(totalItems int, pageSize int, page int) bool {
			// Constrain inputs to reasonable ranges
			if totalItems < 0 || totalItems > 100 {
				return true // Skip invalid inputs
			}
			if pageSize <= 0 || pageSize > 50 {
				return true // Skip invalid inputs
			}
			if page <= 0 || page > 10 {
				return true // Skip invalid inputs
			}

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Calculate expected items for this page
			offset := (page - 1) * pageSize
			if offset >= totalItems {
				// Page is beyond available data
				return true
			}

			expectedItems := pageSize
			if offset+pageSize > totalItems {
				expectedItems = totalItems - offset
			}

			// Mock the query with pagination
			rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
			for i := 0; i < expectedItems; i++ {
				itemID := int64(offset + i + 1)
				rows.AddRow(itemID, "Item", "active", int64(0))
			}

			mock.ExpectQuery("SELECT \\* FROM test_entities LIMIT \\$1 OFFSET \\$2").
				WithArgs(pageSize, offset).
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Query with pagination
			opts := QueryOptions{
				Pagination: Pagination{
					Page:     page,
					PageSize: pageSize,
				},
			}

			results, err := repo.FindAll(context.Background(), opts)
			if err != nil {
				t.Logf("FindAll failed: %v", err)
				return false
			}

			// Verify correct number of items returned
			if len(results) != expectedItems {
				t.Logf("Expected %d items, got %d (totalItems=%d, pageSize=%d, page=%d)",
					expectedItems, len(results), totalItems, pageSize, page)
				return false
			}

			return true
		},
		gen.IntRange(0, 100),   // totalItems
		gen.IntRange(1, 50),    // pageSize
		gen.IntRange(1, 10),    // page
	))

	properties.Property("consecutive pages do not overlap", prop.ForAll(
		func(totalItems int, pageSize int) bool {
			// Constrain inputs to reasonable ranges
			if totalItems < 2 || totalItems > 50 {
				return true // Skip invalid inputs
			}
			if pageSize <= 0 || pageSize >= totalItems {
				return true // Skip invalid inputs
			}

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Mock page 1
			page1Rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
			page1Count := pageSize
			if page1Count > totalItems {
				page1Count = totalItems
			}
			for i := 0; i < page1Count; i++ {
				page1Rows.AddRow(int64(i+1), "Item", "active", int64(0))
			}

			mock.ExpectQuery("SELECT \\* FROM test_entities LIMIT \\$1 OFFSET \\$2").
				WithArgs(pageSize, 0).
				WillReturnRows(page1Rows)

			// Mock page 2
			page2Offset := pageSize
			page2Count := pageSize
			if page2Offset+page2Count > totalItems {
				page2Count = totalItems - page2Offset
			}

			if page2Count > 0 {
				page2Rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
				for i := 0; i < page2Count; i++ {
					page2Rows.AddRow(int64(page2Offset+i+1), "Item", "active", int64(0))
				}

				mock.ExpectQuery("SELECT \\* FROM test_entities LIMIT \\$1 OFFSET \\$2").
					WithArgs(pageSize, page2Offset).
					WillReturnRows(page2Rows)
			}

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Get page 1
			page1Results, err := repo.FindAll(context.Background(), QueryOptions{
				Pagination: Pagination{Page: 1, PageSize: pageSize},
			})
			if err != nil {
				t.Logf("FindAll page 1 failed: %v", err)
				return false
			}

			// Get page 2 if there are more items
			if page2Count > 0 {
				page2Results, err := repo.FindAll(context.Background(), QueryOptions{
					Pagination: Pagination{Page: 2, PageSize: pageSize},
				})
				if err != nil {
					t.Logf("FindAll page 2 failed: %v", err)
					return false
				}

				// Check for overlaps - no ID should appear in both pages
				page1IDs := make(map[int64]bool)
				for _, entity := range page1Results {
					page1IDs[entity.ID] = true
				}

				for _, entity := range page2Results {
					if page1IDs[entity.ID] {
						t.Logf("ID %d appears in both page 1 and page 2 - overlap detected", entity.ID)
						return false
					}
				}

				// Check for gaps - IDs should be consecutive
				allIDs := []int64{}
				for _, entity := range page1Results {
					allIDs = append(allIDs, entity.ID)
				}
				for _, entity := range page2Results {
					allIDs = append(allIDs, entity.ID)
				}

				// Verify IDs are sequential (1, 2, 3, ...)
				for i, id := range allIDs {
					expectedID := int64(i + 1)
					if id != expectedID {
						t.Logf("Expected ID %d at position %d, got %d - gap detected", expectedID, i, id)
						return false
					}
				}
			}

			return true
		},
		gen.IntRange(2, 50),    // totalItems
		gen.IntRange(1, 25),    // pageSize
	))

	properties.Property("last page returns fewer items when necessary", prop.ForAll(
		func(totalItems int, pageSize int) bool {
			// Constrain inputs
			if totalItems <= 0 || totalItems > 100 {
				return true
			}
			if pageSize <= 0 || pageSize > 50 {
				return true
			}

			// Calculate last page
			lastPage := (totalItems + pageSize - 1) / pageSize
			lastPageOffset := (lastPage - 1) * pageSize
			expectedLastPageItems := totalItems - lastPageOffset

			if expectedLastPageItems <= 0 || expectedLastPageItems > pageSize {
				return true // Skip edge cases
			}

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Mock last page query
			rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
			for i := 0; i < expectedLastPageItems; i++ {
				itemID := int64(lastPageOffset + i + 1)
				rows.AddRow(itemID, "Item", "active", int64(0))
			}

			mock.ExpectQuery("SELECT \\* FROM test_entities LIMIT \\$1 OFFSET \\$2").
				WithArgs(pageSize, lastPageOffset).
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Query last page
			results, err := repo.FindAll(context.Background(), QueryOptions{
				Pagination: Pagination{
					Page:     lastPage,
					PageSize: pageSize,
				},
			})
			if err != nil {
				t.Logf("FindAll failed: %v", err)
				return false
			}

			// Verify last page has correct number of items (less than or equal to pageSize)
			if len(results) != expectedLastPageItems {
				t.Logf("Last page: expected %d items, got %d (totalItems=%d, pageSize=%d, lastPage=%d)",
					expectedLastPageItems, len(results), totalItems, pageSize, lastPage)
				return false
			}

			if len(results) > pageSize {
				t.Logf("Last page returned more items (%d) than pageSize (%d)", len(results), pageSize)
				return false
			}

			return true
		},
		gen.IntRange(1, 100),   // totalItems
		gen.IntRange(1, 50),    // pageSize
	))

	properties.Property("empty page beyond data returns zero items", prop.ForAll(
		func(totalItems int, pageSize int) bool {
			// Constrain inputs
			if totalItems < 0 || totalItems > 50 {
				return true
			}
			if pageSize <= 0 || pageSize > 25 {
				return true
			}

			// Calculate a page that's definitely beyond the data
			beyondPage := (totalItems / pageSize) + 5

			db, mock, err := sqlmock.New()
			if err != nil {
				t.Logf("failed to create mock: %v", err)
				return false
			}
			defer db.Close()

			// Mock query for page beyond data - returns empty result
			rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
			offset := (beyondPage - 1) * pageSize

			mock.ExpectQuery("SELECT \\* FROM test_entities LIMIT \\$1 OFFSET \\$2").
				WithArgs(pageSize, offset).
				WillReturnRows(rows)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			// Query page beyond data
			results, err := repo.FindAll(context.Background(), QueryOptions{
				Pagination: Pagination{
					Page:     beyondPage,
					PageSize: pageSize,
				},
			})
			if err != nil {
				t.Logf("FindAll failed: %v", err)
				return false
			}

			// Should return empty slice
			if len(results) != 0 {
				t.Logf("Page beyond data returned %d items, expected 0", len(results))
				return false
			}

			return true
		},
		gen.IntRange(0, 50),    // totalItems
		gen.IntRange(1, 25),    // pageSize
	))

	properties.TestingRun(t)
}
