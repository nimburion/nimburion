package repository

import "context"

// Reader provides read operations for entities
type Reader[T any, ID comparable] interface {
	FindByID(ctx context.Context, id ID) (*T, error)
	FindAll(ctx context.Context, opts QueryOptions) ([]T, error)
	Count(ctx context.Context, filter Filter) (int64, error)
}

// Writer provides write operations for entities
type Writer[T any, ID comparable] interface {
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id ID) error
}

// Repository combines Reader and Writer interfaces for complete CRUD operations
type Repository[T any, ID comparable] interface {
	Reader[T, ID]
	Writer[T, ID]
}

// QueryOptions encapsulates filtering, sorting, and pagination options for queries
type QueryOptions struct {
	Filter     Filter
	Sort       Sort
	Pagination Pagination
}

// Filter represents field-based filtering criteria
type Filter map[string]interface{}

// Sort specifies field and direction for sorting results
type Sort struct {
	Field string
	Order SortOrder
}

// SortOrder defines the direction of sorting
// SortOrder defines the sort direction for queries.
type SortOrder string

// Sort order constants
const (
	// SortAsc sorts in ascending order
	SortAsc SortOrder = "asc"
	// SortDesc sorts in descending order
	SortDesc SortOrder = "desc"
)

// Pagination specifies page-based pagination parameters
type Pagination struct {
	Page     int
	PageSize int
}

// Offset calculates the offset for database queries
func (p Pagination) Offset() int {
	if p.Page <= 0 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// Limit returns the page size for database queries
func (p Pagination) Limit() int {
	return p.PageSize
}
