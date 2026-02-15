package timeout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_DeadlineExceededReturns504(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Default: 5 * time.Millisecond,
	}))
	r.GET("/slow", func(c router.Context) error {
		<-c.Request().Context().Done()
		return c.Request().Context().Err()
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status %d, got %d", http.StatusGatewayTimeout, rec.Code)
	}
}

func TestMiddleware_ExcludedPathBypassesTimeout(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled:              true,
		Default:              1 * time.Millisecond,
		ExcludedPathPrefixes: []string{"/health"},
	}))
	r.GET("/health/live", func(c router.Context) error {
		time.Sleep(3 * time.Millisecond)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestMiddleware_PathPolicyOffBypassesTimeout(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Default: 1 * time.Millisecond,
		PathPolicies: []PathPolicy{
			{Prefix: "/api/public", Mode: ModeOff},
		},
	}))
	r.GET("/api/public/info", func(c router.Context) error {
		time.Sleep(3 * time.Millisecond)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/public/info", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestMiddleware_PassesHandlerErrorsWhenNotTimeout(t *testing.T) {
	boom := errors.New("boom")

	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Default: 50 * time.Millisecond,
	}))
	r.GET("/error", func(c router.Context) error {
		return boom
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestModeForPath_BestPrefixWins(t *testing.T) {
	cfg := normalize(Config{
		Enabled: true,
		PathPolicies: []PathPolicy{
			{Prefix: "/api", Mode: ModeOff},
			{Prefix: "/api/v1/private", Mode: ModeOn},
		},
	})

	mode := cfg.modeForPath("/api/v1/private/items")
	if mode != ModeOn {
		t.Fatalf("expected mode %q, got %q", ModeOn, mode)
	}
}

func TestIsDeadlineExceeded(t *testing.T) {
	if !isDeadlineExceeded(context.DeadlineExceeded, nil) {
		t.Fatal("expected deadline exceeded from handler error")
	}
	if !isDeadlineExceeded(nil, context.DeadlineExceeded) {
		t.Fatal("expected deadline exceeded from request context error")
	}
	if isDeadlineExceeded(errors.New("boom"), nil) {
		t.Fatal("did not expect deadline exceeded")
	}
}
