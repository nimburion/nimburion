package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/factory"
)

func TestClaimsGuard_Required(t *testing.T) {
	r := mustRouter(t)
	r.Use(withClaims(&auth.Claims{TenantID: "tenant-a"}))
	r.GET("/test", okHandler, ClaimsGuard(ClaimRule{
		Claim:    "tenant_id",
		Operator: ClaimOperatorRequired,
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-a/orders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for matching route tenant, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/tenants/tenant-b/orders", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatching route tenant, got %d", w2.Code)
	}
}

func TestClaimsGuard_EqualsHeaderAndQuery(t *testing.T) {
	r := mustRouter(t)
	r.Use(withClaims(&auth.Claims{TenantID: "tenant-a"}))
	r.GET("/header", okHandler, ClaimsGuard(ClaimRule{
		Claim:    "tenant_id",
		Operator: ClaimOperatorEquals,
		Source:   ClaimValueSourceHeader,
		Key:      "X-Tenant-ID",
	}))
	r.GET("/query", okHandler, ClaimsGuard(ClaimRule{
		Claim:    "tenant_id",
		Operator: ClaimOperatorEquals,
		Source:   ClaimValueSourceQuery,
		Key:      "tenant",
	}))

	req := httptest.NewRequest(http.MethodGet, "/header", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for matching header tenant, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/query?tenant=tenant-a", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for matching query tenant, got %d", w2.Code)
	}
}

func TestClaimsGuard_OneOfAndRegex(t *testing.T) {
	r := mustRouter(t)
	r.Use(withClaims(&auth.Claims{TenantID: "tenant-blue"}))
	r.GET("/oneof", okHandler, ClaimsGuard(ClaimRule{
		Claim:    "tenant_id",
		Operator: ClaimOperatorOneOf,
		Values:   []string{"tenant-red", "tenant-blue"},
	}))
	r.GET("/regex", okHandler, ClaimsGuard(ClaimRule{
		Claim:    "tenant_id",
		Operator: ClaimOperatorRegex,
		Values:   []string{"^tenant-[a-z]+$"},
	}))

	wOneOf := httptest.NewRecorder()
	r.ServeHTTP(wOneOf, httptest.NewRequest(http.MethodGet, "/oneof", nil))
	if wOneOf.Code != http.StatusOK {
		t.Fatalf("expected 200 for one_of match, got %d", wOneOf.Code)
	}

	wRegex := httptest.NewRecorder()
	r.ServeHTTP(wRegex, httptest.NewRequest(http.MethodGet, "/regex", nil))
	if wRegex.Code != http.StatusOK {
		t.Fatalf("expected 200 for regex match, got %d", wRegex.Code)
	}
}

func mustRouter(t *testing.T) router.Router {
	t.Helper()
	r, err := factory.NewRouter("nethttp")
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	return r
}

func withClaims(claims *auth.Claims) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set(ClaimsKey, claims)
			return next(c)
		}
	}
}

func okHandler(c router.Context) error {
	return c.String(http.StatusOK, "ok")
}
