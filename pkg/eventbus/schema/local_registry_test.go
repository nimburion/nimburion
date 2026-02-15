package schema

import (
	"strings"
	"testing"
)

func TestNewLocalRegistry_ValidationErrors(t *testing.T) {
	if _, err := NewLocalRegistry(""); err == nil {
		t.Fatal("expected error for empty descriptor path")
	}

	_, err := NewLocalRegistry("/definitely/not/found/descriptors.pb")
	if err == nil {
		t.Fatal("expected error for missing descriptor file")
	}
	if !strings.Contains(err.Error(), "read descriptor set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLocalRegistry_ResolveAndValidationGuards(t *testing.T) {
	var nilRegistry *LocalRegistry
	if _, err := nilRegistry.Resolve("subject", "v1", nil); err == nil {
		t.Fatal("expected error on nil registry resolve")
	}
	if err := nilRegistry.ValidatePayload(&Descriptor{}, []byte("x")); err == nil {
		t.Fatal("expected error on nil registry payload validate")
	}

	reg := &LocalRegistry{}
	if _, err := reg.Resolve("", "v1", nil); err == nil {
		t.Fatal("expected validation error for missing subject")
	}
	if err := reg.ValidatePayload(nil, []byte("x")); err == nil {
		t.Fatal("expected error for nil descriptor")
	}
	if err := reg.ValidateHeaders(nil, nil); err == nil {
		t.Fatal("expected error for nil descriptor headers")
	}
}

func TestNormalizeKey(t *testing.T) {
	if got := normalizeKey("  My.Message "); got != "my.message" {
		t.Fatalf("normalizeKey returned %q", got)
	}
}
