package jobs

import (
	"errors"
	"fmt"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
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
	switch {
	case errors.Is(kind, ErrValidation):
		return coreerrors.New("validation.jobs", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, ErrConflict):
		return coreerrors.New("jobs.conflict", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(409)
	case errors.Is(kind, ErrNotFound):
		return coreerrors.New("jobs.not_found", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(404)
	case errors.Is(kind, ErrRetryable):
		return coreerrors.NewRetryable(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "jobs"})
	case errors.Is(kind, ErrInvalidArgument):
		return coreerrors.New("argument.jobs.invalid", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, ErrNotInitialized):
		return coreerrors.NewNotInitialized(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "jobs"})
	case errors.Is(kind, ErrClosed):
		return coreerrors.NewClosed(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "jobs"})
	default:
		if message == "" {
			return kind
		}
		return fmt.Errorf("%w: %s", kind, message)
	}
}

func messageOrDefault(message, fallback string) string {
	if message != "" {
		return message
	}
	return fallback
}
