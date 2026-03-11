package coordination

import (
	"errors"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

var (
	// ErrValidation classifies input and config validation failures.
	ErrValidation = errors.New("coordination validation error")
	// ErrConflict classifies lease conflicts or token mismatches.
	ErrConflict = errors.New("coordination conflict")
	// ErrRetryable classifies transient backend failures safe to retry.
	ErrRetryable = errors.New("coordination retryable error")
	// ErrInvalidArgument classifies invalid caller/provider arguments.
	ErrInvalidArgument = errors.New("coordination invalid argument")
	// ErrNotInitialized classifies missing provider initialization.
	ErrNotInitialized = errors.New("coordination not initialized")
)

func init() {
	coreerrors.RegisterCanonicalizer(func(err error) (*coreerrors.AppError, bool) {
		switch {
		case errors.Is(err, ErrValidation):
			return coreerrors.NewValidationWithCode("validation.coordination", err.Error(), nil, nil), true
		case errors.Is(err, ErrConflict):
			return coreerrors.New("coordination.conflict", nil, err).WithMessage(err.Error()).WithHTTPStatus(409), true
		case errors.Is(err, ErrRetryable):
			return coreerrors.NewRetryable(err.Error(), err).WithDetails(map[string]interface{}{"family": "coordination"}), true
		case errors.Is(err, ErrInvalidArgument):
			return coreerrors.New("argument.coordination.invalid", nil, err).WithMessage(err.Error()).WithHTTPStatus(400), true
		case errors.Is(err, ErrNotInitialized):
			return coreerrors.NewNotInitialized(err.Error(), err).WithDetails(map[string]interface{}{"family": "coordination"}), true
		default:
			return nil, false
		}
	})
}
