package coordination

import (
	"errors"
	"fmt"
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

func coordinationError(kind error, message string) error {
	if message == "" {
		return kind
	}
	return fmt.Errorf("%w: %s", kind, message)
}
