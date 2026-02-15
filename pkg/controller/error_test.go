package controller

import (
	"context"
	"errors"
	"net/http"
	"testing"

	frameworki18n "github.com/nimburion/nimburion/pkg/i18n"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		want     string
	}{
		{
			name: "error without cause",
			appError: frameworki18n.NewError("validation.failed", nil, nil).
				WithMessage("validation failed"),
			want: "validation failed",
		},
		{
			name: "error with cause",
			appError: frameworki18n.NewError("internal.error", nil, errors.New("connection timeout")).
				WithMessage("database error"),
			want: "database error: connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.appError.Error(); got != tt.want {
				t.Errorf("AppError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	appErr := frameworki18n.NewError("internal.error", nil, cause)

	if unwrapped := appErr.Unwrap(); unwrapped != cause {
		t.Errorf("AppError.Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestMapError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		ctx            context.Context
		wantStatus     int
		wantErrorCode  string
		wantMessage    string
		wantRequestID  string
		wantHasDetails bool
	}{
		{
			name: "i18n app error",
			err: frameworki18n.NewError("validation.email_required", frameworki18n.Params{
				"field": "email",
			}, nil).WithHTTPStatus(http.StatusBadRequest),
			ctx:           context.WithValue(context.Background(), "request_id", "req-i18n"),
			wantStatus:    http.StatusBadRequest,
			wantErrorCode: "validation_error",
			wantMessage:   "validation.email_required",
			wantRequestID: "req-i18n",
		},
		{
			name:           "validation error",
			err:            NewValidationError("invalid input", map[string]interface{}{"field": "email"}),
			ctx:            context.WithValue(context.Background(), "request_id", "req-123"),
			wantStatus:     http.StatusBadRequest,
			wantErrorCode:  "validation_error",
			wantMessage:    "invalid input",
			wantRequestID:  "req-123",
			wantHasDetails: true,
		},
		{
			name:          "not found error",
			err:           NewNotFoundError("user not found"),
			ctx:           context.WithValue(context.Background(), "request_id", "req-456"),
			wantStatus:    http.StatusNotFound,
			wantErrorCode: "not_found",
			wantMessage:   "user not found",
			wantRequestID: "req-456",
		},
		{
			name:           "conflict error",
			err:            NewConflictError("resource already exists", map[string]interface{}{"id": "123"}),
			ctx:            context.WithValue(context.Background(), "request_id", "req-789"),
			wantStatus:     http.StatusConflict,
			wantErrorCode:  "conflict",
			wantMessage:    "resource already exists",
			wantRequestID:  "req-789",
			wantHasDetails: true,
		},
		{
			name:          "unauthorized error",
			err:           NewUnauthorizedError("invalid credentials"),
			ctx:           context.WithValue(context.Background(), "request_id", "req-abc"),
			wantStatus:    http.StatusUnauthorized,
			wantErrorCode: "unauthorized",
			wantMessage:   "invalid credentials",
			wantRequestID: "req-abc",
		},
		{
			name:          "forbidden error",
			err:           NewForbiddenError("insufficient permissions"),
			ctx:           context.WithValue(context.Background(), "request_id", "req-def"),
			wantStatus:    http.StatusForbidden,
			wantErrorCode: "forbidden",
			wantMessage:   "insufficient permissions",
			wantRequestID: "req-def",
		},
		{
			name:          "internal error",
			err:           NewInternalError("database connection failed", nil),
			ctx:           context.WithValue(context.Background(), "request_id", "req-ghi"),
			wantStatus:    http.StatusInternalServerError,
			wantErrorCode: "internal_server_error",
			wantMessage:   "database connection failed",
			wantRequestID: "req-ghi",
		},
		{
			name:          "unknown error type",
			err:           errors.New("some random error"),
			ctx:           context.WithValue(context.Background(), "request_id", "req-jkl"),
			wantStatus:    http.StatusInternalServerError,
			wantErrorCode: "internal_server_error",
			wantMessage:   "an unexpected error occurred",
			wantRequestID: "req-jkl",
		},
		{
			name:          "context without request ID",
			err:           NewValidationError("validation failed", nil),
			ctx:           context.Background(),
			wantStatus:    http.StatusBadRequest,
			wantErrorCode: "validation_error",
			wantMessage:   "validation failed",
			wantRequestID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errResp := MapError(tt.ctx, tt.err)

			if status != tt.wantStatus {
				t.Errorf("MapError() status = %v, want %v", status, tt.wantStatus)
			}

			if errResp.Error != tt.wantErrorCode {
				t.Errorf("MapError() error code = %v, want %v", errResp.Error, tt.wantErrorCode)
			}

			if errResp.Message != tt.wantMessage {
				t.Errorf("MapError() message = %v, want %v", errResp.Message, tt.wantMessage)
			}

			if errResp.RequestID != tt.wantRequestID {
				t.Errorf("MapError() request ID = %v, want %v", errResp.RequestID, tt.wantRequestID)
			}

			if tt.wantHasDetails && errResp.Details == nil {
				t.Errorf("MapError() expected details but got nil")
			}
		})
	}
}

func TestNewValidationError(t *testing.T) {
	details := map[string]interface{}{"field": "email"}
	err := NewValidationError("invalid email", details)

	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("NewValidationError() status = %v, want %v", err.HTTPStatus, http.StatusBadRequest)
	}
	if err.FallbackMessage != "invalid email" {
		t.Errorf("NewValidationError() message = %v, want %v", err.FallbackMessage, "invalid email")
	}
	if err.Details == nil {
		t.Error("NewValidationError() details is nil")
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("resource not found")

	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("NewNotFoundError() status = %v, want %v", err.HTTPStatus, http.StatusNotFound)
	}
	if err.FallbackMessage != "resource not found" {
		t.Errorf("NewNotFoundError() message = %v, want %v", err.FallbackMessage, "resource not found")
	}
}

func TestNewConflictError(t *testing.T) {
	details := map[string]interface{}{"id": "123"}
	err := NewConflictError("resource exists", details)

	if err.HTTPStatus != http.StatusConflict {
		t.Errorf("NewConflictError() status = %v, want %v", err.HTTPStatus, http.StatusConflict)
	}
	if err.FallbackMessage != "resource exists" {
		t.Errorf("NewConflictError() message = %v, want %v", err.FallbackMessage, "resource exists")
	}
	if err.Details == nil {
		t.Error("NewConflictError() details is nil")
	}
}

func TestNewUnauthorizedError(t *testing.T) {
	err := NewUnauthorizedError("invalid token")

	if err.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("NewUnauthorizedError() status = %v, want %v", err.HTTPStatus, http.StatusUnauthorized)
	}
	if err.FallbackMessage != "invalid token" {
		t.Errorf("NewUnauthorizedError() message = %v, want %v", err.FallbackMessage, "invalid token")
	}
}

