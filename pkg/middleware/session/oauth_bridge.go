package session

import (
	"strings"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// BearerFromSession injects Authorization Bearer token from server-side session
// when the incoming request does not already include Authorization header.
func BearerFromSession() router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			req := c.Request()
			if strings.TrimSpace(req.Header.Get("Authorization")) != "" {
				return next(c)
			}

			accessToken, _, ok := GetOAuthTokens(c)
			if !ok || strings.TrimSpace(accessToken) == "" {
				return next(c)
			}

			updated := req.Clone(req.Context())
			updated.Header = req.Header.Clone()
			updated.Header.Set("Authorization", "Bearer "+accessToken)
			c.SetRequest(updated)

			return next(c)
		}
	}
}
