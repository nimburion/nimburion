package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	frameworki18n "github.com/nimburion/nimburion/pkg/i18n"
)

// AppError is the single application error contract shared across layers.
type AppError = frameworki18n.AppError

// ErrorResponse represents the consistent error response format.
type ErrorResponse struct {
	Error     string                 `json:"error"`
	Code      string                 `json:"code,omitempty"`
	Message   string                 `json:"message,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// MapError maps application errors to HTTP responses.
func MapError(ctx context.Context, err error) (int, ErrorResponse) {
	requestID := getRequestID(ctx)

	var appErr *frameworki18n.AppError
	if !errors.As(err, &appErr) {
		return http.StatusInternalServerError, ErrorResponse{
			Error:     "internal_server_error",
			Message:   "an unexpected error occurred",
			RequestID: requestID,
		}
	}

	status := appErr.HTTPStatus
	if status == 0 {
		status = inferStatusFromCode(appErr.Code)
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}

	message := translateMessageWithFallback(
		ctx,
		appErr.Code,
		map[string]interface{}(appErr.Params),
		appErr.FallbackMessage,
	)
	if message == "" {
		message = "an unexpected error occurred"
	}

	return status, ErrorResponse{
		Error:     errorCategory(status, appErr.Code),
		Code:      appErr.Code,
		Message:   message,
		RequestID: requestID,
		Details:   appErr.Details,
	}
}

// getRequestID extracts the request ID from context.
func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return ""
}

// NewValidationError creates a new validation error.
func NewValidationError(message string, details map[string]interface{}) *AppError {
	return frameworki18n.NewError("validation.failed", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusBadRequest).
		WithDetails(details)
}

// NewValidationErrorWithCode creates a validation error that can be localized.
func NewValidationErrorWithCode(code, fallbackMessage string, params map[string]interface{}, details map[string]interface{}) *AppError {
	return frameworki18n.NewError(code, frameworki18n.Params(params), nil).
		WithMessage(fallbackMessage).
		WithHTTPStatus(inferStatusFromCode(code)).
		WithDetails(details)
}

// NewNotFoundError creates a new not found error.
func NewNotFoundError(message string) *AppError {
	return frameworki18n.NewError("resource.not_found", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusNotFound)
}

// NewConflictError creates a new conflict error.
func NewConflictError(message string, details map[string]interface{}) *AppError {
	return frameworki18n.NewError("resource.conflict", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusConflict).
		WithDetails(details)
}

// NewUnauthorizedError creates a new unauthorized error.
func NewUnauthorizedError(message string) *AppError {
	return frameworki18n.NewError("auth.unauthorized", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusUnauthorized)
}

// NewForbiddenError creates a new forbidden error.
func NewForbiddenError(message string) *AppError {
	return frameworki18n.NewError("auth.forbidden", nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusForbidden)
}

// NewInternalError creates a new internal error with optional cause.
func NewInternalError(message string, cause error) *AppError {
	return frameworki18n.NewError("internal.error", nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusInternalServerError)
}

func translateMessage(ctx context.Context, code string, params map[string]interface{}) string {
	if code == "" {
		return ""
	}
	translator := frameworki18n.TranslatorFromContext(ctx)
	return translator.T(code, params)
}

func translateMessageWithFallback(ctx context.Context, code string, params map[string]interface{}, fallback string) string {
	if code == "" {
		return fallback
	}
	translated := translateMessage(ctx, code, params)
	if translated == "" {
		return fallback
	}
	if translated == code && fallback != "" {
		return fallback
	}
	return translated
}

func errorCategory(status int, code string) string {
	lowerCode := strings.ToLower(strings.TrimSpace(code))
	if strings.HasPrefix(lowerCode, "validation.") {
		return "validation_error"
	}

	switch status {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		if status >= 500 {
			return "internal_server_error"
		}
		return "application_error"
	}
}

func inferStatusFromCode(code string) int {
	lowerCode := strings.ToLower(strings.TrimSpace(code))
	switch {
	case strings.HasPrefix(lowerCode, "validation."):
		return http.StatusBadRequest
	case strings.Contains(lowerCode, "unauthorized"):
		return http.StatusUnauthorized
	case strings.Contains(lowerCode, "forbidden"):
		return http.StatusForbidden
	case strings.Contains(lowerCode, "not_found"):
		return http.StatusNotFound
	case strings.Contains(lowerCode, "conflict"):
		return http.StatusConflict
	case strings.Contains(lowerCode, "internal"):
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
