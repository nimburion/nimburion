package session

import (
	"errors"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

func TestOAuthTokens_NoSession(t *testing.T) {
	if err := SetOAuthTokens(nil, "a", "r"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expected ErrNoSession from SetOAuthTokens, got %v", err)
	} else {
		var appErr *coreerrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != coreerrors.CodeNotInitialized {
			t.Fatalf("Code = %q", appErr.Code)
		}
	}
	access, refresh, ok := GetOAuthTokens(nil)
	if ok || access != "" || refresh != "" {
		t.Fatalf("expected empty tokens and ok=false, got access=%q refresh=%q ok=%v", access, refresh, ok)
	}
	if err := ClearOAuthTokens(nil); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expected ErrNoSession from ClearOAuthTokens, got %v", err)
	} else {
		var appErr *coreerrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != coreerrors.CodeNotInitialized {
			t.Fatalf("Code = %q", appErr.Code)
		}
	}
}
