package relational

import (
	"database/sql"
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestRelationalCanonicalizer_SQLNoRows(t *testing.T) {
	RegisterCanonicalizers()

	err := errors.Join(errors.New("query failed"), sql.ErrNoRows)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expected joined error to match sql.ErrNoRows")
	}

	appErr, ok := coreerrors.AsAppError(err)
	if !ok {
		t.Fatal("expected AsAppError to return canonical app error")
	}
	if appErr.Code != coreerrors.CodeNotFound {
		t.Fatalf("expected code %q, got %q", coreerrors.CodeNotFound, appErr.Code)
	}
}

func TestRelationalCanonicalizer_OptimisticLock(t *testing.T) {
	RegisterCanonicalizers()

	err := errors.Join(errors.New("update failed"), NewOptimisticLockError("user-1", 3, 5))
	var lockErr *OptimisticLockError
	if !errors.As(err, &lockErr) {
		t.Fatal("expected errors.As to extract OptimisticLockError")
	}

	appErr, ok := coreerrors.AsAppError(err)
	if !ok {
		t.Fatal("expected AsAppError to return canonical app error")
	}
	if appErr.Code != coreerrors.CodeConflict {
		t.Fatalf("expected code %q, got %q", coreerrors.CodeConflict, appErr.Code)
	}
	if appErr.Details["expected"] != int64(3) || appErr.Details["actual"] != int64(5) {
		t.Fatalf("expected version details, got %#v", appErr.Details)
	}
}
