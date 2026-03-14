// Package response provides HTTP response and error-mapping helpers.
package response

import (
	"context"
	"net/http"
	"strings"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/http/middleware"
	"github.com/nimburion/nimburion/pkg/http/router"
	frameworki18n "github.com/nimburion/nimburion/pkg/i18n"
)

// SuccessBody represents a successful response with data.
type SuccessBody struct {
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id,omitempty"`
}

// ErrorBody represents the consistent HTTP error response format.
type ErrorBody struct {
	Error     string                 `json:"error"`
	Code      string                 `json:"code,omitempty"`
	Message   string                 `json:"message,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// Success sends a successful JSON response with HTTP 200 OK.
func Success(c router.Context, data interface{}) error {
	requestID := getRequestID(c.Request().Context())
	return c.JSON(http.StatusOK, SuccessBody{
		Data:      data,
		RequestID: requestID,
	})
}

// Created sends a successful JSON response with HTTP 201 Created.
func Created(c router.Context, data interface{}) error {
	requestID := getRequestID(c.Request().Context())
	return c.JSON(http.StatusCreated, SuccessBody{
		Data:      data,
		RequestID: requestID,
	})
}

// NoContent sends a successful response with HTTP 204 No Content.
func NoContent(c router.Context) error {
	c.Response().WriteHeader(http.StatusNoContent)
	return nil
}

// Error sends an error response using the shared application error mapping.
func Error(c router.Context, err error) error {
	statusCode, errorResponse := MapError(c.Request().Context(), err)
	return c.JSON(statusCode, errorResponse)
}

// MapError maps application errors to HTTP responses.
func MapError(ctx context.Context, err error) (int, ErrorBody) {
	requestID := getRequestID(ctx)

	appErr, ok := coreerrors.AsAppError(err)
	if !ok {
		return http.StatusInternalServerError, ErrorBody{
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

	return status, ErrorBody{
		Error:     errorCategory(status, appErr.Code),
		Code:      appErr.Code,
		Message:   message,
		RequestID: requestID,
		Details:   appErr.Details,
	}
}

func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(middleware.RequestIDKey).(string); ok {
		return id
	}
	return ""
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
