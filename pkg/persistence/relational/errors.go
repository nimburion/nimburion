package relational

import (
	"database/sql"
	"errors"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// RegisterCanonicalizers registers relational error canonicalizers in the shared app error registry.
func RegisterCanonicalizers() {
	coreerrors.RegisterCanonicalizer(func(err error) (*coreerrors.AppError, bool) {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return coreerrors.NewNotFound(err.Error()), true
		default:
			var lockErr *OptimisticLockError
			if errors.As(err, &lockErr) {
				return coreerrors.NewConflict(lockErr.Error(), map[string]interface{}{
					"entity_id": lockErr.EntityID,
					"expected":  lockErr.Expected,
					"actual":    lockErr.Actual,
				}), true
			}
			return nil, false
		}
	})
}
