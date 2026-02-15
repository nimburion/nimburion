package requestid

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/middleware/logging"
	"github.com/nimburion/nimburion/pkg/middleware/testutil"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// TestProperty1_RequestIDPropagation verifies that request IDs are properly propagated through the request lifecycle.
// Property 1: Request ID Propagation
//
// *For any* HTTP request, if the request includes an X-Request-ID header, then that ID should be preserved
// in the response header, all log entries, and the request context; if no X-Request-ID header is present,
// then a unique ID should be generated and included in the response header, all log entries, and the request context.
//
// **Validates: Requirements 3.1, 15.1, 15.2, 15.3, 15.4, 15.5**
func TestProperty1_RequestIDPropagation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// UUID regex pattern for validation
	uuidPattern := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

	// Generator for request ID strings (alphanumeric with dashes)
	genRequestID := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) <= 100
	})

	properties.Property("preserves existing X-Request-ID header in response and context", prop.ForAll(
		func(existingID string) bool {
			// Create mock logger to capture log entries
			mock := &testutil.MockLogger{}

			// Create router with RequestID and Logging middleware
			r := nethttp.NewRouter()
			r.Use(RequestID())
			r.Use(logging.Logging(mock))

			var capturedContextID string
			r.GET("/test", func(c router.Context) error {
				capturedContextID = GetRequestID(c.Request().Context())
				return c.String(http.StatusOK, "ok")
			})

			// Create request with existing X-Request-ID header
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set(RequestIDHeader, existingID)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify: Response header should contain the same request ID
			responseID := w.Header().Get(RequestIDHeader)
			if responseID != existingID {
				t.Logf("Response header mismatch: expected %s, got %s", existingID, responseID)
				return false
			}

			// Verify: Context should contain the same request ID
			if capturedContextID != existingID {
				t.Logf("Context ID mismatch: expected %s, got %s", existingID, capturedContextID)
				return false
			}

			// Verify: All log entries should contain the request ID
			for _, log := range mock.Logs {
				if log.Fields["request_id"] != existingID {
					t.Logf("Log entry missing correct request_id: expected %s, got %v", existingID, log.Fields["request_id"])
					return false
				}
			}

			return true
		},
		genRequestID,
	))

	properties.Property("generates unique UUID when X-Request-ID header is absent", prop.ForAll(
		func(seed int) bool {
			// Create mock logger to capture log entries
			mock := &testutil.MockLogger{}

			// Create router with RequestID and Logging middleware
			r := nethttp.NewRouter()
			r.Use(RequestID())
			r.Use(logging.Logging(mock))

			var capturedContextID string
			r.GET("/test", func(c router.Context) error {
				capturedContextID = GetRequestID(c.Request().Context())
				return c.String(http.StatusOK, "ok")
			})

			// Create request WITHOUT X-Request-ID header
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify: Response header should contain a generated UUID
			responseID := w.Header().Get(RequestIDHeader)
			if responseID == "" {
				t.Log("Response header is empty, expected generated UUID")
				return false
			}

			// Verify: Generated ID should be a valid UUID format
			if !uuidPattern.MatchString(responseID) {
				t.Logf("Generated ID is not a valid UUID: %s", responseID)
				return false
			}

			// Verify: Context should contain the same generated ID
			if capturedContextID != responseID {
				t.Logf("Context ID mismatch: expected %s, got %s", responseID, capturedContextID)
				return false
			}

			// Verify: All log entries should contain the generated request ID
			for _, log := range mock.Logs {
				if log.Fields["request_id"] != responseID {
					t.Logf("Log entry missing correct request_id: expected %s, got %v", responseID, log.Fields["request_id"])
					return false
				}
			}

			return true
		},
		gen.Int(),
	))

	properties.Property("request ID is accessible across multiple middleware", prop.ForAll(
		func(hasHeader bool, headerID string) bool {
			// Create mock logger
			mock := &testutil.MockLogger{}

			// Create router with RequestID middleware first
			r := nethttp.NewRouter()
			r.Use(RequestID())

			var idInMiddleware1 string
			var idInMiddleware2 string
			var idInHandler string

			// Add custom middleware to capture request ID at different stages
			middleware1 := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					idInMiddleware1 = GetRequestID(c.Request().Context())
					return next(c)
				}
			}

			middleware2 := func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					idInMiddleware2 = GetRequestID(c.Request().Context())
					return next(c)
				}
			}

			r.Use(middleware1, middleware2, logging.Logging(mock))

			r.GET("/test", func(c router.Context) error {
				idInHandler = GetRequestID(c.Request().Context())
				return c.String(http.StatusOK, "ok")
			})

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if hasHeader && headerID != "" {
				req.Header.Set(RequestIDHeader, headerID)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Get the expected ID (either from header or generated)
			expectedID := w.Header().Get(RequestIDHeader)
			if expectedID == "" {
				t.Log("No request ID in response header")
				return false
			}

			// Verify: All middleware and handler see the same request ID
			if idInMiddleware1 != expectedID {
				t.Logf("Middleware1 ID mismatch: expected %s, got %s", expectedID, idInMiddleware1)
				return false
			}

			if idInMiddleware2 != expectedID {
				t.Logf("Middleware2 ID mismatch: expected %s, got %s", expectedID, idInMiddleware2)
				return false
			}

			if idInHandler != expectedID {
				t.Logf("Handler ID mismatch: expected %s, got %s", expectedID, idInHandler)
				return false
			}

			// Verify: All log entries contain the same request ID
			for _, log := range mock.Logs {
				if log.Fields["request_id"] != expectedID {
					t.Logf("Log entry ID mismatch: expected %s, got %v", expectedID, log.Fields["request_id"])
					return false
				}
			}

			return true
		},
		gen.Bool(),
		genRequestID,
	))

	properties.Property("each request without header gets a unique ID", prop.ForAll(
		func(numRequests int) bool {
			// Limit number of requests to reasonable range
			if numRequests < 2 || numRequests > 20 {
				return true
			}

			// Create router
			r := nethttp.NewRouter()
			r.Use(RequestID())

			r.GET("/test", func(c router.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			// Collect request IDs from multiple requests
			seenIDs := make(map[string]bool)
			for i := 0; i < numRequests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				requestID := w.Header().Get(RequestIDHeader)
				if requestID == "" {
					t.Log("Empty request ID generated")
					return false
				}

				// Check for duplicates
				if seenIDs[requestID] {
					t.Logf("Duplicate request ID found: %s", requestID)
					return false
				}
				seenIDs[requestID] = true
			}

			return true
		},
		gen.IntRange(2, 20),
	))

	properties.Property("request ID is available in context for downstream operations", prop.ForAll(
		func(requestID string) bool {
			// Create router
			r := nethttp.NewRouter()
			r.Use(RequestID())

			var extractedFromContext string
			r.GET("/test", func(c router.Context) error {
				// Simulate downstream operation that needs request ID
				ctx := c.Request().Context()
				extractedFromContext = GetRequestID(ctx)
				
				// Verify it can be extracted using the helper function
				if extractedFromContext == "" {
					return c.String(http.StatusInternalServerError, "no request ID")
				}
				return c.String(http.StatusOK, "ok")
			})

			// Create request with or without header
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if requestID != "" {
				req.Header.Set(RequestIDHeader, requestID)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify: Request ID was successfully extracted in handler
			if extractedFromContext == "" {
				t.Log("Failed to extract request ID from context")
				return false
			}

			// Verify: Response header matches what was extracted
			responseID := w.Header().Get(RequestIDHeader)
			if responseID != extractedFromContext {
				t.Logf("Response header mismatch: expected %s, got %s", extractedFromContext, responseID)
				return false
			}

			return true
		},
		genRequestID,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
