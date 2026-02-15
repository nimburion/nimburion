package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func findLastCookieByName(resp *http.Response, name string) *http.Cookie {
	var found *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			copied := cookie
			found = copied
		}
	}
	return found
}

func TestSessionMiddleware_CreatesSessionCookie(t *testing.T) {
	store := NewInMemoryStore()
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   store,
		TTL:     time.Hour,
	}))
	r.GET("/ok", func(c router.Context) error {
		s, ok := FromContext(c)
		if !ok || s == nil {
			t.Fatalf("expected session in context")
		}
		s.Set("k", "v")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	cookie := findLastCookieByName(w.Result(), "sid")
	if cookie == nil {
		t.Fatalf("expected session cookie")
	}
}

func TestSessionMiddleware_PersistsValuesAcrossRequests(t *testing.T) {
	store := NewInMemoryStore()
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   store,
		TTL:     time.Hour,
	}))
	r.GET("/set", func(c router.Context) error {
		s, _ := FromContext(c)
		s.Set("hello", "world")
		return c.String(http.StatusOK, "ok")
	})
	r.GET("/get", func(c router.Context) error {
		s, _ := FromContext(c)
		val, ok := s.Get("hello")
		if !ok || val != "world" {
			t.Fatalf("expected persisted session value")
		}
		return c.String(http.StatusOK, val)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/set", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	cookie := findLastCookieByName(w1.Result(), "sid")
	if cookie == nil {
		t.Fatalf("expected sid cookie in first response")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/get", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}

func TestSessionMiddleware_RenewRotatesSessionID(t *testing.T) {
	store := NewInMemoryStore()
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   store,
		TTL:     time.Hour,
	}))
	r.GET("/renew", func(c router.Context) error {
		s, _ := FromContext(c)
		s.Renew()
		s.Set("k", "v")
		return nil
	})

	req1 := httptest.NewRequest(http.MethodGet, "/renew", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	firstCookie := findLastCookieByName(w1.Result(), "sid")
	if firstCookie == nil {
		t.Fatalf("expected sid cookie in first renew response")
	}
	first := firstCookie.Value

	req2 := httptest.NewRequest(http.MethodGet, "/renew", nil)
	req2.AddCookie(firstCookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	secondCookie := findLastCookieByName(w2.Result(), "sid")
	if secondCookie == nil {
		t.Fatalf("expected sid cookie in second renew response")
	}
	second := secondCookie.Value

	if first == second {
		t.Fatalf("expected renewed session id")
	}
}

func TestSessionMiddleware_DestroyClearsCookie(t *testing.T) {
	store := NewInMemoryStore()
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   store,
		TTL:     time.Hour,
	}))
	r.GET("/destroy", func(c router.Context) error {
		s, _ := FromContext(c)
		s.Destroy()
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/destroy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	cookie := findLastCookieByName(w.Result(), "sid")
	if cookie == nil {
		t.Fatalf("expected sid cookie on destroy response")
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected clearing cookie with MaxAge -1, got %d", cookie.MaxAge)
	}
}

func TestOAuthTokenHelpers(t *testing.T) {
	store := NewInMemoryStore()
	r := nethttp.NewRouter()
	r.Use(Middleware(Config{
		Enabled: true,
		Store:   store,
		TTL:     time.Hour,
	}))
	r.GET("/tokens", func(c router.Context) error {
		if err := SetOAuthTokens(c, "access", "refresh"); err != nil {
			t.Fatalf("set tokens: %v", err)
		}
		access, refresh, ok := GetOAuthTokens(c)
		if !ok || access != "access" || refresh != "refresh" {
			t.Fatalf("unexpected oauth token values")
		}
		if err := ClearOAuthTokens(c); err != nil {
			t.Fatalf("clear tokens: %v", err)
		}
		access, refresh, _ = GetOAuthTokens(c)
		if access != "" || refresh != "" {
			t.Fatalf("expected cleared tokens")
		}
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
