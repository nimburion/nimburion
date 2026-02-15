package openapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// mockContext implements router.Context for testing
type mockContext struct {
	request  *http.Request
	response *mockResponseWriter
	params   map[string]string
	values   map[string]interface{}
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
	if response, ok := w.(*mockResponseWriter); ok {
		m.response = response
	}
}

func (m *mockContext) Param(name string) string {
	if m.params == nil {
		return ""
	}
	return m.params[name]
}

func (m *mockContext) Query(name string) string {
	return m.request.URL.Query().Get(name)
}

func (m *mockContext) Bind(v interface{}) error {
	body, err := io.ReadAll(m.request.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func (m *mockContext) JSON(code int, v interface{}) error {
	m.response.WriteHeader(code)
	m.response.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(m.response).Encode(v)
}

func (m *mockContext) String(code int, s string) error {
	m.response.WriteHeader(code)
	m.response.Header().Set("Content-Type", "text/plain")
	_, err := m.response.Write([]byte(s))
	return err
}

func (m *mockContext) Get(key string) interface{} {
	if m.values == nil {
		return nil
	}
	return m.values[key]
}

func (m *mockContext) Set(key string, value interface{}) {
	if m.values == nil {
		m.values = make(map[string]interface{})
	}
	m.values[key] = value
}

// mockResponseWriter implements router.ResponseWriter for testing
type mockResponseWriter struct {
	*httptest.ResponseRecorder
	status  int
	written bool
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.status = code
	m.written = true
	m.ResponseRecorder.WriteHeader(code)
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.written = true
	return m.ResponseRecorder.Write(b)
}

func (m *mockResponseWriter) Status() int {
	if m.status == 0 {
		return http.StatusOK
	}
	return m.status
}

func (m *mockResponseWriter) Written() bool {
	return m.written
}

// mockRouter implements router.Router for testing
type mockRouter struct {
	routes []string
}

func (m *mockRouter) GET(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	m.routes = append(m.routes, path)
}

func (m *mockRouter) POST(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	m.routes = append(m.routes, path)
}

func (m *mockRouter) PUT(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	m.routes = append(m.routes, path)
}

func (m *mockRouter) DELETE(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	m.routes = append(m.routes, path)
}

func (m *mockRouter) PATCH(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	m.routes = append(m.routes, path)
}

func (m *mockRouter) Group(prefix string, middleware ...router.MiddlewareFunc) router.Router {
	return m
}

func (m *mockRouter) Use(middleware ...router.MiddlewareFunc) {
}

func (m *mockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}
