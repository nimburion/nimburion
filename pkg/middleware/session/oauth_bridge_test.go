package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestBearerFromSession_InjectsAuthorizationWhenMissing(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   NewInMemoryStore(),
		TTL:     time.Hour,
	}))
	r.Use(BearerFromSession())

	r.GET("/login", func(c router.Context) error {
		if err := SetOAuthTokens(c, "access-from-session", "refresh"); err != nil {
			t.Fatalf("set oauth tokens: %v", err)
		}
		return c.String(http.StatusOK, "ok")
	})
	r.GET("/protected", func(c router.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		return c.String(http.StatusOK, authHeader)
	})

	loginReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)

	sidCookie := findLastCookieByName(loginW.Result(), "sid")
	if sidCookie == nil {
		t.Fatalf("expected sid cookie from login")
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protectedReq.AddCookie(sidCookie)
	protectedW := httptest.NewRecorder()
	r.ServeHTTP(protectedW, protectedReq)

	if protectedW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", protectedW.Code)
	}
	if got := protectedW.Body.String(); got != "Bearer access-from-session" {
		t.Fatalf("expected injected bearer token, got %q", got)
	}
}

func TestBearerFromSession_DoesNotOverrideExistingAuthorization(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   NewInMemoryStore(),
		TTL:     time.Hour,
	}))
	r.Use(BearerFromSession())

	r.GET("/set", func(c router.Context) error {
		if err := SetOAuthTokens(c, "session-token", "refresh"); err != nil {
			t.Fatalf("set oauth tokens: %v", err)
		}
		return c.String(http.StatusOK, "ok")
	})
	r.GET("/who", func(c router.Context) error {
		return c.String(http.StatusOK, c.Request().Header.Get("Authorization"))
	})

	setReq := httptest.NewRequest(http.MethodGet, "/set", nil)
	setW := httptest.NewRecorder()
	r.ServeHTTP(setW, setReq)
	sidCookie := findLastCookieByName(setW.Result(), "sid")

	req := httptest.NewRequest(http.MethodGet, "/who", nil)
	req.AddCookie(sidCookie)
	req.Header.Set("Authorization", "Bearer incoming")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Body.String(); got != "Bearer incoming" {
		t.Fatalf("expected incoming authorization to be preserved, got %q", got)
	}
}
