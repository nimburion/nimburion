package scheduler

import (
	"errors"
	"fmt"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
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
	switch {
	case errors.Is(kind, ErrValidation):
		return coreerrors.New("validation.scheduler", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, ErrConflict):
		return coreerrors.New("scheduler.conflict", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(409)
	case errors.Is(kind, ErrNotFound):
		return coreerrors.New("scheduler.not_found", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(404)
	case errors.Is(kind, ErrRetryable):
		return coreerrors.NewRetryable(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "scheduler"})
	case errors.Is(kind, ErrInvalidArgument):
		return coreerrors.New("argument.scheduler.invalid", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, ErrNotInitialized):
		return coreerrors.NewNotInitialized(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "scheduler"})
	case errors.Is(kind, ErrClosed):
		return coreerrors.NewClosed(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "scheduler"})
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
