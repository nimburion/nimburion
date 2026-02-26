package jobs

import (
	"errors"
	"testing"
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
}
