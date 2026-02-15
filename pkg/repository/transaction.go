package repository

import "context"

// TransactionManager provides transaction management capabilities
type TransactionManager interface {
	// WithTransaction executes the given function within a transaction
	// If the function returns an error, the transaction is rolled back
	// Otherwise, the transaction is committed
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// UnitOfWork provides the ability to begin transactions
type UnitOfWork interface {
	Begin(ctx context.Context) (Transaction, error)
}

// Transaction represents an active database transaction
type Transaction interface {
	// Commit commits the transaction
	Commit() error
	
	// Rollback rolls back the transaction
	Rollback() error
	
	// Context returns the context associated with this transaction
	Context() context.Context
}
