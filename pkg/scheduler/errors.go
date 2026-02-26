package scheduler

import (
	"errors"
	"fmt"
)

var (
	// ErrValidation classifies input/config/schedule validation failures.
	ErrValidation = errors.New("scheduler validation error")
	// ErrConflict classifies state conflicts (for example duplicate task, already running).
	ErrConflict = errors.New("scheduler conflict")
	// ErrNotFound classifies missing logical resources.
	ErrNotFound = errors.New("scheduler not found")
	// ErrRetryable classifies transient failures safe to retry.
	ErrRetryable = errors.New("scheduler retryable error")
	// ErrInvalidArgument classifies invalid caller/provider arguments.
	ErrInvalidArgument = errors.New("scheduler invalid argument")
	// ErrNotInitialized classifies missing runtime/provider initialization.
	ErrNotInitialized = errors.New("scheduler not initialized")
	// ErrClosed classifies operations performed on closed components.
	ErrClosed = errors.New("scheduler closed")
)

func schedulerError(kind error, message string) error {
	if message == "" {
		return kind
	}
	return fmt.Errorf("%w: %s", kind, message)
}
