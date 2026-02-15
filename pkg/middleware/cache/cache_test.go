package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/middleware/authz"
	"github.com/nimburion/nimburion/pkg/server/router"
	nethttprouter "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_DisabledByDefault(t *testing.T) {
	r := nethttprouter.NewRouter()
	var calls int32

	cfg := DefaultConfig()
	cfg.Store = NewInMemoryStore()
	mw := Middleware(cfg)

	r.GET("/items", func(c router.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.String(http.StatusOK, fmt.Sprintf("v%d", n))
	}, mw)

	resp1 := performRequest(r, http.MethodGet, "/items", nil)
	resp2 := performRequest(r, http.MethodGet, "/items", nil)

	if resp1.Body.String() != "v1" || resp2.Body.String() != "v2" {
		t.Fatalf("expected middleware disabled behavior, got %q and %q", resp1.Body.String(), resp2.Body.String())
	}
}

func TestMiddleware_HitMissHeadAndETag(t *testing.T) {
	r := nethttprouter.NewRouter()
	var calls int32

	mw, err := New(Config{
		Enabled: true,
		Store:   NewInMemoryStore(),
		TTL:     time.Minute,
		Public:  true,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	r.GET("/resource", func(c router.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.String(http.StatusOK, fmt.Sprintf("data-%d", n))
	}, mw)
	resp1 := performRequest(r, http.MethodGet, "/resource", nil)
	if got := resp1.Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("expected MISS on first request, got %q", got)
	}

	resp2 := performRequest(r, http.MethodGet, "/resource", nil)
	if got := resp2.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("expected HIT on second request, got %q", got)
	}
	if resp2.Body.String() != "data-1" {
		t.Fatalf("unexpected cached body: %q", resp2.Body.String())
	}

	etag := resp2.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected ETag on cached hit")
	}

	resp3 := performRequest(r, http.MethodGet, "/resource", map[string]string{
		"If-None-Match": etag,
	})
	if resp3.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", resp3.Code)
	}

	if calls != 1 {
		t.Fatalf("expected one backend call, got %d", calls)
	}
}

func TestMiddleware_Bypass(t *testing.T) {
	r := nethttprouter.NewRouter()
	var calls int32

	mw, err := New(Config{
		Enabled:          true,
		Store:            NewInMemoryStore(),
		TTL:              time.Minute,
		BypassQueryParam: "__debug_cache",
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	r.GET("/items", func(c router.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.String(http.StatusOK, fmt.Sprintf("v%d", n))
	}, mw)

	performRequest(r, http.MethodGet, "/items", nil)
	respBypassHeader := performRequest(r, http.MethodGet, "/items", map[string]string{
		"Cache-Control": "no-cache",
	})
	respBypassQuery := performRequest(r, http.MethodGet, "/items?__debug_cache=1", nil)

	if respBypassHeader.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected BYPASS for no-cache header")
	}
	if respBypassQuery.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected BYPASS for bypass query param")
	}
	if calls != 3 {
		t.Fatalf("expected 3 backend calls with bypasses, got %d", calls)
	}
}

func TestMiddleware_KeyRulesClaims(t *testing.T) {
	r := nethttprouter.NewRouter()
	var calls int32

	mw, err := New(Config{
		Enabled: true,
		Store:   NewInMemoryStore(),
		TTL:     time.Minute,
		KeyRules: []CacheKeyRule{
			{Source: KeySourceRoute, Key: "tenantId"},
			{Source: KeySourceTenant},
			{Source: KeySourceAuthFingerprint},
		},
		Sensitive: true,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set(authz.ClaimsKey, &auth.Claims{
				Subject:  c.Request().Header.Get("X-Sub"),
				TenantID: c.Request().Header.Get("X-Tenant-ID"),
				Scopes:   []string{c.Request().Header.Get("X-Scope")},
			})
			return next(c)
		}
	})

	r.GET("/tenants/:tenantId/orders", func(c router.Context) error {
		n := atomic.AddInt32(&calls, 1)
		return c.String(http.StatusOK, fmt.Sprintf("orders-%d", n))
	}, mw)

	h := map[string]string{
		"X-Sub":       "user-1",
		"X-Tenant-ID": "t-1",
		"X-Scope":     "orders:read",
	}

	resp1 := performRequest(r, http.MethodGet, "/tenants/t-1/orders", h)
	resp2 := performRequest(r, http.MethodGet, "/tenants/t-1/orders", h)
	if resp2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT on second request")
	}
	if resp1.Body.String() != resp2.Body.String() {
		t.Fatalf("expected same cached payload, got %q vs %q", resp1.Body.String(), resp2.Body.String())
	}

	h2 := map[string]string{
		"X-Sub":       "user-2",
		"X-Tenant-ID": "t-1",
		"X-Scope":     "orders:read",
	}
	resp3 := performRequest(r, http.MethodGet, "/tenants/t-1/orders", h2)
	if resp3.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected MISS with different auth fingerprint")
	}
	if calls != 2 {
		t.Fatalf("expected 2 backend calls, got %d", calls)
	}
}

func TestMiddleware_SWRServesStaleDuringRefresh(t *testing.T) {
	r := nethttprouter.NewRouter()
	var calls int32
	release := make(chan struct{})

	mw, err := New(Config{
		Enabled:                true,
		Store:                  NewInMemoryStore(),
		TTL:                    20 * time.Millisecond,
		StaleWhileRevalidate:   200 * time.Millisecond,
		RequireAuthFingerprint: false,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	r.GET("/slow", func(c router.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n == 2 {
			<-release
		}
		return c.String(http.StatusOK, fmt.Sprintf("value-%d", n))
	}, mw)

	first := performRequest(r, http.MethodGet, "/slow", nil)
	if first.Body.String() != "value-1" {
		t.Fatalf("unexpected first body: %q", first.Body.String())
	}

	time.Sleep(35 * time.Millisecond)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- performRequest(r, http.MethodGet, "/slow", nil)
	}()

	time.Sleep(20 * time.Millisecond)
	stale := performRequest(r, http.MethodGet, "/slow", nil)
	if stale.Header().Get("X-Cache") != "STALE" {
		t.Fatalf("expected STALE while refresh in-flight, got %q", stale.Header().Get("X-Cache"))
	}
	if stale.Body.String() != "value-1" {
		t.Fatalf("expected stale body value-1, got %q", stale.Body.String())
	}

	close(release)
	refreshResp := <-done
	if refreshResp.Body.String() != "value-2" {
		t.Fatalf("expected refreshed body value-2, got %q", refreshResp.Body.String())
	}

	hit := performRequest(r, http.MethodGet, "/slow", nil)
	if hit.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected HIT after refresh, got %q", hit.Header().Get("X-Cache"))
	}
	if hit.Body.String() != "value-2" {
		t.Fatalf("expected cached refreshed body value-2, got %q", hit.Body.String())
	}
}

func TestValidateConfig_SensitiveRequiresStrongRules(t *testing.T) {
	err := ValidateConfig(Config{
		Enabled:   true,
		Store:     NewInMemoryStore(),
		TTL:       time.Minute,
		Sensitive: true,
		KeyRules: []CacheKeyRule{
			{Source: KeySourceHeader, Key: "Accept-Language"},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for weak key rules on sensitive route")
	}
}

func performRequest(r http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}
