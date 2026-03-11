package errors

import (
	stderrors "errors"
	"net/http"
	"reflect"
	"testing"
)

func TestAppErrorError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *AppError
		want string
	}{
		{name: "without cause", err: New("validation.failed", nil, nil).WithMessage("validation failed"), want: "validation failed"},
		{name: "with cause", err: New("internal.error", nil, stderrors.New("connection timeout")).WithMessage("database error"), want: "database error: connection timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppErrorUnwrap(t *testing.T) {
	t.Parallel()

	cause := stderrors.New("root cause")
	appErr := New("internal.error", nil, cause)
	if unwrapped := appErr.Unwrap(); !stderrors.Is(unwrapped, cause) {
		t.Fatalf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestMessageClonesParams(t *testing.T) {
	t.Parallel()

	params := Params{"field": "email"}
	msg := NewMessage("validation.failed", params)
	params["field"] = "mutated"

	if msg.Params["field"] != "email" {
		t.Fatalf("Message params = %v, want cloned value", msg.Params)
	}
}

func TestWithHelpers(t *testing.T) {
	t.Parallel()

	details := map[string]interface{}{"field": "email"}
	err := New("validation.failed", Params{"field": "email"}, nil).
		WithMessage("invalid email").
		WithHTTPStatus(http.StatusBadRequest).
		WithDetails(details)

	if err.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusBadRequest)
	}
	if err.FallbackMessage != "invalid email" {
		t.Fatalf("FallbackMessage = %q, want invalid email", err.FallbackMessage)
	}
	if !reflect.DeepEqual(err.Details, details) {
		t.Fatalf("Details = %v, want %v", err.Details, details)
	}
}

func TestCanonicalConstructors(t *testing.T) {
	t.Parallel()

	cause := stderrors.New("boom")
	details := map[string]interface{}{"field": "email"}

	tests := []struct {
		name       string
		err        *AppError
		wantCode   string
		wantStatus int
		wantCause  error
		wantDetail map[string]interface{}
	}{
		{name: "validation", err: NewValidation("invalid", details), wantCode: CodeValidationFailed, wantStatus: http.StatusBadRequest, wantDetail: details},
		{name: "validation with code", err: NewValidationWithCode("validation.email", "invalid", Params{"field": "email"}, details), wantCode: "validation.email", wantStatus: http.StatusBadRequest, wantDetail: details},
		{name: "invalid argument", err: NewInvalidArgument("bad arg", cause), wantCode: CodeInvalidArgument, wantStatus: http.StatusBadRequest, wantCause: cause},
		{name: "not found", err: NewNotFound("missing"), wantCode: CodeNotFound, wantStatus: http.StatusNotFound},
		{name: "conflict", err: NewConflict("conflict", details), wantCode: CodeConflict, wantStatus: http.StatusConflict, wantDetail: details},
		{name: "unauthorized", err: NewUnauthorized("unauthorized"), wantCode: CodeUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "forbidden", err: NewForbidden("forbidden"), wantCode: CodeForbidden, wantStatus: http.StatusForbidden},
		{name: "timeout", err: NewTimeout("timeout", cause), wantCode: CodeTimeout, wantStatus: http.StatusGatewayTimeout, wantCause: cause},
		{name: "unavailable", err: NewUnavailable("down", cause), wantCode: CodeUnavailable, wantStatus: http.StatusServiceUnavailable, wantCause: cause},
		{name: "retryable", err: NewRetryable("retryable", cause), wantCode: CodeRetryable, wantStatus: http.StatusServiceUnavailable, wantCause: cause},
		{name: "closed", err: NewClosed("closed", cause), wantCode: CodeClosed, wantStatus: http.StatusConflict, wantCause: cause},
		{name: "not initialized", err: NewNotInitialized("not initialized", cause), wantCode: CodeNotInitialized, wantStatus: http.StatusServiceUnavailable, wantCause: cause},
		{name: "internal", err: NewInternal("internal", cause), wantCode: CodeInternal, wantStatus: http.StatusInternalServerError, wantCause: cause},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Fatalf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			if tt.err.HTTPStatus != tt.wantStatus {
				t.Fatalf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.wantStatus)
			}
			if tt.wantCause != nil && !stderrors.Is(tt.err, tt.wantCause) {
				t.Fatalf("expected cause %v to be preserved in %v", tt.wantCause, tt.err)
			}
			if tt.wantDetail != nil && !reflect.DeepEqual(tt.err.Details, tt.wantDetail) {
				t.Fatalf("Details = %v, want %v", tt.err.Details, tt.wantDetail)
			}
		})
	}
}
