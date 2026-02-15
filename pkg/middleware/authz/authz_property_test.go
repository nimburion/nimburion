package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// TestProperty6_ScopeBasedAuthorization verifies that scope-based authorization works correctly.
// Property 6: Scope-Based Authorization
//
// *For any* endpoint with required scopes, requests with tokens containing all required scopes
// should be allowed (HTTP 200/201), and requests with tokens missing any required scope should
// be rejected with HTTP 403.
//
// **Validates: Requirements 6.1, 6.2, 6.3, 6.4**
func TestProperty6_ScopeBasedAuthorization(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for scope strings
	genScope := gen.Identifier()

	// Generator for non-empty list of scopes (1-3 scopes)
	genRequiredScopes := gen.SliceOfN(3, genScope).SuchThat(func(scopes []string) bool {
		return len(scopes) > 0
	})

	// Generator for additional scopes (0-3 scopes)
	genAdditionalScopes := gen.SliceOfN(3, genScope)

	properties.Property("requests with all required scopes are allowed", prop.ForAll(
		func(requiredScopes, additionalScopes []string) bool {
			// Create a superset: user has all required scopes plus additional ones
			allScopes := make([]string, len(requiredScopes))
			copy(allScopes, requiredScopes)
			allScopes = append(allScopes, additionalScopes...)

			// Remove duplicates
			scopeSet := make(map[string]bool)
			for _, scope := range allScopes {
				scopeSet[scope] = true
			}
			uniqueScopes := make([]string, 0, len(scopeSet))
			for scope := range scopeSet {
				uniqueScopes = append(uniqueScopes, scope)
			}

			// Create router with authorization middleware
			r := nethttp.NewRouter()

			// Setup claims middleware
			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  uniqueScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopes(requiredScopes...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 200 since user has all required scopes
			return w.Code == http.StatusOK
		},
		genRequiredScopes,
		genAdditionalScopes,
	))

	properties.Property("requests missing required scopes are rejected with 403", prop.ForAll(
		func(requiredScopes []string) bool {
			// Need at least 2 scopes to have one missing
			if len(requiredScopes) < 2 {
				return true
			}

			// Remove duplicates from required scopes first
			scopeSet := make(map[string]bool)
			for _, scope := range requiredScopes {
				scopeSet[scope] = true
			}
			uniqueRequired := make([]string, 0, len(scopeSet))
			for scope := range scopeSet {
				uniqueRequired = append(uniqueRequired, scope)
			}

			// Need at least 2 unique scopes
			if len(uniqueRequired) < 2 {
				return true
			}

			// User has all scopes except the last one
			userScopes := uniqueRequired[:len(uniqueRequired)-1]

			// Create router with authorization middleware
			r := nethttp.NewRouter()

			// Setup claims middleware
			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  userScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopes(uniqueRequired...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 403 since user is missing a required scope
			return w.Code == http.StatusForbidden
		},
		genRequiredScopes,
	))

	properties.Property("requests with no scopes are rejected when scopes required", prop.ForAll(
		func(requiredScopes []string) bool {
			// Create router with authorization middleware
			r := nethttp.NewRouter()

			// Setup claims middleware with empty scopes
			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  []string{}, // Empty scopes
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopes(requiredScopes...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 403 since user has no scopes
			return w.Code == http.StatusForbidden
		},
		genRequiredScopes,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty7_ScopeLogicANDOR verifies that AND and OR logic work correctly for scope requirements.
// Property 7: Scope Logic (AND/OR)
//
// *For any* endpoint with multiple scope requirements, the authorization middleware should correctly
// evaluate AND logic (all scopes required) and OR logic (any scope sufficient) based on configuration.
//
// **Validates: Requirements 6.5**
func TestProperty7_ScopeLogicANDOR(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for scope strings
	genScope := gen.Identifier()

	// Generator for non-empty list of scopes (1-3 scopes)
	genRequiredScopes := gen.SliceOfN(3, genScope).SuchThat(func(scopes []string) bool {
		return len(scopes) > 0
	})

	// Generator for additional scopes (0-3 scopes)
	genAdditionalScopes := gen.SliceOfN(3, genScope)

	properties.Property("AND logic requires all scopes", prop.ForAll(
		func(requiredScopes, extraScopes []string) bool {
			// User has all required scopes plus extra
			userScopes := make([]string, len(requiredScopes))
			copy(userScopes, requiredScopes)
			userScopes = append(userScopes, extraScopes...)

			// Remove duplicates
			scopeSet := make(map[string]bool)
			for _, scope := range userScopes {
				scopeSet[scope] = true
			}
			uniqueScopes := make([]string, 0, len(scopeSet))
			for scope := range scopeSet {
				uniqueScopes = append(uniqueScopes, scope)
			}

			// Create router with AND logic
			r := nethttp.NewRouter()

			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  uniqueScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopesWithLogic(ScopeLogicAND, requiredScopes...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 200 since user has all required scopes
			return w.Code == http.StatusOK
		},
		genRequiredScopes,
		genAdditionalScopes,
	))

	properties.Property("AND logic rejects when missing any scope", prop.ForAll(
		func(requiredScopes []string) bool {
			// Need at least 2 scopes for meaningful test
			if len(requiredScopes) < 2 {
				return true
			}

			// Remove duplicates from required scopes first
			scopeSet := make(map[string]bool)
			for _, scope := range requiredScopes {
				scopeSet[scope] = true
			}
			uniqueRequired := make([]string, 0, len(scopeSet))
			for scope := range scopeSet {
				uniqueRequired = append(uniqueRequired, scope)
			}

			// Need at least 2 unique scopes
			if len(uniqueRequired) < 2 {
				return true
			}

			// User has all scopes except the last one
			userScopes := uniqueRequired[:len(uniqueRequired)-1]

			// Create router with AND logic
			r := nethttp.NewRouter()

			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  userScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopesWithLogic(ScopeLogicAND, uniqueRequired...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 403 since user is missing one scope
			return w.Code == http.StatusForbidden
		},
		genRequiredScopes,
	))

	properties.Property("OR logic allows with any single scope", prop.ForAll(
		func(requiredScopes []string, selectedIndex int) bool {
			// Select one scope from required scopes
			index := selectedIndex % len(requiredScopes)
			userScopes := []string{requiredScopes[index]}

			// Create router with OR logic
			r := nethttp.NewRouter()

			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  userScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopesWithLogic(ScopeLogicOR, requiredScopes...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 200 since user has at least one required scope
			return w.Code == http.StatusOK
		},
		genRequiredScopes,
		gen.IntRange(0, 100),
	))

	properties.Property("OR logic rejects when no matching scopes", prop.ForAll(
		func(requiredScopes, userScopes []string) bool {
			// Ensure user scopes don't overlap with required scopes
			scopeSet := make(map[string]bool)
			for _, scope := range requiredScopes {
				scopeSet[scope] = true
			}

			// Filter out any overlapping scopes
			filteredUserScopes := make([]string, 0)
			for _, scope := range userScopes {
				if !scopeSet[scope] {
					filteredUserScopes = append(filteredUserScopes, scope)
				}
			}

			// Create router with OR logic
			r := nethttp.NewRouter()

			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  filteredUserScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			r.Use(setupClaims)
			r.Use(RequireScopesWithLogic(ScopeLogicOR, requiredScopes...))

			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Execute request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should return 403 since user has no matching scopes
			return w.Code == http.StatusForbidden
		},
		genRequiredScopes,
		genAdditionalScopes,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
