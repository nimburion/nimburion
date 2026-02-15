package schema

import (
	"errors"
	"testing"
)

func TestValidationError_ErrorAndUnwrap(t *testing.T) {
	base := errors.New("root")
	err := &ValidationError{
		Code:    "validation.kafka.payload_invalid",
		Subject: "orders.created",
		Version: "v1",
		Cause:   base,
	}

	if got := err.Error(); got != "validation.kafka.payload_invalid: root" {
		t.Fatalf("unexpected error string: %q", got)
	}
	if !errors.Is(err, base) {
		t.Fatal("expected unwrap to expose cause")
	}

	noCause := &ValidationError{Code: "validation.kafka.schema_not_found"}
	if got := noCause.Error(); got != "validation.kafka.schema_not_found" {
		t.Fatalf("unexpected error string without cause: %q", got)
	}
}
