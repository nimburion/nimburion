package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestEntity is a test entity for CRUD operations
type TestEntity struct {
	ID      int64  `db:"id"`
	Name    string `db:"name"`
	Status  string `db:"status"`
	Version int64  `db:"version"`
}

func (e *TestEntity) GetVersion() int64 {
	return e.Version
}

func (e *TestEntity) SetVersion(version int64) {
	e.Version = version
}

// TestEntityMapper is a custom mapper for TestEntity
type TestEntityMapper struct{}

func (m *TestEntityMapper) ToRow(entity *TestEntity) ([]string, []interface{}, error) {
	return []string{"id", "name", "status", "version"},
		[]interface{}{entity.ID, entity.Name, entity.Status, entity.Version},
		nil
}

func (m *TestEntityMapper) FromRow(rows *sql.Rows) (*TestEntity, error) {
	entity := &TestEntity{}
	err := rows.Scan(&entity.ID, &entity.Name, &entity.Status, &entity.Version)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (m *TestEntityMapper) GetID(entity *TestEntity) int64 {
	return entity.ID
}

func (m *TestEntityMapper) SetID(entity *TestEntity, id int64) {
	entity.ID = id
}

func TestGenericCrudRepository_Create(t *testing.T) {
	tests := []struct {
		name    string
		entity  *TestEntity
		wantErr bool
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name: "successful create",
			entity: &TestEntity{
				ID:      1,
				Name:    "Test",
				Status:  "active",
				Version: 0,
			},
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO test_entities").
					WithArgs(int64(1), "Test", "active", int64(0)).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "nil entity",
			entity:  nil,
			wantErr: true,
			setup:   func(mock sqlmock.Sqlmock) {},
		},
		{
			name: "database error",
			entity: &TestEntity{
				ID:      1,
				Name:    "Test",
				Status:  "active",
				Version: 0,
			},
			wantErr: true,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO test_entities").
					WithArgs(int64(1), "Test", "active", int64(0)).
					WillReturnError(errors.New("database error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			err = repo.Create(context.Background(), tt.entity)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestGenericCrudRepository_FindByID(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		want    *TestEntity
		wantErr bool
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name: "successful find",
			id:   1,
			want: &TestEntity{
				ID:      1,
				Name:    "Test",
				Status:  "active",
				Version: 0,
			},
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "status", "version"}).
					AddRow(int64(1), "Test", "active", int64(0))
				mock.ExpectQuery("SELECT \\* FROM test_entities WHERE id = \\$1").
					WithArgs(int64(1)).
					WillReturnRows(rows)
			},
		},
		{
			name:    "entity not found",
			id:      999,
			want:    nil,
			wantErr: true,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "status", "version"})
				mock.ExpectQuery("SELECT \\* FROM test_entities WHERE id = \\$1").
					WithArgs(int64(999)).
					WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			got, err := repo.FindByID(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != nil && tt.want != nil {
				if got.ID != tt.want.ID || got.Name != tt.want.Name {
					t.Errorf("FindByID() = %v, want %v", got, tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestGenericCrudRepository_FindAll(t *testing.T) {
	tests := []struct {
		name    string
		opts    QueryOptions
		want    int
		wantErr bool
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name: "find all without filters",
			opts: QueryOptions{},
			want: 2,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "status", "version"}).
					AddRow(int64(1), "Test1", "active", int64(0)).
					AddRow(int64(2), "Test2", "inactive", int64(0))
				mock.ExpectQuery("SELECT \\* FROM test_entities").
					WillReturnRows(rows)
			},
		},
		{
			name: "find all with filter",
			opts: QueryOptions{
				Filter: Filter{"status": "active"},
			},
			want: 1,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "status", "version"}).
					AddRow(int64(1), "Test1", "active", int64(0))
				mock.ExpectQuery("SELECT \\* FROM test_entities WHERE status = \\$1").
					WithArgs("active").
					WillReturnRows(rows)
			},
		},
		{
			name: "find all with sorting",
			opts: QueryOptions{
				Sort: Sort{Field: "name", Order: SortAsc},
			},
			want: 2,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "status", "version"}).
					AddRow(int64(1), "Test1", "active", int64(0)).
					AddRow(int64(2), "Test2", "inactive", int64(0))
				mock.ExpectQuery("SELECT \\* FROM test_entities ORDER BY name ASC").
					WillReturnRows(rows)
			},
		},
		{
			name: "find all with pagination",
			opts: QueryOptions{
				Pagination: Pagination{Page: 1, PageSize: 10},
			},
			want: 2,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "status", "version"}).
					AddRow(int64(1), "Test1", "active", int64(0)).
					AddRow(int64(2), "Test2", "inactive", int64(0))
				mock.ExpectQuery("SELECT \\* FROM test_entities LIMIT \\$1 OFFSET \\$2").
					WithArgs(10, 0).
					WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			got, err := repo.FindAll(context.Background(), tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("FindAll() returned %d entities, want %d", len(got), tt.want)
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// TestEntityNoVersion is a test entity without version for testing regular updates
type TestEntityNoVersion struct {
	ID     int64  `db:"id"`
	Name   string `db:"name"`
	Status string `db:"status"`
}

// TestEntityNoVersionMapper is a custom mapper for TestEntityNoVersion
type TestEntityNoVersionMapper struct{}

func (m *TestEntityNoVersionMapper) ToRow(entity *TestEntityNoVersion) ([]string, []interface{}, error) {
	return []string{"id", "name", "status"},
		[]interface{}{entity.ID, entity.Name, entity.Status},
		nil
}

func (m *TestEntityNoVersionMapper) FromRow(rows *sql.Rows) (*TestEntityNoVersion, error) {
	entity := &TestEntityNoVersion{}
	err := rows.Scan(&entity.ID, &entity.Name, &entity.Status)
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (m *TestEntityNoVersionMapper) GetID(entity *TestEntityNoVersion) int64 {
	return entity.ID
}

func (m *TestEntityNoVersionMapper) SetID(entity *TestEntityNoVersion, id int64) {
	entity.ID = id
}

func TestGenericCrudRepository_Update(t *testing.T) {
	tests := []struct {
		name    string
		entity  *TestEntityNoVersion
		wantErr bool
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name: "successful update",
			entity: &TestEntityNoVersion{
				ID:     1,
				Name:   "Updated",
				Status: "active",
			},
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE test_entities SET").
					WithArgs(int64(1), "Updated", "active", int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:    "nil entity",
			entity:  nil,
			wantErr: true,
			setup:   func(mock sqlmock.Sqlmock) {},
		},
		{
			name: "entity not found",
			entity: &TestEntityNoVersion{
				ID:     999,
				Name:   "Updated",
				Status: "active",
			},
			wantErr: true,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE test_entities SET").
					WithArgs(int64(999), "Updated", "active", int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntityNoVersion, int64](
				db,
				"test_entities",
				"id",
				&TestEntityNoVersionMapper{},
			)

			err = repo.Update(context.Background(), tt.entity)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestGenericCrudRepository_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		wantErr bool
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name:    "successful delete",
			id:      1,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM test_entities WHERE id = \\$1").
					WithArgs(int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:    "entity not found",
			id:      999,
			wantErr: true,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM test_entities WHERE id = \\$1").
					WithArgs(int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			err = repo.Delete(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestGenericCrudRepository_Count(t *testing.T) {
	tests := []struct {
		name    string
		filter  Filter
		want    int64
		wantErr bool
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name:    "count all",
			filter:  Filter{},
			want:    5,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(5))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM test_entities").
					WillReturnRows(rows)
			},
		},
		{
			name:    "count with filter",
			filter:  Filter{"status": "active"},
			want:    3,
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(int64(3))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM test_entities WHERE status = \\$1").
					WithArgs("active").
					WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			got, err := repo.Count(context.Background(), tt.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Count() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("Count() = %d, want %d", got, tt.want)
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestGenericCrudRepository_UpdateWithOptimisticLock(t *testing.T) {
	tests := []struct {
		name    string
		entity  *TestEntity
		wantErr bool
		errType error
		setup   func(mock sqlmock.Sqlmock)
	}{
		{
			name: "successful update with version increment",
			entity: &TestEntity{
				ID:      1,
				Name:    "Updated",
				Status:  "active",
				Version: 1,
			},
			wantErr: false,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE test_entities SET .* WHERE id = \\$5 AND version = \\$6").
					WithArgs(int64(1), "Updated", "active", int64(2), int64(1), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "optimistic lock conflict",
			entity: &TestEntity{
				ID:      1,
				Name:    "Updated",
				Status:  "active",
				Version: 1,
			},
			wantErr: true,
			errType: &OptimisticLockError{},
			setup: func(mock sqlmock.Sqlmock) {
				// Update fails due to version mismatch
				mock.ExpectExec("UPDATE test_entities SET .* WHERE id = \\$5 AND version = \\$6").
					WithArgs(int64(1), "Updated", "active", int64(2), int64(1), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 0))
				
				// Check query to get actual version
				rows := sqlmock.NewRows([]string{"version"}).AddRow(int64(2))
				mock.ExpectQuery("SELECT version FROM test_entities WHERE id = \\$1").
					WithArgs(int64(1)).
					WillReturnRows(rows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.setup(mock)

			repo := NewGenericCrudRepository[TestEntity, int64](
				db,
				"test_entities",
				"id",
				&TestEntityMapper{},
			)

			err = repo.Update(context.Background(), tt.entity)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errType != nil {
				var lockErr *OptimisticLockError
				if !errors.As(err, &lockErr) {
					t.Errorf("Update() error type = %T, want %T", err, tt.errType)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil && !tt.wantErr {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
