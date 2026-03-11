package session

import (
	"errors"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/http/router"
)

const (
	// AccessTokenKey stores the OAuth2 access token in session data.
	AccessTokenKey = "oauth_access_token" // #nosec G101 -- session storage key name, not a credential
	// RefreshTokenKey stores the OAuth2 refresh token in session data.
	RefreshTokenKey = "oauth_refresh_token" // #nosec G101 -- session storage key name, not a credential
)

// ErrNoSession indicates no session is attached to the current request context.
var ErrNoSession = errors.New("session middleware not configured or missing in request context")

func init() {
	coreerrors.RegisterCanonicalizer(func(err error) (*coreerrors.AppError, bool) {
		if errors.Is(err, ErrNoSession) {
			return coreerrors.NewNotInitialized(err.Error(), err).
				WithDetails(map[string]interface{}{"family": "http_session"}), true
		}
		return nil, false
	})
}

// SetOAuthTokens stores OAuth2 access/refresh tokens in the server-side session.
func SetOAuthTokens(c router.Context, accessToken, refreshToken string) error {
	s, ok := FromContext(c)
	if !ok || s == nil {
		return noSessionError()
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
		return noSessionError()
	}
	s.Delete(AccessTokenKey)
	s.Delete(RefreshTokenKey)
	return nil
}

func noSessionError() error {
	return coreerrors.NewNotInitialized(ErrNoSession.Error(), ErrNoSession).
		WithDetails(map[string]interface{}{"family": "http_session"})
}
