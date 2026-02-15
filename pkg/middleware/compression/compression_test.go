package compression

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMiddleware_UsesBrotliWhenAccepted(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(DefaultConfig()))

	r.GET("/items", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"name": "value"})
	})

	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("expected br encoding, got %q", rec.Header().Get("Content-Encoding"))
	}

	reader := brotli.NewReader(bytes.NewReader(rec.Body.Bytes()))
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decode br body: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode json payload: %v", err)
	}
	if payload["name"] != "value" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestMiddleware_FallsBackToGzip(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(DefaultConfig()))

	r.GET("/text", func(c router.Context) error {
		return c.String(http.StatusOK, "compressed-response")
	})

	req := httptest.NewRequest(http.MethodGet, "/text", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding, got %q", rec.Header().Get("Content-Encoding"))
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	decoded, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to decode gzip body: %v", err)
	}
	if string(decoded) != "compressed-response" {
		t.Fatalf("unexpected decoded body %q", string(decoded))
	}
}

func TestMiddleware_SkipsWhenNotAccepted(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(DefaultConfig()))

	r.GET("/plain", func(c router.Context) error {
		return c.String(http.StatusOK, "plain")
	})

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no content encoding, got %q", rec.Header().Get("Content-Encoding"))
	}
	if rec.Body.String() != "plain" {
		t.Fatalf("expected plain body, got %q", rec.Body.String())
	}
}

func TestMiddleware_RespectsExistingContentEncoding(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(DefaultConfig()))

	r.GET("/pre-encoded", func(c router.Context) error {
		c.Response().Header().Set("Content-Encoding", "identity")
		return c.String(http.StatusOK, "body")
	})

	req := httptest.NewRequest(http.MethodGet, "/pre-encoded", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "identity" {
		t.Fatalf("expected original content encoding to remain, got %q", rec.Header().Get("Content-Encoding"))
	}
	if rec.Body.String() != "body" {
		t.Fatalf("expected uncompressed body, got %q", rec.Body.String())
	}
}

func TestMiddleware_SkipsNoBodyStatuses(t *testing.T) {
	r := nethttp.NewRouter()
	r.Use(Middleware(DefaultConfig()))

	r.GET("/empty", func(c router.Context) error {
		return c.String(http.StatusNoContent, "")
	})

	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no content encoding for 204, got %q", rec.Header().Get("Content-Encoding"))
	}
}

func TestMiddleware_ExcludedPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExcludedPathPrefixes = []string{"/metrics"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	r.GET("/metrics", func(c router.Context) error {
		return c.String(http.StatusOK, "metrics-data")
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no encoding for excluded path, got %q", rec.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(rec.Body.String(), "metrics-data") {
		t.Fatalf("expected original body, got %q", rec.Body.String())
	}
}

func TestMiddleware_ExcludedPathRegex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExcludedPathRegex = []string{`^/assets/.*$`}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	r.GET("/assets/main.js", func(c router.Context) error {
		return c.String(http.StatusOK, "asset")
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/main.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no encoding for regex-excluded path, got %q", rec.Header().Get("Content-Encoding"))
	}
}

func TestMiddleware_ExcludedExtension(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExcludedExtensions = []string{".jpg", "png"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	r.GET("/image/photo.jpg", func(c router.Context) error {
		c.Response().Header().Set("Content-Type", "image/jpeg")
		return c.String(http.StatusOK, strings.Repeat("x", 500))
	})

	req := httptest.NewRequest(http.MethodGet, "/image/photo.jpg", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no encoding for excluded extension, got %q", rec.Header().Get("Content-Encoding"))
	}
}

func TestMiddleware_ExcludedMIMEType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExcludedMIMETypes = []string{"image/*", "application/pdf"}

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	r.GET("/img", func(c router.Context) error {
		c.Response().Header().Set("Content-Type", "image/png")
		c.Response().WriteHeader(http.StatusOK)
		_, err := c.Response().Write([]byte(strings.Repeat("a", 500)))
		return err
	})

	req := httptest.NewRequest(http.MethodGet, "/img", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no encoding for excluded mime type, got %q", rec.Header().Get("Content-Encoding"))
	}
}

func TestMiddleware_MinSizeAcrossMultipleWrites(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinSize = 10

	r := nethttp.NewRouter()
	r.Use(Middleware(cfg))

	r.GET("/chunked", func(c router.Context) error {
		c.Response().Header().Set("Content-Type", "text/plain")
		if _, err := c.Response().Write([]byte("12345")); err != nil {
			return err
		}
		_, err := c.Response().Write([]byte("67890"))
		return err
	})

	req := httptest.NewRequest(http.MethodGet, "/chunked", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip encoding after reaching min size, got %q", rec.Header().Get("Content-Encoding"))
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()
	decoded, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to decode gzip body: %v", err)
	}
	if string(decoded) != "1234567890" {
		t.Fatalf("unexpected decoded body %q", string(decoded))
	}
}

func TestMiddleware_ExcludeServerPush(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ExcludeServerPush = true

	mw := Middleware(cfg)
	req := httptest.NewRequest(http.MethodGet, "/push", nil)
	ctx := &pushMockContext{
		req: req,
		res: &pushMockResponseWriter{
			header: make(http.Header),
		},
	}

	handler := mw(func(c router.Context) error {
		c.Response().Header().Set("Content-Type", "text/plain")
		_, err := c.Response().Write([]byte(strings.Repeat("x", 300)))
		return err
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if ctx.res.Header().Get("Content-Encoding") != "" {
		t.Fatalf("expected no encoding when server push exclusion is enabled, got %q", ctx.res.Header().Get("Content-Encoding"))
	}
}

type pushMockContext struct {
	req *http.Request
	res *pushMockResponseWriter
}

func (m *pushMockContext) Request() *http.Request              { return m.req }
func (m *pushMockContext) SetRequest(r *http.Request)          { m.req = r }
func (m *pushMockContext) Response() router.ResponseWriter     { return m.res }
func (m *pushMockContext) SetResponse(w router.ResponseWriter) { m.res = w.(*pushMockResponseWriter) }
func (m *pushMockContext) Param(string) string                 { return "" }
func (m *pushMockContext) Query(string) string                 { return "" }
func (m *pushMockContext) Bind(v interface{}) error            { _ = v; return nil }
func (m *pushMockContext) JSON(int, interface{}) error         { return nil }
func (m *pushMockContext) String(int, string) error            { return nil }
func (m *pushMockContext) Get(string) interface{}              { return nil }
func (m *pushMockContext) Set(string, interface{})             {}

type pushMockResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (m *pushMockResponseWriter) Header() http.Header { return m.header }
func (m *pushMockResponseWriter) Write(p []byte) (int, error) {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	return m.body.Write(p)
}
func (m *pushMockResponseWriter) WriteHeader(code int) {
	m.status = code
}
func (m *pushMockResponseWriter) Status() int {
	if m.status == 0 {
		return http.StatusOK
	}
	return m.status
}
func (m *pushMockResponseWriter) Written() bool {
	return m.status != 0 || m.body.Len() > 0
}
func (m *pushMockResponseWriter) Push(target string, opts *http.PushOptions) error {
	_ = target
	_ = opts
	return nil
}
func (m *pushMockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, http.ErrNotSupported
}
