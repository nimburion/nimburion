package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/middleware/session"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func setupCSRFRouter(t *testing.T) *nethttp.NetHTTPRouter {
	t.Helper()
	r := nethttp.NewRouter()
	r.Use(session.Middleware(session.Config{
		Enabled: true,
		Store:   session.NewInMemoryStore(),
		TTL:     time.Hour,
	}))
	r.Use(Middleware(Config{
		Enabled:      true,
		CookieSecure: false,
	}))
	r.GET("/safe", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	r.POST("/unsafe", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	return r
}

func TestCSRF_SafeMethodIssuesCookie(t *testing.T) {
	r := setupCSRFRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/safe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "XSRF-TOKEN" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected XSRF-TOKEN cookie")
	}
}

func TestCSRF_UnsafeMethodWithoutHeaderBlocked(t *testing.T) {
	r := setupCSRFRouter(t)

	getReq := httptest.NewRequest(http.MethodGet, "/safe", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	cookies := getW.Result().Cookies()

	postReq := httptest.NewRequest(http.MethodPost, "/unsafe", nil)
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", postW.Code)
	}
}

func TestCSRF_UnsafeMethodWithValidHeaderAllowed(t *testing.T) {
	r := setupCSRFRouter(t)

	getReq := httptest.NewRequest(http.MethodGet, "/safe", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	cookies := getW.Result().Cookies()

	var csrfToken string
	for _, c := range cookies {
		if c.Name == "XSRF-TOKEN" {
			csrfToken = c.Value
		}
	}
	if csrfToken == "" {
		t.Fatalf("missing csrf token cookie")
	}

	postReq := httptest.NewRequest(http.MethodPost, "/unsafe", nil)
	for _, c := range cookies {
		postReq.AddCookie(c)
	}
	postReq.Header.Set("X-CSRF-Token", csrfToken)
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", postW.Code)
	}
}
