package httpsignature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_ValidRequest(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeyProvider = StaticKeyProvider{"device-1": "secret-1"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.POST("/hook", func(c router.Context) error {
		if got := c.Get(AuthKeyIDContextKey); got != "device-1" {
			t.Fatalf("expected key id in context, got %v", got)
		}
		return c.String(http.StatusOK, "ok")
	})

	body := `{"value":42}`
	req := httptest.NewRequest(http.MethodPost, "/hook?type=test", strings.NewReader(body))
	applySignedHeaders(req, cfg, "device-1", "secret-1", body, time.Now().UTC(), "nonce-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_InvalidSignature(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeyProvider = StaticKeyProvider{"device-1": "secret-1"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.POST("/hook", func(c router.Context) error { return c.String(http.StatusOK, "ok") })

	body := `{"value":42}`
	req := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	applySignedHeaders(req, cfg, "device-1", "wrong-secret", body, time.Now().UTC(), "nonce-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_RejectsReplayNonce(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeyProvider = StaticKeyProvider{"device-1": "secret-1"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.POST("/hook", func(c router.Context) error { return c.String(http.StatusOK, "ok") })

	nonce := "nonce-dup"
	body := `{"value":42}`
	req1 := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	applySignedHeaders(req1, cfg, "device-1", "secret-1", body, time.Now().UTC(), nonce)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	applySignedHeaders(req2, cfg, "device-1", "secret-1", body, time.Now().UTC(), nonce)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected replay request 401, got %d", rec2.Code)
	}
}

func TestMiddleware_RejectsExpiredTimestamp(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeyProvider = StaticKeyProvider{"device-1": "secret-1"}
	cfg.MaxClockSkew = 2 * time.Minute

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.POST("/hook", func(c router.Context) error { return c.String(http.StatusOK, "ok") })

	body := `{"value":42}`
	req := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	applySignedHeaders(req, cfg, "device-1", "secret-1", body, time.Now().UTC().Add(-10*time.Minute), "nonce-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_ExcludedPathBypassesValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeyProvider = StaticKeyProvider{"device-1": "secret-1"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.GET("/health/live", func(c router.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for excluded path, got %d", rec.Code)
	}
}

func TestMiddleware_PreservesRequestBody(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeyProvider = StaticKeyProvider{"device-1": "secret-1"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))
	r.POST("/hook", func(c router.Context) error {
		raw, err := io.ReadAll(c.Request().Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return c.String(http.StatusOK, string(raw))
	})

	body := `{"value":42}`
	req := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	applySignedHeaders(req, cfg, "device-1", "secret-1", body, time.Now().UTC(), "nonce-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != body {
		t.Fatalf("expected echoed body %q, got %q", body, rec.Body.String())
	}
}

func applySignedHeaders(req *http.Request, cfg Config, keyID string, secret string, body string, ts time.Time, nonce string) {
	timestamp := ts.UTC().Format(time.RFC3339)
	payload := canonicalPayload(req.Method, req.URL.EscapedPath(), req.URL.RawQuery, timestamp, nonce, []byte(body))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set(cfg.KeyIDHeader, keyID)
	req.Header.Set(cfg.TimestampHeader, timestamp)
	req.Header.Set(cfg.NonceHeader, nonce)
	req.Header.Set(cfg.SignatureHeader, signature)
}

func canonicalPayload(method, path, query, timestamp, nonce string, body []byte) string {
	requestURI := path
	if query != "" {
		requestURI += "?" + query
	}
	sum := sha256.Sum256(body)
	return strings.Join([]string{
		strings.ToUpper(method),
		requestURI,
		timestamp,
		nonce,
		hex.EncodeToString(sum[:]),
	}, "\n")
}
