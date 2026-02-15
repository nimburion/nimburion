package openapivalidation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/routers"
	logpkg "github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
)

func TestSnapshotAndRestoreRequestBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString("payload"))
	body, err := snapshotAndRestoreRequestBody(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "payload" {
		t.Fatalf("unexpected body: %s", string(body))
	}

	buf := make([]byte, 7)
	if _, err := req.Body.Read(buf); err != nil {
		t.Fatalf("expected body to be restored: %v", err)
	}
}

func TestCloneRequestForValidation(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/users", nil)
	clone := cloneRequestForValidation(req, nil, "/api")
	if clone.URL.Path != "/users" {
		t.Fatalf("unexpected cloned path: %s", clone.URL.Path)
	}
	if clone.Body != http.NoBody {
		t.Fatalf("expected NoBody when no payload")
	}
}

func TestOpenAPIValidationStatusCode(t *testing.T) {
	if code := openapiValidationStatusCode(nil); code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
	if code := openapiValidationStatusCode(routers.ErrMethodNotAllowed); code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", code)
	}
}

type testLogger struct{}

func (l testLogger) Debug(msg string, args ...any)  {}
func (l testLogger) Info(msg string, args ...any)   {}
func (l testLogger) Warn(msg string, args ...any)   {}
func (l testLogger) Error(msg string, args ...any)  {}
func (l testLogger) With(args ...any) logpkg.Logger { return l }
func (l testLogger) WithContext(ctx context.Context) logpkg.Logger {
	return l
}

type openapiTestResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w openapiTestResponseWriter) Status() int   { return w.Code }
func (w openapiTestResponseWriter) Written() bool { return w.Code != 0 }

type openapiTestContext struct {
	req  *http.Request
	resp openapiTestResponseWriter
}

func newOpenAPITestContext(req *http.Request) *openapiTestContext {
	return &openapiTestContext{req: req, resp: openapiTestResponseWriter{httptest.NewRecorder()}}
}

func (c *openapiTestContext) Request() *http.Request          { return c.req }
func (c *openapiTestContext) SetRequest(r *http.Request)      { c.req = r }
func (c *openapiTestContext) Response() router.ResponseWriter { return c.resp }
func (c *openapiTestContext) SetResponse(w router.ResponseWriter) {
	c.resp = w.(openapiTestResponseWriter)
}
func (c *openapiTestContext) Param(name string) string          { return "" }
func (c *openapiTestContext) Query(name string) string          { return "" }
func (c *openapiTestContext) Bind(v interface{}) error          { return nil }
func (c *openapiTestContext) Get(key string) interface{}        { return nil }
func (c *openapiTestContext) Set(key string, value interface{}) {}
func (c *openapiTestContext) JSON(code int, v interface{}) error {
	c.resp.WriteHeader(code)
	return json.NewEncoder(c.resp).Encode(v)
}
func (c *openapiTestContext) String(code int, s string) error {
	c.resp.WriteHeader(code)
	_, err := c.resp.Write([]byte(s))
	return err
}

func TestOpenAPIRequestValidationStrictAndWarnOnly(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.yaml")
	spec := `openapi: 3.0.3
info:
  title: test
  version: "1.0"
paths:
  /items:
    get:
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	nextCalled := false
	next := func(c router.Context) error {
		nextCalled = true
		return nil
	}

	strictMw, err := NewRequestValidationMiddleware(Config{
		SpecPath: specPath,
		Mode:     ValidationModeStrict,
	}, testLogger{})
	if err != nil {
		t.Fatalf("create strict middleware: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	ctx := newOpenAPITestContext(req)
	if err := strictMw(next)(ctx); err != nil {
		t.Fatalf("strict middleware error: %v", err)
	}
	if ctx.resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 in strict mode, got %d", ctx.resp.Code)
	}
	if nextCalled {
		t.Fatalf("next should not be called in strict mode on invalid request")
	}

	nextCalled = false
	warnMw, err := NewRequestValidationMiddleware(Config{
		SpecPath: specPath,
		Mode:     ValidationModeWarnOnly,
	}, testLogger{})
	if err != nil {
		t.Fatalf("create warn-only middleware: %v", err)
	}
	ctx = newOpenAPITestContext(httptest.NewRequest(http.MethodPost, "/items", nil))
	if err := warnMw(next)(ctx); err != nil {
		t.Fatalf("warn-only middleware error: %v", err)
	}
	if !nextCalled {
		t.Fatalf("next should be called in warn-only mode")
	}
}

func TestOpenAPIRequestValidationStripPrefix(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.yaml")
	spec := `openapi: 3.0.3
info:
  title: test
  version: "1.0"
paths:
  /items:
    get:
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	nextCalled := false
	next := func(c router.Context) error {
		nextCalled = true
		return nil
	}

	mw, err := NewRequestValidationMiddleware(Config{
		SpecPath:    specPath,
		StripPrefix: "/api",
		Mode:        ValidationModeStrict,
	}, testLogger{})
	if err != nil {
		t.Fatalf("create middleware: %v", err)
	}

	ctx := newOpenAPITestContext(httptest.NewRequest(http.MethodGet, "/api/items", nil))
	if err := mw(next)(ctx); err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	if !nextCalled {
		t.Fatalf("expected next to be called")
	}
}
