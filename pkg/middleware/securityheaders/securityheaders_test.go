package securityheaders

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_AppliesDefaultHeaders(t *testing.T) {
	r := nethttp.NewRouter()
	cfg := DefaultConfig()
	r.Use(Middleware(cfg))
	r.GET("/ok", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options=DENY, got %q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff, got %q", got)
	}
	if got := w.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatalf("expected Content-Security-Policy header")
	}
	if got := w.Header().Get("Referrer-Policy"); got == "" {
		t.Fatalf("expected Referrer-Policy header")
	}
	if got := w.Header().Get("Permissions-Policy"); got == "" {
		t.Fatalf("expected Permissions-Policy header")
	}
	// Insecure request should not include HSTS.
	if got := w.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS on insecure request, got %q", got)
	}
}

func TestMiddleware_AppliesHSTSOnSecureRequest(t *testing.T) {
	r := nethttp.NewRouter()
	cfg := DefaultConfig()
	r.Use(Middleware(cfg))
	r.GET("/ok", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "https://example.com/ok", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	got := w.Header().Get("Strict-Transport-Security")
	if got == "" {
		t.Fatalf("expected HSTS header on secure request")
	}
}

func TestMiddleware_HTTPSRedirect(t *testing.T) {
	r := nethttp.NewRouter()
	cfg := DefaultConfig()
	cfg.SSLRedirect = true
	r.Use(Middleware(cfg))
	r.GET("/ok", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 redirect, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://example.com/ok" {
		t.Fatalf("unexpected location: %q", got)
	}
}

func TestMiddleware_HTTPSRedirect_TrustedProxyHeader(t *testing.T) {
	r := nethttp.NewRouter()
	cfg := DefaultConfig()
	cfg.SSLRedirect = true
	r.Use(Middleware(cfg))
	r.GET("/ok", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ok", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when proxy marks request secure, got %d", w.Code)
	}
}

func TestMiddleware_AllowedHosts(t *testing.T) {
	r := nethttp.NewRouter()
	cfg := DefaultConfig()
	cfg.AllowedHosts = []string{"api.example.com"}
	r.Use(Middleware(cfg))
	r.GET("/ok", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://evil.example.com/ok", nil)
	req.Host = "evil.example.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for bad host, got %d", w.Code)
	}
}

func TestMiddleware_IsDevelopmentDisablesPolicy(t *testing.T) {
	r := nethttp.NewRouter()
	cfg := DefaultConfig()
	cfg.IsDevelopment = true
	cfg.SSLRedirect = true
	cfg.AllowedHosts = []string{"api.example.com"}
	r.Use(Middleware(cfg))
	r.GET("/ok", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "http://evil.example.com/ok", nil)
	req.Host = "evil.example.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 in development mode, got %d", w.Code)
	}
}
