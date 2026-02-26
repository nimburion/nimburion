package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// SQLExecutor defines the interface for executing SQL queries
// This can be a *sql.DB, *sql.Tx, or any adapter that provides these methods
type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// GenericCrudRepository provides a generic implementation of CRUD operations for SQL databases
type GenericCrudRepository[T any, ID comparable] struct {
	executor  SQLExecutor
	tableName string
	idColumn  string
	mapper    EntityMapper[T, ID]
}

// EntityMapper defines how to map between entities and database rows
type EntityMapper[T any, ID comparable] interface {
	// ToRow converts an entity to column names and values for INSERT/UPDATE
	ToRow(entity *T) (columns []string, values []interface{}, err error)
	
	// FromRow scans a database row into an entity
	FromRow(rows *sql.Rows) (*T, error)
	
	// GetID extracts the ID from an entity
	GetID(entity *T) ID
	
	// SetID sets the ID on an entity
	SetID(entity *T, id ID)
}

// NewGenericCrudRepository creates a new generic CRUD repository
func NewGenericCrudRepository[T any, ID comparable](
	executor SQLExecutor,
	tableName string,
	idColumn string,
	mapper EntityMapper[T, ID],
) *GenericCrudRepository[T, ID] {
	return &GenericCrudRepository[T, ID]{
		executor:  executor,
		tableName: tableName,
		idColumn:  idColumn,
		mapper:    mapper,
	}
}

// Create inserts a new entity into the database
func (r *GenericCrudRepository[T, ID]) Create(ctx context.Context, entity *T) error {
	if entity == nil {
		return errors.New("entity cannot be nil")
	}

	columns, values, err := r.mapper.ToRow(entity)
	if err != nil {
		return fmt.Errorf("failed to map entity to row: %w", err)
	}

	// Build INSERT query
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		r.tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = r.executor.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	return nil
}

// FindByID retrieves an entity by its ID
func (r *GenericCrudRepository[T, ID]) FindByID(ctx context.Context, id ID) (*T, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", r.tableName, r.idColumn)

	rows, err := r.executor.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query entity: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	entity, err := r.mapper.FromRow(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to scan entity: %w", err)
	}

	return entity, nil
}

// FindAll retrieves entities matching the query options with support for filtering, sorting, and pagination.
// Filters are combined with AND logic. Returns an empty slice if no entities match.
func (r *GenericCrudRepository[T, ID]) FindAll(ctx context.Context, opts QueryOptions) ([]T, error) {
	query := fmt.Sprintf("SELECT * FROM %s", r.tableName)
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if len(opts.Filter) > 0 {
		whereClauses := []string{}
		for field, value := range opts.Filter {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", field, argIndex))
			args = append(args, value)
			argIndex++
		}
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Apply sorting
	if opts.Sort.Field != "" {
		order := "ASC"
		if opts.Sort.Order == SortDesc {
			order = "DESC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", opts.Sort.Field, order)
	}

	// Apply pagination
	if opts.Pagination.PageSize > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
		args = append(args, opts.Pagination.Limit(), opts.Pagination.Offset())
	}

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer rows.Close()

	entities := []T{}
	for rows.Next() {
		entity, err := r.mapper.FromRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity: %w", err)
		}
		entities = append(entities, *entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entities, nil
}

// Count returns the number of entities matching the filter
func (r *GenericCrudRepository[T, ID]) Count(ctx context.Context, filter Filter) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if len(filter) > 0 {
		whereClauses := []string{}
		for field, value := range filter {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", field, argIndex))
			args = append(args, value)
			argIndex++
		}
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	var count int64
	err := r.executor.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entities: %w", err)
	}

	return count, nil
}

