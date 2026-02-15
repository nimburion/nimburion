# Repository Package

This package defines generic repository interfaces for data access operations in the Go microservices framework.

## Overview

The repository package provides a clean abstraction layer between business logic and data storage, following the Repository pattern. It uses Go generics to provide type-safe CRUD operations while maintaining flexibility for different storage backends.

## Interfaces

### Reader[T, ID]

Provides read operations for entities:

```go
type Reader[T any, ID comparable] interface {
    FindByID(ctx context.Context, id ID) (*T, error)
    FindAll(ctx context.Context, opts QueryOptions) ([]T, error)
    Count(ctx context.Context, filter Filter) (int64, error)
}
```

### Writer[T, ID]

Provides write operations for entities:

```go
type Writer[T any, ID comparable] interface {
    Create(ctx context.Context, entity *T) error
    Update(ctx context.Context, entity *T) error
    Delete(ctx context.Context, id ID) error
}
```

### Repository[T, ID]

Combines Reader and Writer for complete CRUD operations:

```go
type Repository[T any, ID comparable] interface {
    Reader[T, ID]
    Writer[T, ID]
}
```

## Query Options

### QueryOptions

Encapsulates filtering, sorting, and pagination:

```go
type QueryOptions struct {
    Filter     Filter
    Sort       Sort
    Pagination Pagination
}
```

### Filter

Field-based filtering criteria:

```go
type Filter map[string]interface{}
```

Example:
```go
filter := Filter{
    "status": "active",
    "age": 25,
}
```

### Sort

Specifies field and direction for sorting:

```go
type Sort struct {
    Field string
    Order SortOrder
}

const (
    SortAsc  SortOrder = "asc"
    SortDesc SortOrder = "desc"
)
```

Example:
```go
sort := Sort{
    Field: "created_at",
    Order: SortDesc,
}
```

### Pagination

Page-based pagination with helper methods:

```go
type Pagination struct {
    Page     int
    PageSize int
}
```

Helper methods:
- `Offset()` - Calculates the offset for database queries
- `Limit()` - Returns the page size

Example:
```go
pagination := Pagination{
    Page:     2,
    PageSize: 20,
}
offset := pagination.Offset() // Returns 20
limit := pagination.Limit()   // Returns 20
```

## Transaction Management

### TransactionManager

Provides transaction management with automatic commit/rollback:

```go
type TransactionManager interface {
    WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Example:
```go
err := txManager.WithTransaction(ctx, func(txCtx context.Context) error {
    // All operations within this function are part of the transaction
    if err := repo.Create(txCtx, entity1); err != nil {
        return err // Transaction will be rolled back
    }
    if err := repo.Create(txCtx, entity2); err != nil {
        return err // Transaction will be rolled back
    }
    return nil // Transaction will be committed
})
```

### UnitOfWork

Provides explicit transaction control:

```go
type UnitOfWork interface {
    Begin(ctx context.Context) (Transaction, error)
}

type Transaction interface {
    Commit() error
    Rollback() error
    Context() context.Context
}
```

## Optimistic Locking

### Versioned Interface

Entities implementing this interface support optimistic locking:

```go
type Versioned interface {
    GetVersion() int64
    SetVersion(version int64)
}
```

Example entity:
```go
type User struct {
    ID      string
    Name    string
    Version int64
}

func (u *User) GetVersion() int64 {
    return u.Version
}

func (u *User) SetVersion(version int64) {
    u.Version = version
}
```

### OptimisticLockError

Returned when a version conflict is detected:

```go
type OptimisticLockError struct {
    EntityID string
    Expected int64
    Actual   int64
}
```

Example handling:
```go
err := repo.Update(ctx, user)
if lockErr, ok := err.(*OptimisticLockError); ok {
    // Handle conflict - typically retry or return conflict error to client
    log.Printf("Version conflict: expected %d, got %d", 
        lockErr.Expected, lockErr.Actual)
}
```

## Design Principles

1. **Small, Focused Interfaces**: Interfaces are kept small and focused on specific concerns (read vs write)
2. **Generic Type Safety**: Uses Go generics for type-safe operations without code duplication
3. **Context Propagation**: All operations accept context for cancellation and timeout support
4. **Domain-Driven**: Interfaces are defined by domain needs, not database capabilities
5. **Adapter Pattern**: Concrete implementations are provided by storage adapters in `pkg/store/`

## Generic CRUD Repository

The framework provides a generic CRUD repository implementation that can be used with any SQL database adapter.

### GenericCrudRepository

A concrete implementation of the Repository interface that works with SQL databases:

```go
type GenericCrudRepository[T any, ID comparable] struct {
    // ...
}

