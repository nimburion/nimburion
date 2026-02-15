package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// mockJWTValidator is a mock implementation of auth.JWTValidator for testing.
type mockJWTValidator struct {
	validateFunc func(ctx context.Context, token string) (*auth.Claims, error)
}

func (m *mockJWTValidator) Validate(ctx context.Context, token string) (*auth.Claims, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, token)
	}
	return nil, errors.New("not implemented")
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		validatorFunc  func(ctx context.Context, token string) (*auth.Claims, error)
		expectedStatus int
		expectedError  string
		expectClaims   bool
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing authorization header",
			expectClaims:   false,
		},
		{
			name:           "invalid authorization header format - no Bearer",
			authHeader:     "InvalidToken",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid authorization header format",
			expectClaims:   false,
		},
		{
			name:           "invalid authorization header format - wrong scheme",
			authHeader:     "Basic dXNlcjpwYXNz",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid authorization header format",
			expectClaims:   false,
		},
		{
			name:       "invalid token - validation fails",
			authHeader: "Bearer invalid.token.here",
			validatorFunc: func(ctx context.Context, token string) (*auth.Claims, error) {
				return nil, errors.New("token validation failed")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid token",
			expectClaims:   false,
		},
		{
			name:       "valid token - authentication succeeds",
			authHeader: "Bearer valid.token.here",
			validatorFunc: func(ctx context.Context, token string) (*auth.Claims, error) {
				return &auth.Claims{
					Subject: "user123",
					Issuer:  "https://auth.example.com",
					Scopes:  []string{"read", "write"},
				}, nil
			},
			expectedStatus: http.StatusOK,
			expectClaims:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock validator
			validator := &mockJWTValidator{
				validateFunc: tt.validatorFunc,
			}

			// Create router and apply middleware
			r := nethttp.NewRouter()
			r.Use(Authenticate(validator))

			// Create test handler that checks for claims
			r.GET("/test", func(c router.Context) error {
				if tt.expectClaims {
					claims := c.Get(ClaimsKey)
					if claims == nil {
						t.Error("expected claims in context, got nil")
					}

					// Verify claims are also in request context
					reqClaims := auth.GetClaims(c.Request().Context())
					if reqClaims == nil {
						t.Error("expected claims in request context, got nil")
					}
				}
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Record response
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify status code
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify error message if expected
			if tt.expectedError != "" {
				body := w.Body.String()
				if body == "" {
					t.Errorf("expected error message containing %q, got empty body", tt.expectedError)
				}
			}
		})
	}
}

func TestRequireScopes(t *testing.T) {
	tests := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing authentication - no claims",
			userScopes:     nil,
			requiredScopes: []string{"read"},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing authentication",
		},
		{
			name:           "user has all required scopes",
			userScopes:     []string{"read", "write", "delete"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user missing one required scope",
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusForbidden,
			expectedError:  "insufficient permissions",
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"read"},
			expectedStatus: http.StatusForbidden,
			expectedError:  "insufficient permissions",
		},
		{
			name:           "no scopes required - always passes",
			userScopes:     []string{"read"},
			requiredScopes: []string{},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user has exact required scopes",
			userScopes:     []string{"admin"},
			requiredScopes: []string{"admin"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router
			r := nethttp.NewRouter()

			// Create middleware that sets up claims (simulating Authenticate middleware)
			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					if tt.userScopes != nil {
						claims := &auth.Claims{
							Subject: "user123",
							Scopes:  tt.userScopes,
						}
						c.Set(ClaimsKey, claims)
					}
					return next(c)
				}
			}

			// Apply middleware
			r.Use(setupClaims)
			r.Use(RequireScopes(tt.requiredScopes...))

			// Create test handler
			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify status code
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify error message if expected
			if tt.expectedError != "" {
				body := w.Body.String()
				if body == "" {
					t.Errorf("expected error message containing %q, got empty body", tt.expectedError)
				}
			}
		})
	}
}

func TestRequireAnyScope(t *testing.T) {
	tests := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "missing authentication - no claims",
			userScopes:     nil,
			requiredScopes: []string{"read"},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing authentication",
		},
		{
			name:           "user has one of required scopes",
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user has all required scopes",
			userScopes:     []string{"read", "write", "delete"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user has none of required scopes",
			userScopes:     []string{"delete"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusForbidden,
			expectedError:  "insufficient permissions",
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"read"},
			expectedStatus: http.StatusForbidden,
			expectedError:  "insufficient permissions",
		},
		{
			name:           "no scopes required - always passes",
			userScopes:     []string{"read"},
			requiredScopes: []string{},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user has exact one required scope",
			userScopes:     []string{"admin"},
			requiredScopes: []string{"admin"},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router
			r := nethttp.NewRouter()

			// Create middleware that sets up claims (simulating Authenticate middleware)
			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					if tt.userScopes != nil {
						claims := &auth.Claims{
							Subject: "user123",
							Scopes:  tt.userScopes,
						}
						c.Set(ClaimsKey, claims)
					}
					return next(c)
				}
			}

			// Apply middleware
			r.Use(setupClaims)
			r.Use(RequireAnyScope(tt.requiredScopes...))

			// Create test handler
			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify status code
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify error message if expected
			if tt.expectedError != "" {
				body := w.Body.String()
				if body == "" {
					t.Errorf("expected error message containing %q, got empty body", tt.expectedError)
				}
			}
		})
	}
}

