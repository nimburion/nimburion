package sse

import (
	"strings"
	"testing"
	"time"
)

func TestEventNormalize_AssignsTimestampAndID(t *testing.T) {
	e := &Event{Channel: "orders"}
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.FixedZone("CET", 3600))

	e.normalize(now)

	if e.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be set")
	}
	if e.Timestamp.Location() != time.UTC {
		t.Fatalf("expected UTC timestamp, got %v", e.Timestamp.Location())
	}
	if e.ID == "" {
		t.Fatal("expected generated id")
	}
	if !strings.Contains(e.ID, "-") {
		t.Fatalf("expected generated id format, got %q", e.ID)
	}
}

func TestEventNormalize_PreservesExistingValues(t *testing.T) {
	ts := time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)
	e := &Event{
		ID:        "existing-id",
		Timestamp: ts,
	}

	e.normalize(time.Now())

	if e.ID != "existing-id" {
		t.Fatalf("expected id unchanged, got %q", e.ID)
	}
	if !e.Timestamp.Equal(ts) {
		t.Fatalf("expected timestamp unchanged, got %v", e.Timestamp)
	}
}
