package safepath

import (
	"errors"
	"testing"
)

func TestValidateFilePath(t *testing.T) {
	if err := ValidateFilePath("", ""); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected invalid path, got %v", err)
	}
	if err := ValidateFilePath("../etc/passwd", ""); !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("expected traversal error, got %v", err)
	}
	if err := ValidateFilePath("config/app.yaml", ""); err != nil {
		t.Fatalf("expected valid path, got %v", err)
	}
}