func TestRequireScopesWithLogic(t *testing.T) {
	tests := []struct {
		name           string
		logic          ScopeLogic
		userScopes     []string
		requiredScopes []string
		expectedStatus int
	}{
		{
			name:           "AND logic - user has all scopes",
			logic:          ScopeLogicAND,
			userScopes:     []string{"read", "write"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "AND logic - user missing one scope",
			logic:          ScopeLogicAND,
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "OR logic - user has one scope",
			logic:          ScopeLogicOR,
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OR logic - user has no scopes",
			logic:          ScopeLogicOR,
			userScopes:     []string{"delete"},
			requiredScopes: []string{"read", "write"},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router
			r := nethttp.NewRouter()

			// Create middleware that sets up claims
			setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					claims := &auth.Claims{
						Subject: "user123",
						Scopes:  tt.userScopes,
					}
					c.Set(ClaimsKey, claims)
					return next(c)
				}
			}

			// Apply middleware
			r.Use(setupClaims)
			r.Use(RequireScopesWithLogic(tt.logic, tt.requiredScopes...))

			// Create test handler
			r.GET("/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify status code
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHasAllScopes(t *testing.T) {
	tests := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expected       bool
	}{
		{
			name:           "user has all required scopes",
			userScopes:     []string{"read", "write", "delete"},
			requiredScopes: []string{"read", "write"},
			expected:       true,
		},
		{
			name:           "user missing one scope",
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expected:       false,
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"read"},
			expected:       false,
		},
		{
			name:           "no scopes required",
			userScopes:     []string{"read"},
			requiredScopes: []string{},
			expected:       true,
		},
		{
			name:           "exact match",
			userScopes:     []string{"admin"},
			requiredScopes: []string{"admin"},
			expected:       true,
		},
		{
			name:           "user has extra scopes",
			userScopes:     []string{"read", "write", "admin"},
			requiredScopes: []string{"read"},
			expected:       true,
		},
		{
			name:           "case sensitive - no match",
			userScopes:     []string{"Read"},
			requiredScopes: []string{"read"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAllScopes(tt.userScopes, tt.requiredScopes)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasAnyScope(t *testing.T) {
	tests := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expected       bool
	}{
		{
			name:           "user has one of required scopes",
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expected:       true,
		},
		{
			name:           "user has all required scopes",
			userScopes:     []string{"read", "write", "delete"},
			requiredScopes: []string{"read", "write"},
			expected:       true,
		},
		{
			name:           "user has none of required scopes",
			userScopes:     []string{"delete"},
			requiredScopes: []string{"read", "write"},
			expected:       false,
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"read"},
			expected:       false,
		},
		{
			name:           "no scopes required",
			userScopes:     []string{"read"},
			requiredScopes: []string{},
			expected:       true,
		},
		{
			name:           "exact match",
			userScopes:     []string{"admin"},
			requiredScopes: []string{"admin"},
			expected:       true,
		},
		{
			name:           "case sensitive - no match",
			userScopes:     []string{"Read"},
			requiredScopes: []string{"read"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasAnyScope(tt.userScopes, tt.requiredScopes)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasRequiredScopes(t *testing.T) {
	tests := []struct {
		name           string
		userScopes     []string
		requiredScopes []string
		expected       bool
	}{
		{
			name:           "user has all required scopes",
			userScopes:     []string{"read", "write", "delete"},
			requiredScopes: []string{"read", "write"},
			expected:       true,
		},
		{
			name:           "user missing one scope",
			userScopes:     []string{"read"},
			requiredScopes: []string{"read", "write"},
			expected:       false,
		},
		{
			name:           "user has no scopes",
			userScopes:     []string{},
			requiredScopes: []string{"read"},
			expected:       false,
		},
		{
			name:           "no scopes required",
			userScopes:     []string{"read"},
			requiredScopes: []string{},
			expected:       true,
		},
		{
			name:           "exact match",
			userScopes:     []string{"admin"},
			requiredScopes: []string{"admin"},
			expected:       true,
		},
		{
			name:           "user has extra scopes",
			userScopes:     []string{"read", "write", "admin"},
			requiredScopes: []string{"read"},
			expected:       true,
		},
		{
			name:           "case sensitive - no match",
			userScopes:     []string{"Read"},
			requiredScopes: []string{"read"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRequiredScopes(tt.userScopes, tt.requiredScopes)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAuthenticateIntegration(t *testing.T) {
	// This test verifies the full authentication flow with a real validator
	t.Run("full authentication flow", func(t *testing.T) {
		// Create a validator that returns specific claims
		validator := &mockJWTValidator{
			validateFunc: func(ctx context.Context, token string) (*auth.Claims, error) {
				if token == "valid-token" {
					return &auth.Claims{
						Subject: "user123",
						Issuer:  "https://auth.example.com",
						Scopes:  []string{"todos:read", "todos:write"},
					}, nil
				}
				return nil, errors.New("invalid token")
			},
		}

		// Create router with authentication and authorization
		r := nethttp.NewRouter()
		r.Use(Authenticate(validator))

		// Protected route requiring specific scopes
		r.GET("/todos", func(c router.Context) error {
			claims := c.Get(ClaimsKey).(*auth.Claims)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": "todos retrieved",
				"user":    claims.Subject,
			})
		}, RequireScopes("todos:read"))

		// Test with valid token
		req := httptest.NewRequest(http.MethodGet, "/todos", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		// Test with invalid token
		req = httptest.NewRequest(http.MethodGet, "/todos", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		// Test with missing token
		req = httptest.NewRequest(http.MethodGet, "/todos", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

// TestAuthorizationEdgeCases tests edge cases for authorization middleware.
// Requirements: 6.4
func TestAuthorizationEdgeCases(t *testing.T) {
	t.Run("missing claims returns 401", func(t *testing.T) {
		// Create router without authentication middleware
		r := nethttp.NewRouter()
		r.Use(RequireScopes("read"))

		r.GET("/test", func(c router.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		body := w.Body.String()
		if !contains(body, "missing authentication") {
			t.Errorf("expected error message about missing authentication, got: %s", body)
		}
	})

	t.Run("empty scopes list - always passes", func(t *testing.T) {
		// Create router with authentication
		r := nethttp.NewRouter()

		// Setup claims middleware
		setupClaims := func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				claims := &auth.Claims{
					Subject: "user123",
					Scopes:  []string{"read"},
				}
				c.Set(ClaimsKey, claims)
				return next(c)
			}
		}

		r.Use(setupClaims)
		r.Use(RequireScopes()) // Empty scopes list

		r.GET("/test", func(c router.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 for empty scopes list, got %d", w.Code)
		}
	})

	t.Run("user with empty scopes fails when scopes required", func(t *testing.T) {
		// Create router with authentication
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
		r.Use(RequireScopes("read"))

		r.GET("/test", func(c router.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403 for user with no scopes, got %d", w.Code)
		}
	})

	t.Run("invalid claims type returns 401", func(t *testing.T) {
		// Create router
		r := nethttp.NewRouter()

		// Setup invalid claims middleware
		setupInvalidClaims := func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				// Set wrong type for claims
				c.Set(ClaimsKey, "invalid-claims-type")
				return next(c)
			}
		}

		r.Use(setupInvalidClaims)
		r.Use(RequireScopes("read"))

		r.GET("/test", func(c router.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 for invalid claims type, got %d", w.Code)
		}

		body := w.Body.String()
		if !contains(body, "invalid authentication") {
			t.Errorf("expected error message about invalid authentication, got: %s", body)
		}
	})
}

func TestClaimsGuardFromConfig(t *testing.T) {
	cfg := config.AuthConfig{
		Claims: config.AuthClaimsConfig{
			Mappings: map[string][]string{
				"tenant_id": {"tenantId"},
			},
			Rules: []config.AuthClaimRule{
				{
					Claim:    "tenant_id",
					Operator: "required",
				},
				{
					Claim:    "tenant_id",
					Operator: "equals",
					Source:   "header",
					Key:      "X-Tenant-ID",
				},
			},
		},
	}

	r := nethttp.NewRouter()
	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set(ClaimsKey, &auth.Claims{TenantID: "tenant-1"})
			return next(c)
		}
	})
	r.Use(ClaimsGuardFromConfig(cfg))
	r.GET("/test", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestForwardIdentityHeaders(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set(ClaimsKey, &auth.Claims{
				Subject:  "user-1",
				TenantID: "tenant-1",
				Scopes:   []string{"orders:read", "orders:write"},
				Roles:    []string{"admin", "ops"},
			})
			return next(c)
		}
	})
	r.Use(ForwardIdentityHeaders(IdentityHeaderConfig{}))

	r.GET("/test", func(c router.Context) error {
		headers := c.Request().Header
		if got := headers.Get(DefaultTenantHeader); got != "tenant-1" {
			t.Fatalf("expected tenant header tenant-1, got %q", got)
		}
		if got := headers.Get(DefaultSubjectHeader); got != "user-1" {
			t.Fatalf("expected subject header user-1, got %q", got)
		}
		if got := headers.Get(DefaultScopesHeader); got != "orders:read orders:write" {
			t.Fatalf("unexpected scopes header: %q", got)
		}
		if got := headers.Get(DefaultRolesHeader); got != "admin,ops" {
			t.Fatalf("unexpected roles header: %q", got)
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
