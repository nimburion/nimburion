package authorization

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/http/authentication"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/http/router/nethttp"
)

func TestRequireScopes(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(withClaims(&auth.Claims{Subject: "user123", Scopes: []string{"read"}}))
	r.Use(RequireScopes("read"))
	r.GET("/test", okHandler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestClaimsGuard_EqualsRoute(t *testing.T) {
	r := mustRouter(t)
	r.Use(withClaims(&auth.Claims{TenantID: "tenant-a"}))
	r.GET("/tenants/:tenantId/orders", okHandler, ClaimsGuard(ClaimRule{
		Claim:    "tenant_id",
		Operator: ClaimOperatorEquals,
		Source:   ClaimValueSourceRoute,
		Key:      "tenantId",
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tenants/tenant-a/orders", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHasAnyScope(t *testing.T) {
	if !HasAnyScope([]string{"read"}, []string{"read", "write"}) {
		t.Fatal("expected HasAnyScope to return true")
	}
}

func mustRouter(t *testing.T) router.Router {
	t.Helper()
	return nethttp.NewRouter()
}

func withClaims(claims *auth.Claims) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set(authentication.ClaimsKey, claims)
			return next(c)
		}
	}
}

func okHandler(c router.Context) error {
	return c.String(http.StatusOK, "ok")
}
