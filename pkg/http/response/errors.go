package response

import (
	"net/http"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// AppError is the shared application error contract consumed by HTTP response mapping.
type AppError = coreerrors.AppError

// NewValidationError creates a new validation error.
func NewValidationError(message string, details map[string]interface{}) *AppError {
	return coreerrors.New("validation.failed", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusBadRequest).
		WithDetails(details)
}

// NewValidationErrorWithCode creates a validation error that can be localized.
func NewValidationErrorWithCode(code, fallbackMessage string, params map[string]interface{}, details map[string]interface{}) *AppError {
	return coreerrors.New(code, coreerrors.Params(params), nil).
		WithMessage(fallbackMessage).
		WithHTTPStatus(inferStatusFromCode(code)).
		WithDetails(details)
}

// NewNotFoundError creates a new not found error.
func NewNotFoundError(message string) *AppError {
	return coreerrors.New("resource.not_found", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusNotFound)
}

// NewConflictError creates a new conflict error.
func NewConflictError(message string, details map[string]interface{}) *AppError {
	return coreerrors.New("resource.conflict", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusConflict).
		WithDetails(details)
}

// NewUnauthorizedError creates a new unauthorized error.
func NewUnauthorizedError(message string) *AppError {
	return coreerrors.New("auth.unauthorized", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusUnauthorized)
}

// NewForbiddenError creates a new forbidden error.
func NewForbiddenError(message string) *AppError {
	return coreerrors.New("auth.forbidden", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusForbidden)
}

// NewInternalError creates a new internal error with optional cause.
func NewInternalError(message string, cause error) *AppError {
	return coreerrors.New("internal.error", nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusInternalServerError)
}
