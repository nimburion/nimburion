package config

import (
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestExtensionValidate_ReturnsAppError(t *testing.T) {
	ext := Extension{RateLimit: Config{Enabled: true, Type: "invalid"}}

	err := ext.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.rate_limit.type.invalid" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}
