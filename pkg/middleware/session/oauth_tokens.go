package session

import (
	"errors"

	"github.com/nimburion/nimburion/pkg/server/router"
)

const (
	// AccessTokenKey stores the OAuth2 access token in session data.
	AccessTokenKey = "oauth_access_token"
	// RefreshTokenKey stores the OAuth2 refresh token in session data.
	RefreshTokenKey = "oauth_refresh_token"
)

var (
	// ErrNoSession indicates no session is attached to the current request context.
	ErrNoSession = errors.New("session middleware not configured or missing in request context")
)

// SetOAuthTokens stores OAuth2 access/refresh tokens in the server-side session.
func SetOAuthTokens(c router.Context, accessToken, refreshToken string) error {
	s, ok := FromContext(c)
	if !ok || s == nil {
		return ErrNoSession
	}
	if accessToken != "" {
		s.Set(AccessTokenKey, accessToken)
	}
	if refreshToken != "" {
		s.Set(RefreshTokenKey, refreshToken)
	}
	return nil
}

// GetOAuthTokens returns OAuth2 access/refresh tokens from server-side session.
func GetOAuthTokens(c router.Context) (accessToken, refreshToken string, ok bool) {
	s, hasSession := FromContext(c)
	if !hasSession || s == nil {
		return "", "", false
	}
	accessToken, _ = s.Get(AccessTokenKey)
	refreshToken, _ = s.Get(RefreshTokenKey)
	return accessToken, refreshToken, true
}

// ClearOAuthTokens removes OAuth2 tokens from session storage.
func ClearOAuthTokens(c router.Context) error {
	s, ok := FromContext(c)
	if !ok || s == nil {
		return ErrNoSession
	}
	s.Delete(AccessTokenKey)
	s.Delete(RefreshTokenKey)
	return nil
}
