// Package nethttp provides a net/http-based implementation of the router.Router interface.
package nethttp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// NetHTTPRouter implements router.Router using net/http and a simple pattern matcher.
type NetHTTPRouter struct {
	routes            *[]route
	middleware        []router.MiddlewareFunc
	prefix            string
	mu                *sync.RWMutex
	optionsRegistered *map[string]struct{}
}

type route struct {
	method     string
	pattern    string
	handler    router.HandlerFunc
	middleware []router.MiddlewareFunc
}

// NewRouter creates a new NetHTTPRouter.
func NewRouter() *NetHTTPRouter {
	routes := make([]route, 0)
	optionsRegistered := make(map[string]struct{})
	mu := &sync.RWMutex{}
	return &NetHTTPRouter{
		routes:            &routes,
		mu:                mu,
		optionsRegistered: &optionsRegistered,
	}
}

// GET registers a GET route.
func (r *NetHTTPRouter) GET(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.addRoute(http.MethodGet, path, handler, middleware)
}

// POST registers a POST route.
func (r *NetHTTPRouter) POST(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.addRoute(http.MethodPost, path, handler, middleware)
}

// PUT registers a PUT route.
func (r *NetHTTPRouter) PUT(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.addRoute(http.MethodPut, path, handler, middleware)
}

// DELETE registers a DELETE route.
func (r *NetHTTPRouter) DELETE(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.addRoute(http.MethodDelete, path, handler, middleware)
}

// PATCH registers a PATCH route.
func (r *NetHTTPRouter) PATCH(path string, handler router.HandlerFunc, middleware ...router.MiddlewareFunc) {
	r.addRoute(http.MethodPatch, path, handler, middleware)
}

// Group creates a route group with common prefix and middleware.
func (r *NetHTTPRouter) Group(prefix string, middleware ...router.MiddlewareFunc) router.Router {
	return &NetHTTPRouter{
		routes:            r.routes,
		middleware:        append(r.middleware, middleware...),
		prefix:            r.prefix + prefix,
		mu:                r.mu,
		optionsRegistered: r.optionsRegistered,
	}
}

// Use applies middleware to all routes.
func (r *NetHTTPRouter) Use(middleware ...router.MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware...)
}

// ServeHTTP implements http.Handler.
func (r *NetHTTPRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find matching route
	for _, rt := range *r.routes {
		params, ok := matchRoute(rt.pattern, req.URL.Path)
		if !ok || rt.method != req.Method {
			continue
		}

		// Create context
		ctx := newContext(w, req, params)

		// Build middleware chain
		handler := rt.handler

		// Apply route middleware (global/group/route-specific snapshot, in reverse order)
		for i := len(rt.middleware) - 1; i >= 0; i-- {
			handler = rt.middleware[i](handler)
		}

		// Execute handler
		if err := handler(ctx); err != nil {
			// If handler returns an error and hasn't written a response, write error
			if !ctx.Response().Written() {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		return
	}

	// No route found
	http.NotFound(w, req)
}

func (r *NetHTTPRouter) addRoute(method, path string, handler router.HandlerFunc, middleware []router.MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fullPath := r.prefix + path
	baseMiddleware := append([]router.MiddlewareFunc{}, r.middleware...)
	allMiddleware := append([]router.MiddlewareFunc{}, baseMiddleware...)
	allMiddleware = append(allMiddleware, middleware...)

	*r.routes = append(*r.routes, route{
		method:     method,
		pattern:    fullPath,
		handler:    handler,
		middleware: allMiddleware,
	})

	r.ensureOptionsRouteLocked(fullPath, baseMiddleware)
}

func (r *NetHTTPRouter) ensureOptionsRouteLocked(path string, middleware []router.MiddlewareFunc) {
	if _, exists := (*r.optionsRegistered)[path]; exists {
		return
	}
	(*r.optionsRegistered)[path] = struct{}{}

	*r.routes = append(*r.routes, route{
		method:  http.MethodOptions,
		pattern: path,
		handler: func(c router.Context) error {
			if !c.Response().Written() {
				c.Response().WriteHeader(http.StatusNoContent)
			}
			return nil
		},
		middleware: middleware,
	})
}

// matchRoute checks if a pattern matches a path and extracts parameters.
// Supports patterns like /users/:id/posts/:postId
func matchRoute(pattern, path string) (map[string]string, bool) {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil, false
	}

	params := make(map[string]string)
	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") {
			// This is a parameter
			paramName := part[1:]
			params[paramName] = pathParts[i]
		} else if part != pathParts[i] {
			// Static part doesn't match
			return nil, false
		}
	}

	return params, true
}

// netHTTPContext implements router.Context.
type netHTTPContext struct {
	request  *http.Request
	response router.ResponseWriter
	params   map[string]string
	store    map[string]interface{}
	mu       sync.RWMutex
}

func newContext(w http.ResponseWriter, r *http.Request, params map[string]string) *netHTTPContext {
	return &netHTTPContext{
		request:  r,
		response: &responseWriter{ResponseWriter: w},
		params:   params,
		store:    make(map[string]interface{}),
	}
}

func (c *netHTTPContext) Request() *http.Request {
	return c.request
}

func (c *netHTTPContext) SetRequest(r *http.Request) {
	c.request = r
}

func (c *netHTTPContext) Response() router.ResponseWriter {
	return c.response
}

func (c *netHTTPContext) SetResponse(w router.ResponseWriter) {
	c.response = w
}

func (c *netHTTPContext) Param(name string) string {
	return c.params[name]
}

func (c *netHTTPContext) Query(name string) string {
	return c.request.URL.Query().Get(name)
}

func (c *netHTTPContext) Bind(v interface{}) error {
	if c.request.Body == nil {
		return fmt.Errorf("request body is empty")
	}

	defer c.request.Body.Close()

	contentType := c.request.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		decoder := json.NewDecoder(c.request.Body)
		return decoder.Decode(v)
	}

	return fmt.Errorf("unsupported content type: %s", contentType)
}

func (c *netHTTPContext) JSON(code int, v interface{}) error {
	c.response.Header().Set("Content-Type", "application/json")
	c.response.WriteHeader(code)

	encoder := json.NewEncoder(c.response)
	return encoder.Encode(v)
}

func (c *netHTTPContext) String(code int, s string) error {
	c.response.Header().Set("Content-Type", "text/plain")
	c.response.WriteHeader(code)

	_, err := io.WriteString(c.response, s)
	return err
}

func (c *netHTTPContext) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store[key]
}

func (c *netHTTPContext) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

// responseWriter wraps http.ResponseWriter to track status and written state.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *responseWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *responseWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}

func (w *responseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *responseWriter) Written() bool {
	return w.written
}
