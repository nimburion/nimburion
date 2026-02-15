// Package gorilla provides a gorilla/mux based implementation of the router.Router interface.
package gorilla

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// GorillaRouter implements router.Router using gorilla/mux.
type GorillaRouter struct {
	router            *mux.Router
	middleware        []router.MiddlewareFunc
	mu                *sync.RWMutex
	optionsRegistered *map[string]struct{}
}

// NewRouter creates a new GorillaRouter.
func NewRouter() *GorillaRouter {
	optionsRegistered := make(map[string]struct{})
	return &GorillaRouter{
		router:            mux.NewRouter(),
		mu:                &sync.RWMutex{},
		optionsRegistered: &optionsRegistered,
	}
}

func (r *GorillaRouter) GET(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodGet, path, handler, middleware)
}

func (r *GorillaRouter) POST(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodPost, path, handler, middleware)
}

func (r *GorillaRouter) PUT(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodPut, path, handler, middleware)
}

func (r *GorillaRouter) DELETE(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodDelete, path, handler, middleware)
}

func (r *GorillaRouter) PATCH(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodPatch, path, handler, middleware)
}

// Group creates a route group with common prefix and middleware.
func (r *GorillaRouter) Group(prefix string, middleware ...router.MiddlewareFunc) router.Router {
	r.mu.RLock()
	combined := append([]router.MiddlewareFunc{}, r.middleware...)
	r.mu.RUnlock()
	combined = append(combined, middleware...)

	return &GorillaRouter{
		router:            r.router.PathPrefix(prefix).Subrouter(),
		middleware:        combined,
		mu:                r.mu,
		optionsRegistered: r.optionsRegistered,
	}
}

// Use applies middleware to all routes.
func (r *GorillaRouter) Use(middleware ...router.MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware...)
}

// ServeHTTP implements http.Handler.
func (r *GorillaRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.router.ServeHTTP(w, req)
}

func (r *GorillaRouter) handle(method, path string, h router.HandlerFunc, routeMiddleware []router.MiddlewareFunc) {
	r.mu.RLock()
	global := append([]router.MiddlewareFunc{}, r.middleware...)
	r.mu.RUnlock()

	muxPath := toMuxPath(path)
	r.router.HandleFunc(muxPath, func(w http.ResponseWriter, req *http.Request) {
		ctx := newContext(w, req)
		handler := h

		for i := len(routeMiddleware) - 1; i >= 0; i-- {
			handler = routeMiddleware[i](handler)
		}
		for i := len(global) - 1; i >= 0; i-- {
			handler = global[i](handler)
		}

		if err := handler(ctx); err != nil && !ctx.Response().Written() {
			http.Error(ctx.Response(), err.Error(), http.StatusInternalServerError)
		}
	}).Methods(method)

	r.ensureOptionsRoute(muxPath)
}

func (r *GorillaRouter) ensureOptionsRoute(muxPath string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := (*r.optionsRegistered)[muxPath]; exists {
		return
	}
	(*r.optionsRegistered)[muxPath] = struct{}{}

	r.router.HandleFunc(muxPath, func(w http.ResponseWriter, req *http.Request) {
		ctx := newContext(w, req)
		handler := func(c router.Context) error {
			if !c.Response().Written() {
				c.Response().WriteHeader(http.StatusNoContent)
			}
			return nil
		}

		for i := len(r.middleware) - 1; i >= 0; i-- {
			handler = r.middleware[i](handler)
		}
		_ = handler(ctx)
	}).Methods(http.MethodOptions)
}

func toMuxPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// gorillaContext adapts mux request/response to router.Context.
type gorillaContext struct {
	request  *http.Request
	response router.ResponseWriter
	store    map[string]interface{}
	mu       sync.RWMutex
}

func newContext(w http.ResponseWriter, r *http.Request) *gorillaContext {
	return &gorillaContext{
		request:  r,
		response: &gorillaResponseWriter{ResponseWriter: w},
		store:    make(map[string]interface{}),
	}
}

func (c *gorillaContext) Request() *http.Request {
	return c.request
}

func (c *gorillaContext) SetRequest(r *http.Request) {
	c.request = r
}

func (c *gorillaContext) Response() router.ResponseWriter {
	return c.response
}

func (c *gorillaContext) SetResponse(w router.ResponseWriter) {
	c.response = w
}

func (c *gorillaContext) Param(name string) string {
	return mux.Vars(c.request)[name]
}

func (c *gorillaContext) Query(name string) string {
	return c.request.URL.Query().Get(name)
}

func (c *gorillaContext) Bind(v interface{}) error {
	if c.request.Body == nil || c.request.Body == http.NoBody {
		return fmt.Errorf("request body is empty")
	}
	defer c.request.Body.Close()

	contentType := c.request.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}

	return json.NewDecoder(c.request.Body).Decode(v)
}

func (c *gorillaContext) JSON(code int, v interface{}) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(code)
	return json.NewEncoder(c.response).Encode(v)
}

func (c *gorillaContext) String(code int, s string) error {
	c.response.Header().Set("Content-Type", "text/plain")
	c.response.WriteHeader(code)
	_, err := io.WriteString(c.response, s)
	return err
}

func (c *gorillaContext) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store[key]
}

func (c *gorillaContext) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

// gorillaResponseWriter wraps http.ResponseWriter and tracks status/written state.
type gorillaResponseWriter struct {
	http.ResponseWriter
	status  int
	written bool
	mu      sync.RWMutex
}

func (w *gorillaResponseWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.written {
		return
	}
	w.status = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *gorillaResponseWriter) Write(b []byte) (int, error) {
	if !w.Written() {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gorillaResponseWriter) Status() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *gorillaResponseWriter) Written() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.written
}

func (w *gorillaResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *gorillaResponseWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}
