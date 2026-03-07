// Package router provides an abstraction layer for HTTP routing.
// It defines interfaces that allow pluggable router implementations (net/http, gin-gonic, gorilla/mux).
package router

import "net/http"

// Router defines the interface for HTTP routing.
// Implementations can use different underlying routers (net/http, gin-gonic, gorilla/mux).
type Router interface {
	// HTTP method handlers
	GET(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
	POST(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
	PUT(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
	DELETE(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
	PATCH(path string, handler HandlerFunc, middleware ...MiddlewareFunc)

	// Group creates a route group with common prefix and middleware
	Group(prefix string, middleware ...MiddlewareFunc) Router

	// Use applies middleware to all routes
	Use(middleware ...MiddlewareFunc)

	// ServeHTTP implements http.Handler
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// HandlerFunc is the function signature for route handlers.
// It receives a Context and returns an error.
type HandlerFunc func(Context) error

// MiddlewareFunc is the function signature for middleware.
// It wraps a HandlerFunc and returns a new HandlerFunc.
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// Context provides access to request and response in a router-agnostic way.
type Context interface {
	// Request returns the underlying HTTP request
	Request() *http.Request

	// SetRequest sets the HTTP request (useful for middleware that modifies the request)
	SetRequest(r *http.Request)

	// Response returns the response writer
	Response() ResponseWriter

	// SetResponse sets the HTTP response writer (useful for middleware that wraps responses)
	SetResponse(w ResponseWriter)

	// Param returns a URL parameter by name (e.g., /users/:id)
	Param(name string) string

	// Query returns a query parameter by name (e.g., /users?name=john)
	Query(name string) string

	// Bind parses the request body into the provided struct
	Bind(v interface{}) error

	// JSON sends a JSON response with the given status code
	JSON(code int, v interface{}) error

	// String sends a plain text response with the given status code
	String(code int, s string) error

	// Get retrieves a value from the context by key
	Get(key string) interface{}

	// Set stores a value in the context by key
	Set(key string, value interface{})
}

// ResponseWriter wraps http.ResponseWriter to track response status.
type ResponseWriter interface {
	http.ResponseWriter

	// Status returns the HTTP status code of the response
	Status() int

	// Written returns whether the response has been written
	Written() bool
}
