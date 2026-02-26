package jobs

import (
	"errors"
	"fmt"
)

var (
	// ErrValidation classifies input/config/payload validation failures.
	ErrValidation = errors.New("jobs validation error")
	// ErrConflict classifies state conflicts (for example already-running runtime).
	ErrConflict = errors.New("jobs conflict")
	// ErrNotFound classifies missing logical resources (for example missing lease).
	ErrNotFound = errors.New("jobs not found")
	// ErrRetryable classifies transient backend/runtime failures that may succeed on retry.
	ErrRetryable = errors.New("jobs retryable error")
	// ErrInvalidArgument classifies invalid caller arguments.
	ErrInvalidArgument = errors.New("jobs invalid argument")
	// ErrNotInitialized classifies missing runtime/backend initialization.
	ErrNotInitialized = errors.New("jobs not initialized")
	// ErrClosed classifies operations on an already closed runtime/backend.
	ErrClosed = errors.New("jobs closed")
)

func jobsError(kind error, message string) error {
	if message == "" {
		return kind
	}
	return fmt.Errorf("%w: %s", kind, message)
}