// Update updates an existing entity in the database.
// Automatically uses optimistic locking if the entity implements the Versioned interface.
// Returns sql.ErrNoRows if the entity doesn't exist or version mismatch occurs.
func (r *GenericCrudRepository[T, ID]) Update(ctx context.Context, entity *T) error {
	if entity == nil {
		return errors.New("entity cannot be nil")
	}

	id := r.mapper.GetID(entity)
	columns, values, err := r.mapper.ToRow(entity)
	if err != nil {
		return fmt.Errorf("failed to map entity to row: %w", err)
	}

	// Check if entity implements Versioned interface for optimistic locking
	if versioned, ok := any(entity).(Versioned); ok {
		return r.updateWithOptimisticLock(ctx, entity, versioned, id, columns, values)
	}

	// Build UPDATE query
	setClauses := make([]string, len(columns))
	for i, col := range columns {
		setClauses[i] = fmt.Sprintf("%s = $%d", col, i+1)
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		r.tableName,
		strings.Join(setClauses, ", "),
		r.idColumn,
		len(values)+1,
	)

	values = append(values, id)

	result, err := r.executor.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// updateWithOptimisticLock updates an entity with optimistic locking
func (r *GenericCrudRepository[T, ID]) updateWithOptimisticLock(
	ctx context.Context,
	entity *T,
	versioned Versioned,
	id ID,
	columns []string,
	values []interface{},
) error {
	currentVersion := versioned.GetVersion()
	newVersion := currentVersion + 1

	// Build UPDATE query with version check
	setClauses := make([]string, len(columns))
	for i, col := range columns {
		if col == "version" {
			setClauses[i] = fmt.Sprintf("%s = $%d", col, i+1)
			values[i] = newVersion
		} else {
			setClauses[i] = fmt.Sprintf("%s = $%d", col, i+1)
		}
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d AND version = $%d",
		r.tableName,
		strings.Join(setClauses, ", "),
		r.idColumn,
		len(values)+1,
		len(values)+2,
	)

	values = append(values, id, currentVersion)

	result, err := r.executor.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Check if entity exists
		var actualVersion int64
		checkQuery := fmt.Sprintf("SELECT version FROM %s WHERE %s = $1", r.tableName, r.idColumn)
		err := r.executor.QueryRowContext(ctx, checkQuery, id).Scan(&actualVersion)
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		if err != nil {
			return fmt.Errorf("failed to check entity version: %w", err)
		}

		// Version conflict detected
		return NewOptimisticLockError(fmt.Sprintf("%v", id), currentVersion, actualVersion)
	}

	// Update entity version
	versioned.SetVersion(newVersion)

	return nil
}

// Delete removes an entity from the database by its ID
func (r *GenericCrudRepository[T, ID]) Delete(ctx context.Context, id ID) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", r.tableName, r.idColumn)

	result, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ReflectionMapper provides a basic entity mapper using reflection
// This is a simple implementation for demonstration purposes
// Production code should use custom mappers for better performance and control
type ReflectionMapper[T any, ID comparable] struct {
	idField string
}

// NewReflectionMapper creates a new reflection-based entity mapper
func NewReflectionMapper[T any, ID comparable](idField string) *ReflectionMapper[T, ID] {
	return &ReflectionMapper[T, ID]{
		idField: idField,
	}
}

// ToRow converts an entity to column names and values using reflection
func (m *ReflectionMapper[T, ID]) ToRow(entity *T) ([]string, []interface{}, error) {
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()

	columns := []string{}
	values := []interface{}{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		
		// Get column name from db tag or use lowercase field name
		columnName := field.Tag.Get("db")
		if columnName == "" {
			columnName = strings.ToLower(field.Name)
		}
		
		// Skip fields marked with db:"-"
		if columnName == "-" {
			continue
		}

		columns = append(columns, columnName)
		values = append(values, v.Field(i).Interface())
	}

	return columns, values, nil
}

// FromRow scans a database row into an entity using reflection
func (m *ReflectionMapper[T, ID]) FromRow(rows *sql.Rows) (*T, error) {
	entity := new(T)
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()

	// Get column names from the result set
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Create a slice of pointers to scan into
	scanDest := make([]interface{}, len(columns))
	columnMap := make(map[string]int)
	
	for i, col := range columns {
		columnMap[col] = i
		scanDest[i] = new(interface{})
	}

	// Scan the row
	if err := rows.Scan(scanDest...); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	// Map scanned values to struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		
		columnName := field.Tag.Get("db")
		if columnName == "" {
			columnName = strings.ToLower(field.Name)
		}
		
		if columnName == "-" {
			continue
		}

		if colIndex, ok := columnMap[columnName]; ok {
			value := *(scanDest[colIndex].(*interface{}))
			if value != nil {
				v.Field(i).Set(reflect.ValueOf(value).Convert(v.Field(i).Type()))
			}
		}
	}

	return entity, nil
}

// GetID extracts the ID from an entity using reflection
func (m *ReflectionMapper[T, ID]) GetID(entity *T) ID {
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Name == m.idField {
			return v.Field(i).Interface().(ID)
		}
	}

	var zero ID
	return zero
}

// SetID sets the ID on an entity using reflection
func (m *ReflectionMapper[T, ID]) SetID(entity *T, id ID) {
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Name == m.idField {
			v.Field(i).Set(reflect.ValueOf(id))
			return
		}
	}
}
