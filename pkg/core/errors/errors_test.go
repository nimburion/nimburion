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
	if unwrapped := appErr.Unwrap(); unwrapped != cause {
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
