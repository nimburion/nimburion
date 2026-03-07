package session

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryStore_SaveLoadCloneAndExpiry(t *testing.T) {
	s := NewInMemoryStore()
	ctx := context.Background()

	data := map[string]string{"k": "v"}
	if err := s.Save(ctx, "sid", data, 50*time.Millisecond); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Ensure clone on save/load.
	data["k"] = "changed"
	got, err := s.Load(ctx, "sid")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got["k"] != "v" {
		t.Fatalf("expected original value, got %q", got["k"])
	}
	got["k"] = "mutated"
	got2, err := s.Load(ctx, "sid")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	if got2["k"] != "v" {
		t.Fatalf("expected isolated value, got %q", got2["k"])
	}

	time.Sleep(70 * time.Millisecond)
	if _, err := s.Load(ctx, "sid"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after expiry, got %v", err)
	}
}

func TestInMemoryStore_TouchAndDelete(t *testing.T) {
	s := NewInMemoryStore()
	ctx := context.Background()

	if err := s.Touch(ctx, "missing", time.Second); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound on missing touch, got %v", err)
	}
	if err := s.Save(ctx, "sid", map[string]string{"k": "v"}, 30*time.Millisecond); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.Touch(ctx, "sid", 120*time.Millisecond); err != nil {
		t.Fatalf("touch: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := s.Load(ctx, "sid"); err != nil {
		t.Fatalf("expected session still alive after touch, got %v", err)
	}

	if err := s.Delete(ctx, "sid"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Load(ctx, "sid"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
