// Package authentication provides HTTP authentication middleware.
package authentication

import (
	"net/http"
	"strings"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/tenant"
)

// ClaimsKey is the context key for storing JWT claims.
const ClaimsKey = "claims"

// Default header name constants for identity propagation.
const (
	DefaultTenantHeader  = "X-Tenant-ID"
	DefaultSubjectHeader = "X-Subject-ID"
	DefaultScopesHeader  = "X-Scopes"
	DefaultRolesHeader   = "X-Roles"
)

// Authenticate validates JWT bearer tokens and stores claims in the request context.
func Authenticate(validator auth.JWTValidator) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "missing authorization header",
				})
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid authorization header format",
				})
			}

			claims, err := validator.Validate(c.Request().Context(), parts[1])
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid token",
				})
			}

			c.Set(ClaimsKey, claims)
			ctx := auth.WithClaims(c.Request().Context(), claims)
			if strings.TrimSpace(claims.TenantID) != "" {
				ctx = tenant.WithContext(ctx, tenant.Context{
					Tenant:  tenant.Identity{ID: claims.TenantID},
					Subject: claims.Subject,
				})
			}
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// IdentityHeaderConfig defines header names used when propagating identity upstream.
type IdentityHeaderConfig struct {
	TenantHeader  string
	SubjectHeader string
	ScopesHeader  string
	RolesHeader   string
}

func (c *IdentityHeaderConfig) normalize() {
	if strings.TrimSpace(c.TenantHeader) == "" {
		c.TenantHeader = DefaultTenantHeader
	}
	if strings.TrimSpace(c.SubjectHeader) == "" {
		c.SubjectHeader = DefaultSubjectHeader
	}
	if strings.TrimSpace(c.ScopesHeader) == "" {
		c.ScopesHeader = DefaultScopesHeader
	}
	if strings.TrimSpace(c.RolesHeader) == "" {
		c.RolesHeader = DefaultRolesHeader
	}
}

// ForwardIdentityHeaders propagates identity information from JWT claims to request headers.
func ForwardIdentityHeaders(cfg IdentityHeaderConfig) router.MiddlewareFunc {
	cfg.normalize()

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			claimsValue := c.Get(ClaimsKey)
			if claimsValue == nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "missing authentication",
				})
			}

			claims, ok := claimsValue.(*auth.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid authentication",
				})
			}

			if strings.TrimSpace(claims.TenantID) != "" {
				c.Request().Header.Set(cfg.TenantHeader, claims.TenantID)
			}
			if strings.TrimSpace(claims.Subject) != "" {
				c.Request().Header.Set(cfg.SubjectHeader, claims.Subject)
			}
			if len(claims.Scopes) > 0 {
				c.Request().Header.Set(cfg.ScopesHeader, strings.Join(claims.Scopes, " "))
			}
			if len(claims.Roles) > 0 {
				c.Request().Header.Set(cfg.RolesHeader, strings.Join(claims.Roles, ","))
			}

			return next(c)
		}
	}
}
