package repository

import "fmt"

// Versioned interface for entities that support optimistic locking
type Versioned interface {
	GetVersion() int64
	SetVersion(version int64)
}

// OptimisticLockError is returned when an optimistic lock conflict is detected
type OptimisticLockError struct {
	EntityID string
	Expected int64
	Actual   int64
}

// Error TODO: add description
func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock failed for entity %s: expected version %d, got %d",
		e.EntityID, e.Expected, e.Actual)
}

// NewOptimisticLockError creates a new OptimisticLockError
func NewOptimisticLockError(entityID string, expected, actual int64) *OptimisticLockError {
	return &OptimisticLockError{
		EntityID: entityID,
		Expected: expected,
		Actual:   actual,
	}
}
