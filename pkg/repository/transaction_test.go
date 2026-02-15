package repository

import (
	"context"
	"testing"
)

type fakeTransaction struct {
	ctx        context.Context
	committed  bool
	rolledback bool
}

func (t *fakeTransaction) Commit() error {
	t.committed = true
	return nil
}

func (t *fakeTransaction) Rollback() error {
	t.rolledback = true
	return nil
}

func (t *fakeTransaction) Context() context.Context {
	return t.ctx
}

func TestTransactionContract(t *testing.T) {
	base := context.WithValue(context.Background(), "k", "v")
	tx := &fakeTransaction{ctx: base}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !tx.committed {
		t.Fatal("expected committed flag")
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !tx.rolledback {
		t.Fatal("expected rolledback flag")
	}

	if got := tx.Context().Value("k"); got != "v" {
		t.Fatalf("unexpected context value: %v", got)
	}
}
