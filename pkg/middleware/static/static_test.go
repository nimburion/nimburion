package static

import (
	"bytes"
	"embed"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
)

//go:embed testdata/*
var embeddedAssets embed.FS

func TestServe_ServesLocalFile(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello nimburion")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), content, 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	middleware := Serve("/static", LocalFile(dir, false))
	req := httptest.NewRequest(http.MethodGet, "/static/hello.txt", nil)
	ctx := newMockContext(req)

	nextCalled := false
	handler := middleware(func(c router.Context) error {
		nextCalled = true
		return nil
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if nextCalled {
		t.Fatalf("next handler should not be called when static file exists")
	}
	if !ctx.response.written {
		t.Fatalf("expected static file to write a response")
	}
	if ctx.response.body.String() != string(content) {
		t.Fatalf("unexpected body: %q", ctx.response.body.String())
	}
	if ctx.response.statusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", ctx.response.statusCode)
	}
}

func TestServe_CallsNextWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	middleware := Serve("/static", LocalFile(dir, false))
	req := httptest.NewRequest(http.MethodGet, "/static/missing.txt", nil)
	ctx := newMockContext(req)

	nextCalled := false
	handler := middleware(func(c router.Context) error {
		nextCalled = true
		return nil
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if !nextCalled {
		t.Fatalf("next handler should be called when static file is missing")
	}
	if ctx.response.written {
		t.Fatalf("unexpected response body when file missing")
	}
}

func TestServe_PrefixMismatchDoesNotServe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "staticfile"), []byte("not served"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	middleware := Serve("/static", LocalFile(dir, false))
	req := httptest.NewRequest(http.MethodGet, "/staticfile", nil)
	ctx := newMockContext(req)

	nextCalled := false
	handler := middleware(func(c router.Context) error {
		nextCalled = true
		return nil
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if !nextCalled {
		t.Fatalf("next handler should run when prefix does not match")
	}
	if ctx.response.written {
		t.Fatalf("static middleware should not write when prefix mismatch")
	}
}

func TestServe_WithEmbedFolder(t *testing.T) {
	fs, err := EmbedFolder(embeddedAssets, "testdata")
	if err != nil {
		t.Fatalf("EmbedFolder returned error: %v", err)
	}

	middleware := Serve("/assets", fs)
	req := httptest.NewRequest(http.MethodGet, "/assets/hello.txt", nil)
	ctx := newMockContext(req)

	nextCalled := false
	handler := middleware(func(c router.Context) error {
		nextCalled = true
		return nil
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if nextCalled {
		t.Fatalf("next handler should not be invoked when embed asset exists")
	}
	body := strings.TrimSpace(ctx.response.body.String())
	if body != "embedded asset" {
		t.Fatalf("unexpected embed response body: %q", body)
	}
	if ctx.response.statusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", ctx.response.statusCode)
	}
}

func newMockContext(req *http.Request) *mockContext {
	return &mockContext{
		request: req,
		response: &mockResponseWriter{
			header: make(http.Header),
		},
		data: make(map[string]interface{}),
	}
}

func (m *mockContext) Request() *http.Request {
	return m.request
}

func (m *mockContext) SetRequest(r *http.Request) {
	m.request = r
}

func (m *mockContext) Response() router.ResponseWriter {
	return m.response
}

func (m *mockContext) SetResponse(w router.ResponseWriter) {
	if rw, ok := w.(*mockResponseWriter); ok {
		m.response = rw
	}
}

func (m *mockContext) Param(name string) string {
	return ""
}

func (m *mockContext) Query(name string) string {
	return m.request.URL.Query().Get(name)
}

func (m *mockContext) Bind(v interface{}) error {
	return nil
}

func (m *mockContext) JSON(code int, v interface{}) error {
	m.response.statusCode = code
	m.response.written = true
	return nil
}

func (m *mockContext) String(code int, s string) error {
	m.response.statusCode = code
	m.response.written = true
	return nil
}

func (m *mockContext) Get(key string) interface{} {
	return m.data[key]
}

func (m *mockContext) Set(key string, value interface{}) {
	m.data[key] = value
}

type mockContext struct {
	request  *http.Request
	response *mockResponseWriter
	data     map[string]interface{}
}

type mockResponseWriter struct {
	header     http.Header
	statusCode int
	written    bool
	body       bytes.Buffer
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	if m.statusCode == 0 {
		m.statusCode = http.StatusOK
	}
	m.written = true
	return m.body.Write(b)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	m.written = true
}

func (m *mockResponseWriter) Status() int {
	return m.statusCode
}

func (m *mockResponseWriter) Written() bool {
	return m.written
}
