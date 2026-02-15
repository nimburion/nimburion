package eventbus

import (
	"context"
	"sync"
	"testing"
)

type memoryVersionStore struct {
	mu     sync.Mutex
	values map[string]int64
}

func newMemoryVersionStore() *memoryVersionStore {
	return &memoryVersionStore{values: map[string]int64{}}
}

func (s *memoryVersionStore) GetLastSeenVersion(_ context.Context, tenantID, aggregateID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[tenantID+":"+aggregateID], nil
}

func (s *memoryVersionStore) SetLastSeenVersion(_ context.Context, tenantID, aggregateID string, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[tenantID+":"+aggregateID] = version
	return nil
}

func TestNextAggregateVersion(t *testing.T) {
	next, err := NextAggregateVersion(10)
	if err != nil {
		t.Fatalf("next version failed: %v", err)
	}
	if next != 11 {
		t.Fatalf("expected next=11, got %d", next)
	}
}

func TestValidateAggregateVersion(t *testing.T) {
	tests := []struct {
		name     string
		lastSeen int64
		received int64
		status   VersionStatus
	}{
		{name: "ok", lastSeen: 3, received: 4, status: VersionStatusOK},
		{name: "duplicate", lastSeen: 3, received: 3, status: VersionStatusDuplicate},
		{name: "out-of-order", lastSeen: 3, received: 2, status: VersionStatusOutOfOrder},
		{name: "gap", lastSeen: 3, received: 6, status: VersionStatusGap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := ValidateAggregateVersion(tt.lastSeen, tt.received)
			if check.Status != tt.status {
				t.Fatalf("expected status %s, got %s", tt.status, check.Status)
			}
		})
	}
}

func TestAggregateVersionTracker_CheckAndAdvance(t *testing.T) {
	store := newMemoryVersionStore()
	tracker, err := NewAggregateVersionTracker(store, testLogger{})
	if err != nil {
		t.Fatalf("new tracker failed: %v", err)
	}

	check, err := tracker.CheckAndAdvance(context.Background(), "t1", "a1", 1)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if check.Status != VersionStatusOK {
		t.Fatalf("expected status ok, got %s", check.Status)
	}

	check, err = tracker.CheckAndAdvance(context.Background(), "t1", "a1", 1)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if check.Status != VersionStatusDuplicate {
		t.Fatalf("expected duplicate, got %s", check.Status)
	}

	check, err = tracker.CheckAndAdvance(context.Background(), "t1", "a1", 3)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if check.Status != VersionStatusGap {
		t.Fatalf("expected gap, got %s", check.Status)
	}
}