func NewGenericCrudRepository[T any, ID comparable](
    executor SQLExecutor,
    tableName string,
    idColumn string,
    mapper EntityMapper[T, ID],
) *GenericCrudRepository[T, ID]
```

### EntityMapper Interface

To use the generic repository, you need to implement an EntityMapper for your entity:

```go
type EntityMapper[T any, ID comparable] interface {
    ToRow(entity *T) (columns []string, values []interface{}, err error)
    FromRow(rows *sql.Rows) (*T, error)
    GetID(entity *T) ID
    SetID(entity *T, id ID)
}
```

## Document Repository Family

For document databases (MongoDB and DynamoDB), use `pkg/repository/document`.

It provides:
- document-oriented repository contracts (`Reader`/`Writer`/`Repository`)
- backend executors:
  - `MongoDBExecutor` (on top of `pkg/store/mongodb`)
  - `DynamoDBExecutor` (on top of `pkg/store/dynamodb`)

This keeps SQL and document semantics separated and avoids leaky cross-backend abstractions.

### Features

- **CRUD Operations**: Create, FindByID, FindAll, Update, Delete, Count
- **Filtering**: Field-based filtering with map syntax
- **Sorting**: Sort by field with ascending/descending order
- **Pagination**: Page-based pagination with offset/limit
- **Optimistic Locking**: Automatic version checking for entities implementing Versioned interface
- **Transaction Support**: Works with transactions when passed a transaction context

### Example Usage

```go
// Define your entity
type User struct {
    ID      int64  `db:"id"`
    Name    string `db:"name"`
    Email   string `db:"email"`
    Version int64  `db:"version"`
}

func (u *User) GetVersion() int64 { return u.Version }
func (u *User) SetVersion(v int64) { u.Version = v }

// Implement EntityMapper
type UserMapper struct{}

func (m *UserMapper) ToRow(user *User) ([]string, []interface{}, error) {
    return []string{"id", "name", "email", "version"},
        []interface{}{user.ID, user.Name, user.Email, user.Version},
        nil
}

func (m *UserMapper) FromRow(rows *sql.Rows) (*User, error) {
    user := &User{}
    err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Version)
    return user, err
}

func (m *UserMapper) GetID(user *User) int64 { return user.ID }
func (m *UserMapper) SetID(user *User, id int64) { user.ID = id }

// Create repository
userRepo := repository.NewGenericCrudRepository[User, int64](
    db, // or adapter.DB()
    "users",
    "id",
    &UserMapper{},
)

// Use the repository
user := &User{ID: 1, Name: "John", Email: "john@example.com"}
err := userRepo.Create(ctx, user)

// Query with options
opts := repository.QueryOptions{
    Filter: repository.Filter{"name": "John"},
    Sort: repository.Sort{Field: "email", Order: repository.SortAsc},
    Pagination: repository.Pagination{Page: 1, PageSize: 10},
}
users, err := userRepo.FindAll(ctx, opts)
```

## Design Principles

1. **Small, Focused Interfaces**: Interfaces are kept small and focused on specific concerns (read vs write)
2. **Generic Type Safety**: Uses Go generics for type-safe operations without code duplication
3. **Context Propagation**: All operations accept context for cancellation and timeout support
4. **Domain-Driven**: Interfaces are defined by domain needs, not database capabilities
5. **Adapter Pattern**: Concrete implementations are provided by storage adapters in `pkg/store/`

## Usage Example

```go
// Define your entity
type Todo struct {
    ID        string
    Title     string
    Completed bool
    Version   int64
}

func (t *Todo) GetVersion() int64 { return t.Version }
func (t *Todo) SetVersion(v int64) { t.Version = v }

// Use the repository interface
type TodoRepository interface {
    repository.Repository[Todo, string]
    repository.TransactionManager
}

// Query with options
opts := repository.QueryOptions{
    Filter: repository.Filter{
        "completed": false,
    },
    Sort: repository.Sort{
        Field: "created_at",
        Order: repository.SortDesc,
    },
    Pagination: repository.Pagination{
        Page:     1,
        PageSize: 10,
    },
}

todos, err := repo.FindAll(ctx, opts)
```

## Requirements Satisfied

This implementation satisfies the following requirements:

- **23.1**: Repository interfaces defined in domain layer
- **23.2**: Small, focused interfaces
- **23.3**: Concrete implementations in store/adapters package
- **23.4**: All interfaces are mockable
- **23.5**: Interfaces documented with examples
- **29.1**: Generic CRUD interfaces provided
- **29.2**: Pagination support with configurable page size
- **29.3**: Filtering with field-based criteria
- **29.4**: Sorting with field and direction specification
- **29.5**: Transaction management for CRUD operations
- **29.6**: Optimistic locking support via Versioned interface
