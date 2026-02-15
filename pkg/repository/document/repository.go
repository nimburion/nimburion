package document

import "context"

// Filter represents field-based filtering criteria for document stores.
type Filter map[string]interface{}

// Sort specifies field and direction for sorting results.
type Sort struct {
	Field string
	Order SortOrder
}

// SortOrder defines the direction of sorting.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// Pagination specifies limit-based pagination for document stores.
// Cursor is backend-specific (e.g. Mongo cursor token, Dynamo LastEvaluatedKey encoding).
type Pagination struct {
	Limit  int
	Cursor string
}

// QueryOptions encapsulates filtering, sorting, and pagination options for document queries.
type QueryOptions struct {
	Filter     Filter
	Sort       Sort
	Pagination Pagination
}

// Reader provides read operations for document entities.
type Reader[T any, ID comparable] interface {
	FindByID(ctx context.Context, id ID) (*T, error)
	FindAll(ctx context.Context, opts QueryOptions) ([]T, error)
	Count(ctx context.Context, filter Filter) (int64, error)
}

// Writer provides write operations for document entities.
type Writer[T any, ID comparable] interface {
	Create(ctx context.Context, entity *T) error
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id ID) error
}

// Repository combines Reader and Writer interfaces for document stores.
type Repository[T any, ID comparable] interface {
	Reader[T, ID]
	Writer[T, ID]
}
