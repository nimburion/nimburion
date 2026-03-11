// Package errorbridge lifts family-level sentinel errors into the canonical core AppError model.
package errorbridge

import (
	stderrors "errors"

	"github.com/nimburion/nimburion/pkg/cache"
	"github.com/nimburion/nimburion/pkg/coordination"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	httpsession "github.com/nimburion/nimburion/pkg/http/session"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/scheduler"
	"github.com/nimburion/nimburion/pkg/session"
)

// Canonicalize lifts package-family sentinels into the canonical AppError contract.
// When err is already an AppError it is returned unchanged.
func Canonicalize(err error) error {
	if err == nil {
		return nil
	}

	var appErr *coreerrors.AppError
	if stderrors.As(err, &appErr) {
		return err
	}

	switch {
	case stderrors.Is(err, jobs.ErrValidation):
		return coreerrors.NewValidationWithCode("validation.jobs", err.Error(), nil, nil)
	case stderrors.Is(err, jobs.ErrConflict):
		return coreerrors.New("jobs.conflict", nil, err).WithMessage(err.Error()).WithHTTPStatus(409)
	case stderrors.Is(err, jobs.ErrNotFound):
		return coreerrors.New("jobs.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404)
	case stderrors.Is(err, jobs.ErrRetryable):
		return coreerrors.NewRetryable(err.Error(), err).WithDetails(map[string]interface{}{"family": "jobs"})
	case stderrors.Is(err, jobs.ErrInvalidArgument):
		return coreerrors.New("argument.jobs.invalid", nil, err).WithMessage(err.Error()).WithHTTPStatus(400)
	case stderrors.Is(err, jobs.ErrNotInitialized):
		return coreerrors.NewNotInitialized(err.Error(), err).WithDetails(map[string]interface{}{"family": "jobs"})
	case stderrors.Is(err, jobs.ErrClosed):
		return coreerrors.NewClosed(err.Error(), err).WithDetails(map[string]interface{}{"family": "jobs"})

	case stderrors.Is(err, scheduler.ErrValidation):
		return coreerrors.NewValidationWithCode("validation.scheduler", err.Error(), nil, nil)
	case stderrors.Is(err, scheduler.ErrConflict):
		return coreerrors.New("scheduler.conflict", nil, err).WithMessage(err.Error()).WithHTTPStatus(409)
	case stderrors.Is(err, scheduler.ErrNotFound):
		return coreerrors.New("scheduler.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404)
	case stderrors.Is(err, scheduler.ErrRetryable):
		return coreerrors.NewRetryable(err.Error(), err).WithDetails(map[string]interface{}{"family": "scheduler"})
	case stderrors.Is(err, scheduler.ErrInvalidArgument):
		return coreerrors.New("argument.scheduler.invalid", nil, err).WithMessage(err.Error()).WithHTTPStatus(400)
	case stderrors.Is(err, scheduler.ErrNotInitialized):
		return coreerrors.NewNotInitialized(err.Error(), err).WithDetails(map[string]interface{}{"family": "scheduler"})
	case stderrors.Is(err, scheduler.ErrClosed):
		return coreerrors.NewClosed(err.Error(), err).WithDetails(map[string]interface{}{"family": "scheduler"})

	case stderrors.Is(err, coordination.ErrValidation):
		return coreerrors.NewValidationWithCode("validation.coordination", err.Error(), nil, nil)
	case stderrors.Is(err, coordination.ErrConflict):
		return coreerrors.New("coordination.conflict", nil, err).WithMessage(err.Error()).WithHTTPStatus(409)
	case stderrors.Is(err, coordination.ErrRetryable):
		return coreerrors.NewRetryable(err.Error(), err).WithDetails(map[string]interface{}{"family": "coordination"})
	case stderrors.Is(err, coordination.ErrInvalidArgument):
		return coreerrors.New("argument.coordination.invalid", nil, err).WithMessage(err.Error()).WithHTTPStatus(400)
	case stderrors.Is(err, coordination.ErrNotInitialized):
		return coreerrors.NewNotInitialized(err.Error(), err).WithDetails(map[string]interface{}{"family": "coordination"})

	case stderrors.Is(err, cache.ErrCacheMiss):
		return coreerrors.New("cache.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404)
	case stderrors.Is(err, session.ErrNotFound):
		return coreerrors.New("session.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404)
	case stderrors.Is(err, httpsession.ErrNoSession):
		return coreerrors.NewNotInitialized(err.Error(), err).WithDetails(map[string]interface{}{"family": "http_session"})
	default:
		return err
	}
}

// AsAppError returns one canonical AppError when the error is already canonical
// or can be recognized from family-level sentinels.
func AsAppError(err error) (*coreerrors.AppError, bool) {
	if err == nil {
		return nil, false
	}
	canonical := Canonicalize(err)
	var appErr *coreerrors.AppError
	if stderrors.As(canonical, &appErr) {
		return appErr, true
	}
	return nil, false
}
