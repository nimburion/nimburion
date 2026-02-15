package repository

import "testing"

func TestNewOptimisticLockError(t *testing.T) {
	err := NewOptimisticLockError("user-42", 3, 4)
	if err.EntityID != "user-42" || err.Expected != 3 || err.Actual != 4 {
		t.Fatalf("unexpected fields: %+v", err)
	}
}

func TestOptimisticLockError_Error(t *testing.T) {
	err := &OptimisticLockError{
		EntityID: "order-7",
		Expected: 2,
		Actual:   5,
	}
	got := err.Error()
	want := "optimistic lock failed for entity order-7: expected version 2, got 5"
	if got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