func TestNewForbiddenError(t *testing.T) {
	err := NewForbiddenError("access denied")

	if err.HTTPStatus != http.StatusForbidden {
		t.Errorf("NewForbiddenError() status = %v, want %v", err.HTTPStatus, http.StatusForbidden)
	}
	if err.FallbackMessage != "access denied" {
		t.Errorf("NewForbiddenError() message = %v, want %v", err.FallbackMessage, "access denied")
	}
}

func TestNewInternalError(t *testing.T) {
	cause := errors.New("connection failed")
	err := NewInternalError("database error", cause)

	if err.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("NewInternalError() status = %v, want %v", err.HTTPStatus, http.StatusInternalServerError)
	}
	if err.FallbackMessage != "database error" {
		t.Errorf("NewInternalError() message = %v, want %v", err.FallbackMessage, "database error")
	}
	if err.Cause != cause {
		t.Errorf("NewInternalError() cause = %v, want %v", err.Cause, cause)
	}
}

func TestErrorMapping_DomainVsInfrastructure(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantStatus int
		category   string
	}{
		{
			name:       "domain error - validation",
			err:        NewValidationError("invalid", nil),
			wantStatus: http.StatusBadRequest,
			category:   "4xx",
		},
		{
			name:       "domain error - not found",
			err:        NewNotFoundError("not found"),
			wantStatus: http.StatusNotFound,
			category:   "4xx",
		},
		{
			name:       "domain error - conflict",
			err:        NewConflictError("conflict", nil),
			wantStatus: http.StatusConflict,
			category:   "4xx",
		},
		{
			name:       "infrastructure error - internal",
			err:        NewInternalError("db error", nil),
			wantStatus: http.StatusInternalServerError,
			category:   "5xx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := MapError(context.Background(), tt.err)

			if status != tt.wantStatus {
				t.Errorf("MapError() status = %v, want %v", status, tt.wantStatus)
			}

			if tt.category == "4xx" && (status < 400 || status >= 500) {
				t.Errorf("Domain error should map to 4xx, got %v", status)
			}
			if tt.category == "5xx" && (status < 500 || status >= 600) {
				t.Errorf("Infrastructure error should map to 5xx, got %v", status)
			}
		})
	}
}
