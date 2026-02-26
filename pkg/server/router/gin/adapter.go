// Package gin provides a gin-gonic based implementation of the router.Router interface.
package gin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	ginpkg "github.com/gin-gonic/gin"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// GinRouter implements router.Router using gin-gonic/gin.
type GinRouter struct {
	engine            *ginpkg.Engine
	group             *ginpkg.RouterGroup
	middleware        []router.MiddlewareFunc
	mu                *sync.RWMutex
	optionsRegistered *map[string]struct{}
}

// NewRouter creates a new GinRouter.
func NewRouter() *GinRouter {
	ginpkg.SetMode(ginpkg.ReleaseMode)
	engine := ginpkg.New()
	optionsRegistered := make(map[string]struct{})
	return &GinRouter{
		engine:            engine,
		mu:                &sync.RWMutex{},
		optionsRegistered: &optionsRegistered,
	}
}

// GET registers a handler for HTTP GET requests at the specified path.
func (r *GinRouter) GET(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodGet, path, handler, middleware)
}

// POST registers a handler for HTTP POST requests at the specified path.
func (r *GinRouter) POST(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodPost, path, handler, middleware)
}

// PUT registers a handler for HTTP PUT requests at the specified path.
func (r *GinRouter) PUT(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodPut, path, handler, middleware)
}

// DELETE registers a handler for HTTP DELETE requests at the specified path.
func (r *GinRouter) DELETE(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodDelete, path, handler, middleware)
}

// PATCH registers a handler for HTTP PATCH requests at the specified path.
func (r *GinRouter) PATCH(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.handle(http.MethodPatch, path, handler, middleware)
}

// Group creates a route group with common prefix and middleware.
func (r *GinRouter) Group(prefix string, middleware ...router.MiddlewareFunc) router.Router {
	r.mu.RLock()
	combined := append([]router.MiddlewareFunc{}, r.middleware...)
	r.mu.RUnlock()
	combined = append(combined, middleware...)

	var group *ginpkg.RouterGroup
	if r.group == nil {
		group = r.engine.Group(prefix)
	} else {
		group = r.group.Group(prefix)
	}

	return &GinRouter{
		engine:            r.engine,
		group:             group,
		middleware:        combined,
		mu:                r.mu,
		optionsRegistered: r.optionsRegistered,
	}
}

// Use applies middleware to all routes.
func (r *GinRouter) Use(middleware ...router.MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware...)
}

// ServeHTTP implements http.Handler.
func (r *GinRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}

func (r *GinRouter) handle(method, path string, h router.HandlerFunc, routeMiddleware []router.MiddlewareFunc) {
	r.mu.RLock()
	global := append([]router.MiddlewareFunc{}, r.middleware...)
	r.mu.RUnlock()

	ginHandler := func(gc *ginpkg.Context) {
		ctx := newContext(gc)
		handler := h

		for i := len(routeMiddleware) - 1; i >= 0; i-- {
			handler = routeMiddleware[i](handler)
		}
		for i := len(global) - 1; i >= 0; i-- {
			handler = global[i](handler)
		}

		if err := handler(ctx); err != nil && !ctx.Response().Written() {
			gc.AbortWithStatus(http.StatusInternalServerError)
		}
	}

	if r.group != nil {
		r.group.Handle(method, path, ginHandler)
		r.ensureOptionsRoute(path)
		return
	}
	r.engine.Handle(method, path, ginHandler)
	r.ensureOptionsRoute(path)
}

func (r *GinRouter) ensureOptionsRoute(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := path
	if r.group != nil {
		key = r.group.BasePath() + path
	}
	if _, exists := (*r.optionsRegistered)[key]; exists {
		return
	}
	(*r.optionsRegistered)[key] = struct{}{}

	optionsHandler := func(gc *ginpkg.Context) {
		ctx := newContext(gc)
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
	}

	if r.group != nil {
		r.group.Handle(http.MethodOptions, path, optionsHandler)
		return
	}
	r.engine.Handle(http.MethodOptions, path, optionsHandler)
}

// ginContext adapts gin.Context to router.Context.
type ginContext struct {
	ctx      *ginpkg.Context
	response router.ResponseWriter
}

func newContext(c *ginpkg.Context) *ginContext {
	return &ginContext{ctx: c, response: &ginResponseWriter{ResponseWriter: c.Writer}}
}

// Request returns the underlying HTTP request being processed.
func (c *ginContext) Request() *http.Request {
	return c.ctx.Request
}

// SetRequest updates the HTTP request associated with this context.
func (c *ginContext) SetRequest(r *http.Request) {
	c.ctx.Request = r
}

// Response returns the response writer for sending HTTP responses.
func (c *ginContext) Response() router.ResponseWriter {
	return c.response
}

// SetResponse updates the response writer associated with this context.
func (c *ginContext) SetResponse(w router.ResponseWriter) {
	c.response = w
}

// Param retrieves a URL path parameter by name.
func (c *ginContext) Param(name string) string {
	return c.ctx.Param(name)
}

// Query retrieves a URL query parameter by name.
func (c *ginContext) Query(name string) string {
	return c.ctx.Query(name)
}

// Bind deserializes the request body into the provided struct based on Content-Type.
func (c *ginContext) Bind(v interface{}) error {
	if c.ctx.Request.Body == nil || c.ctx.Request.Body == http.NoBody {
		return fmt.Errorf("request body is empty")
	}
	defer c.ctx.Request.Body.Close()

	contentType := c.ctx.GetHeader("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}

	return json.NewDecoder(c.ctx.Request.Body).Decode(v)
}

// JSON serializes the given value as JSON and writes it to the response with the specified status code.
func (c *ginContext) JSON(code int, v interface{}) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(code)
	return json.NewEncoder(c.response).Encode(v)
}

// String writes a plain text response with the specified status code.
func (c *ginContext) String(code int, s string) error {
	c.response.Header().Set("Content-Type", "text/plain")
	c.response.WriteHeader(code)
	_, err := c.response.Write([]byte(s))
	return err
}

// Get retrieves a value from the context by key.
func (c *ginContext) Get(key string) interface{} {
	v, ok := c.ctx.Get(key)
	if !ok {
		return nil
	}
	return v
}

// Set stores a value in the context with the given key.
func (c *ginContext) Set(key string, value interface{}) {
	c.ctx.Set(key, value)
}

// ginResponseWriter wraps gin.ResponseWriter to satisfy router.ResponseWriter.
type ginResponseWriter struct {
	ginpkg.ResponseWriter
	mu      sync.RWMutex
	status  int
	written bool
}

// Status returns the HTTP status code that was written, or 0 if not yet written.
func (w *ginResponseWriter) Status() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Written returns true if the response headers and body have been written.
func (w *ginResponseWriter) Written() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.written
}

// WriteHeader sends an HTTP response header with the provided status code.
func (w *ginResponseWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.written {
		return
	}
	w.status = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

// Write writes data to the response body. Implements io.Writer interface.
func (w *ginResponseWriter) Write(b []byte) (int, error) {
	if !w.Written() {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush sends any buffered data to the client immediately.
func (w *ginResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
