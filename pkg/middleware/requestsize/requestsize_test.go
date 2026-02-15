package requestsize

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_AllowsRequestWithinLimit(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(64))

	r.POST("/items", func(c router.Context) error {
		var payload map[string]interface{}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestMiddleware_RejectsWhenContentLengthExceedsLimit(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(8))

	called := false
	r.POST("/items", func(c router.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"too-long"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be called for oversized requests")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "request_too_large") {
		t.Fatalf("expected request_too_large error payload, got %s", rec.Body.String())
	}
}

func TestMiddleware_TransformsMaxBytesErrorTo413(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(16))

	r.POST("/items", func(c router.Context) error {
		var payload map[string]interface{}
		if err := c.Bind(&payload); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	body := bytes.NewBufferString(`{"name":"012345678901234567890123456789"}`)
	req := httptest.NewRequest(http.MethodPost, "/items", body)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
}

func TestMiddleware_DisabledForNonPositiveLimit(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(0))

	r.POST("/items", func(c router.Context) error {
		var payload map[string]interface{}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"01234567890123456789"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
