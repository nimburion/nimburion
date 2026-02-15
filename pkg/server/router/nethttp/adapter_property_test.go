package nethttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// Property 35: Router Interface Compatibility
// **Validates: Requirements 2.1**
//
// For any router implementation (net/http, gin, gorilla/mux), the router should satisfy
// the Router interface contract and correctly route requests to handlers with middleware support.
func TestProperty35_RouterInterfaceCompatibility(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for HTTP methods
	genMethod := gen.OneConstOf(
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	)

	// Generator for path segments (alphanumeric strings, 1-10 chars)
	genPathSegment := gen.Identifier().Map(func(s string) string {
		if len(s) > 10 {
			return s[:10]
		}
		if len(s) == 0 {
			return "a"
		}
		return s
	})

	// Generator for simple paths
	genPath := genPathSegment.Map(func(segment string) string {
		return "/" + segment
	})

	// Generator for response status codes
	genStatusCode := gen.OneConstOf(
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
	)

	// Generator for response body
	genResponseBody := gen.Identifier()

	properties.Property("router correctly routes requests to registered handlers", prop.ForAll(
		func(method, path string, statusCode int, responseBody string) bool {
			// Create router
			r := NewRouter()

			// Track if handler was called
			handlerCalled := false
			var receivedMethod, receivedPath string

			// Create handler
			handler := func(c router.Context) error {
				handlerCalled = true
				receivedMethod = c.Request().Method
				receivedPath = c.Request().URL.Path
				return c.String(statusCode, responseBody)
			}

			// Register route based on method
			switch method {
			case http.MethodGet:
				r.GET(path, handler)
			case http.MethodPost:
				r.POST(path, handler)
			case http.MethodPut:
				r.PUT(path, handler)
			case http.MethodDelete:
				r.DELETE(path, handler)
			case http.MethodPatch:
				r.PATCH(path, handler)
			}

			// Create request
			req := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()

			// Serve request
			r.ServeHTTP(w, req)

			// Verify handler was called
			if !handlerCalled {
				return false
			}

			// Verify method and path were received correctly
			if receivedMethod != method {
				return false
			}

			if receivedPath != path {
				return false
			}

			// Verify response status code
			if w.Code != statusCode {
				return false
			}

			// Verify response body
			if w.Body.String() != responseBody {
				return false
			}

			return true
		},
		genMethod,
		genPath,
		genStatusCode,
		genResponseBody,
	))

	properties.Property("router correctly applies middleware in order", prop.ForAll(
		func(path string) bool {
			r := NewRouter()

			// Track middleware execution order
			var executionOrder []string

			// Create middleware that tracks execution
			middleware1 := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					executionOrder = append(executionOrder, "middleware1_before")
					err := next(c)
					executionOrder = append(executionOrder, "middleware1_after")
					return err
				}
			}

			middleware2 := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					executionOrder = append(executionOrder, "middleware2_before")
					err := next(c)
					executionOrder = append(executionOrder, "middleware2_after")
					return err
				}
			}

			// Apply global middleware
			r.Use(middleware1, middleware2)

			// Register handler
			r.GET(path, func(c router.Context) error {
				executionOrder = append(executionOrder, "handler")
				return c.String(http.StatusOK, "ok")
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			// Serve request
			r.ServeHTTP(w, req)

			// Verify execution order: middleware1 -> middleware2 -> handler -> middleware2 -> middleware1
			expectedOrder := []string{
				"middleware1_before",
				"middleware2_before",
				"handler",
				"middleware2_after",
				"middleware1_after",
			}

			if len(executionOrder) != len(expectedOrder) {
				return false
			}

			for i, expected := range expectedOrder {
				if executionOrder[i] != expected {
					return false
				}
			}

			return true
		},
		genPath,
	))

	properties.Property("router correctly extracts path parameters", prop.ForAll(
		func(paramName, paramValue string) bool {
			r := NewRouter()

			// Track extracted parameter
			var extractedValue string

			// Register route with parameter
			pattern := fmt.Sprintf("/resource/:%s", paramName)
			r.GET(pattern, func(c router.Context) error {
				extractedValue = c.Param(paramName)
				return c.String(http.StatusOK, "ok")
			})

			// Create request with parameter value
			requestPath := fmt.Sprintf("/resource/%s", paramValue)
			req := httptest.NewRequest(http.MethodGet, requestPath, nil)
			w := httptest.NewRecorder()

			// Serve request
			r.ServeHTTP(w, req)

			// Verify parameter was extracted correctly
			if extractedValue != paramValue {
				return false
			}

			return true
		},
		genPathSegment,
		genPathSegment,
	))

	properties.Property("router correctly handles query parameters", prop.ForAll(
		func(path, queryKey, queryValue string) bool {
			r := NewRouter()

			// Track extracted query value
			var extractedValue string

			// Register route
			r.GET(path, func(c router.Context) error {
				extractedValue = c.Query(queryKey)
				return c.String(http.StatusOK, "ok")
			})

			// Create request with query parameter
			requestPath := fmt.Sprintf("%s?%s=%s", path, queryKey, queryValue)
			req := httptest.NewRequest(http.MethodGet, requestPath, nil)
			w := httptest.NewRecorder()

			// Serve request
			r.ServeHTTP(w, req)

			// Verify query parameter was extracted correctly
			if extractedValue != queryValue {
				return false
			}

			return true
		},
		genPath,
		genPathSegment,
		gen.Identifier(),
	))

	properties.Property("router correctly handles route groups with prefixes", prop.ForAll(
		func(prefix, subPath string) bool {
			r := NewRouter()

			// Track if handler was called
			handlerCalled := false

			// Create group with prefix
			group := r.Group("/" + prefix)
			group.GET("/"+subPath, func(c router.Context) error {
				handlerCalled = true
				return c.String(http.StatusOK, "ok")
			})

			// Create request with full path
			fullPath := fmt.Sprintf("/%s/%s", prefix, subPath)
			req := httptest.NewRequest(http.MethodGet, fullPath, nil)
			w := httptest.NewRecorder()

			// Serve request
			r.ServeHTTP(w, req)

			// Verify handler was called
			if !handlerCalled {
				return false
			}

			// Verify response status
			if w.Code != http.StatusOK {
				return false
			}

			return true
		},
		genPathSegment,
		genPathSegment,
	))

	properties.Property("router returns 404 for unregistered routes", prop.ForAll(
		func(segment1, segment2 string) bool {
			r := NewRouter()

			// Register one route
			registeredPath := "/" + segment1
			r.GET(registeredPath, func(c router.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			// Request a different route
			requestedPath := "/" + segment2
			
			// Only test if paths are different
			if registeredPath == requestedPath {
				return true // Skip this case
			}

			req := httptest.NewRequest(http.MethodGet, requestedPath, nil)
			w := httptest.NewRecorder()

			// Serve request
			r.ServeHTTP(w, req)

			// Verify 404 response
			if w.Code != http.StatusNotFound {
				return false
			}

			return true
		},
		genPathSegment,
		genPathSegment,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
