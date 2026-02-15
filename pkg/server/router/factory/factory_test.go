package factory

import (
	"strings"
	"testing"
)

func TestNewRouter_ValidTypes(t *testing.T) {
	types := []string{"nethttp", "gin", "gorilla", "", " NETHTTP "}
	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			r, err := NewRouter(typ)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r == nil {
				t.Fatal("expected non-nil router")
			}
		})
	}
}

func TestNewRouter_InvalidType(t *testing.T) {
	_, err := NewRouter("chi")
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	msg := err.Error()
	for _, typ := range []string{"nethttp", "gin", "gorilla"} {
		if !strings.Contains(msg, typ) {
			t.Fatalf("expected error to include %q, got %q", typ, msg)
		}
	}
}
