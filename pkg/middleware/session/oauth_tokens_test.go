package session

import "testing"

func TestOAuthTokens_NoSession(t *testing.T) {
	if err := SetOAuthTokens(nil, "a", "r"); err != ErrNoSession {
		t.Fatalf("expected ErrNoSession from SetOAuthTokens, got %v", err)
	}
	access, refresh, ok := GetOAuthTokens(nil)
	if ok || access != "" || refresh != "" {
		t.Fatalf("expected empty tokens and ok=false, got access=%q refresh=%q ok=%v", access, refresh, ok)
	}
	if err := ClearOAuthTokens(nil); err != ErrNoSession {
		t.Fatalf("expected ErrNoSession from ClearOAuthTokens, got %v", err)
	}
}
