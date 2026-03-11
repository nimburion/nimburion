package authentication

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/http/router/nethttp"
)

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
		expectClaims   bool
	}{
		{name: "missing authorization header", expectedStatus: http.StatusUnauthorized},
		{name: "invalid authorization header format", authHeader: "InvalidToken", expectedStatus: http.StatusUnauthorized},
		{
			name:       "valid token",
			authHeader: "Bearer valid.token.here",
			validatorFunc: func(_ context.Context, _ string) (*auth.Claims, error) {
				return &auth.Claims{Subject: "user123", Scopes: []string{"read", "write"}}, nil
			},
			expectedStatus: http.StatusOK,
			expectClaims:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &mockJWTValidator{validateFunc: tt.validatorFunc}
			r := nethttp.NewRouter()
			r.Use(Authenticate(validator))
			r.GET("/test", func(c router.Context) error {
				if tt.expectClaims {
					if c.Get(ClaimsKey) == nil {
						t.Fatal("expected claims in context")
					}
					if auth.GetClaims(c.Request().Context()) == nil {
						t.Fatal("expected claims in request context")
					}
				}
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
