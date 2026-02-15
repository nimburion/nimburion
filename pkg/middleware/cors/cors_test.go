package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestCORS_SimpleRequest(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:      true,
		AllowOrigins: []string{"https://app.example.com"},
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("expected Access-Control-Allow-Origin to match origin, got %q", got)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:      true,
		AllowOrigins: []string{"https://app.example.com"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodOptions, "/members", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods header")
	}
}

func TestCORS_RejectsUnknownOriginPreflight(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:      true,
		AllowOrigins: []string{"https://allowed.example.com"},
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodOptions, "/members", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", w.Code)
	}
}

func TestCORS_AllowAllOriginsDisablesCredentials(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:          true,
		AllowAllOrigins:  true,
		AllowCredentials: true,
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	req.Header.Set("Origin", "https://any.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin='*', got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("expected no credentials header with AllowAllOrigins, got %q", got)
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:       true,
		AllowWildcard: true,
		AllowOrigins:  []string{"https://*.example.com"},
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	req.Header.Set("Origin", "https://tenant.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://tenant.example.com" {
		t.Fatalf("expected origin reflection for wildcard match, got %q", got)
	}
}

func TestCORS_AllowOriginWithContextFuncPreferred(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		AllowOriginFunc: func(origin string) bool {
			return false
		},
		AllowOriginWithContextFunc: func(c router.Context, origin string) bool {
			return origin == "https://ctx.example.com"
		},
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	req.Header.Set("Origin", "https://ctx.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://ctx.example.com" {
		t.Fatalf("expected context function to allow origin, got %q", got)
	}
}

func TestCORS_PrivateNetworkPreflightHeader(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:             true,
		AllowOrigins:        []string{"https://app.example.com"},
		AllowPrivateNetwork: true,
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodOptions, "/members", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Fatalf("expected Access-Control-Allow-Private-Network=true, got %q", got)
	}
}

func TestCORS_CustomSchemaAllowed(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:       true,
		AllowOrigins:  []string{"tauri://app.local"},
		CustomSchemas: []string{"tauri"},
	}))

	r.GET("/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	req.Header.Set("Origin", "tauri://app.local")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "tauri://app.local" {
		t.Fatalf("expected custom schema origin allowed, got %q", got)
	}
}
