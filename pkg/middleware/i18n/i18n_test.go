package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	frameworki18n "github.com/nimburion/nimburion/pkg/i18n"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_ResolvesLocaleByPriority(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.SupportedLocales = []string{"en", "it"}
	cfg.DefaultLocale = "en"
	cfg.QueryParam = "lang"
	cfg.HeaderName = "X-Locale"

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	var localeFromCtx string
	r.GET("/test", func(c router.Context) error {
		localeFromCtx = frameworki18n.GetLocale(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test?lang=it", nil)
	req.Header.Set("X-Locale", "en")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if localeFromCtx != "it" {
		t.Fatalf("expected locale it, got %q", localeFromCtx)
	}
	if rec.Header().Get("Content-Language") != "it" {
		t.Fatalf("expected Content-Language=it, got %q", rec.Header().Get("Content-Language"))
	}
}

func TestMiddleware_UsesAcceptLanguageFallbackToBaseLocale(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.SupportedLocales = []string{"en", "it"}
	cfg.DefaultLocale = "en"
	cfg.QueryParam = "lang"
	cfg.HeaderName = "X-Locale"
	cfg.FallbackMode = "base"

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	var localeFromCtx string
	r.GET("/test", func(c router.Context) error {
		localeFromCtx = frameworki18n.GetLocale(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Language", "it-IT,it;q=0.9,en;q=0.1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if localeFromCtx != "it" {
		t.Fatalf("expected fallback locale it, got %q", localeFromCtx)
	}
}

func TestMiddleware_SetsVaryHeaders(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.SupportedLocales = []string{"en"}
	cfg.DefaultLocale = "en"
	cfg.HeaderName = "X-Locale"

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.GET("/test", func(c router.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	vary := rec.Header().Get("Vary")
	if vary == "" {
		t.Fatal("expected Vary header to be set")
	}
}

func TestDefaultConfig_ExcludedPathPrefixes(t *testing.T) {
	cfg := DefaultConfig()

	expected := map[string]bool{
		"/metrics": false,
		"/health":  false,
		"/version": false,
	}
	for _, path := range cfg.ExcludedPathPrefixes {
		if _, ok := expected[path]; ok {
			expected[path] = true
		}
	}

	for path, found := range expected {
		if !found {
			t.Fatalf("expected %s in default excluded path prefixes", path)
		}
	}
}
