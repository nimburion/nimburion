package jobs

import (
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestJobValidate_ReturnsTypedValidationError(t *testing.T) {
	job := &Job{}
	err := job.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != "validation.jobs" {
		t.Fatalf("Code = %q", appErr.Code)
	}
}
