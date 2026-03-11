package response

import (
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// AppError is the shared application error contract consumed by HTTP response mapping.
type AppError = coreerrors.AppError

// NewValidationError creates a new validation error.
func NewValidationError(message string, details map[string]interface{}) *AppError {
	return coreerrors.NewValidation(message, details)
}

// NewValidationErrorWithCode creates a validation error that can be localized.
func NewValidationErrorWithCode(code, fallbackMessage string, params, details map[string]interface{}) *AppError {
	return coreerrors.NewValidationWithCode(code, fallbackMessage, coreerrors.Params(params), details).
		WithHTTPStatus(inferStatusFromCode(code))
}

// NewNotFoundError creates a new not found error.
func NewNotFoundError(message string) *AppError {
	return coreerrors.NewNotFound(message)
}

// NewConflictError creates a new conflict error.
func NewConflictError(message string, details map[string]interface{}) *AppError {
	return coreerrors.NewConflict(message, details)
}

// NewUnauthorizedError creates a new unauthorized error.
func NewUnauthorizedError(message string) *AppError {
	return coreerrors.NewUnauthorized(message)
}

// NewForbiddenError creates a new forbidden error.
func NewForbiddenError(message string) *AppError {
	return coreerrors.NewForbidden(message)
}

// NewInternalError creates a new internal error with optional cause.
func NewInternalError(message string, cause error) *AppError {
	return coreerrors.NewInternal(message, cause)
}
