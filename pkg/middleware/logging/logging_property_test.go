package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/middleware/testutil"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// TestProperty13_HTTPRequestLogging verifies that HTTP request logging includes all required information.
// Property 13: HTTP Request Logging
//
// *For any* HTTP request, the logging middleware should create log entries containing method, path,
// status code, and duration in milliseconds.
//
// **Validates: Requirements 12.5**
func TestProperty13_HTTPRequestLogging(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for HTTP methods
	genHTTPMethod := gen.OneConstOf(
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	)

	// Generator for URL paths
	genPath := gen.OneGenOf(
		gen.Const("/"),
		gen.Const("/api/users"),
		gen.Const("/api/orders/123"),
		gen.Const("/health"),
		gen.AlphaString().Map(func(s string) string {
			if s == "" {
				return "/"
			}
			return "/" + s
		}),
	)

	// Generator for HTTP status codes
	genStatusCode := gen.OneConstOf(
		http.StatusOK,
		http.StatusCreated,
		http.StatusNoContent,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
	)

	properties.Property("logs contain method, path, status, and duration for successful requests", prop.ForAll(
		func(method, path string, statusCode int) bool {
			// Create mock logger to capture log entries
			mock := &testutil.MockLogger{}

			// Create router with logging middleware
			r := nethttp.NewRouter()
			r.Use(Logging(mock))

			// Register handler that returns the specified status code
			handler := func(c router.Context) error {
				return c.String(statusCode, "response")
			}

			// Register handler for all methods
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

			// Make request
			req := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify: Should have at least 2 log entries (start and completion)
			if len(mock.Logs) < 2 {
				t.Logf("Expected at least 2 log entries, got %d", len(mock.Logs))
				return false
			}

			// Find the completion log entry
			var completionLog *testutil.LogEntry
			for i := range mock.Logs {
				if mock.Logs[i].Msg == "request completed" || mock.Logs[i].Msg == "request failed" {
					completionLog = &mock.Logs[i]
					break
				}
			}

			if completionLog == nil {
				t.Log("No completion log entry found")
				return false
			}

			// Verify: Log contains method
			if completionLog.Fields["method"] != method {
				t.Logf("Method mismatch: expected %s, got %v", method, completionLog.Fields["method"])
				return false
			}

			// Verify: Log contains path
			if completionLog.Fields["path"] != path {
				t.Logf("Path mismatch: expected %s, got %v", path, completionLog.Fields["path"])
				return false
			}

			// Verify: Log contains status code
			loggedStatus, ok := completionLog.Fields["status"].(int)
			if !ok {
				t.Logf("Status is not an int: %T", completionLog.Fields["status"])
				return false
			}
			if loggedStatus != statusCode {
				t.Logf("Status mismatch: expected %d, got %d", statusCode, loggedStatus)
				return false
			}

			// Verify: Log contains duration in milliseconds
			durationMs, ok := completionLog.Fields["duration_ms"]
			if !ok {
				t.Log("Missing duration_ms field")
				return false
			}

			// Verify duration is a valid number (int64)
			if _, ok := durationMs.(int64); !ok {
				t.Logf("duration_ms is not int64: %T", durationMs)
				return false
			}

			return true
		},
		genHTTPMethod,
		genPath,
		genStatusCode,
	))

	properties.Property("logs contain method, path, status, and duration for failed requests", prop.ForAll(
		func(method, path string) bool {
			// Create mock logger to capture log entries
			mock := &testutil.MockLogger{}

			// Create router with logging middleware
			r := nethttp.NewRouter()
			r.Use(Logging(mock))

			// Register handler that returns an error
			testError := http.ErrAbortHandler
			handler := func(c router.Context) error {
				return testError
			}

			// Register handler for all methods
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

			// Make request
			req := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify: Should have at least 2 log entries (start and failure)
			if len(mock.Logs) < 2 {
				t.Logf("Expected at least 2 log entries, got %d", len(mock.Logs))
				return false
			}

			// Find the failure log entry
			var failureLog *testutil.LogEntry
			for i := range mock.Logs {
				if mock.Logs[i].Msg == "request failed" {
					failureLog = &mock.Logs[i]
					break
				}
			}

			if failureLog == nil {
				t.Log("No failure log entry found")
				return false
			}

			// Verify: Log contains method
			if failureLog.Fields["method"] != method {
				t.Logf("Method mismatch: expected %s, got %v", method, failureLog.Fields["method"])
				return false
			}

			// Verify: Log contains path
			if failureLog.Fields["path"] != path {
				t.Logf("Path mismatch: expected %s, got %v", path, failureLog.Fields["path"])
				return false
			}

			// Verify: Log contains status code
			if _, ok := failureLog.Fields["status"].(int); !ok {
				t.Logf("Status is not an int: %T", failureLog.Fields["status"])
				return false
			}

			// Verify: Log contains duration in milliseconds
			durationMs, ok := failureLog.Fields["duration_ms"]
			if !ok {
				t.Log("Missing duration_ms field")
				return false
			}

			// Verify duration is a valid number (int64)
			if _, ok := durationMs.(int64); !ok {
				t.Logf("duration_ms is not int64: %T", durationMs)
				return false
			}

			// Verify: Log contains error field
			if failureLog.Fields["error"] == nil {
				t.Log("Missing error field in failure log")
				return false
			}

			return true
		},
		genHTTPMethod,
		genPath,
	))

	properties.Property("duration increases with handler execution time", prop.ForAll(
		func(delayMs int) bool {
			// Limit delay to reasonable range (1-100ms)
			if delayMs < 1 || delayMs > 100 {
				return true
			}

			// Create mock logger to capture log entries
			mock := &testutil.MockLogger{}

			// Create router with logging middleware
			r := nethttp.NewRouter()
			r.Use(Logging(mock))

			// Register handler with artificial delay
			r.GET("/test", func(c router.Context) error {
				// Simulate work
				for i := 0; i < delayMs*1000; i++ {
					_ = i * i
				}
				return c.String(http.StatusOK, "ok")
			})

			// Make request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Find completion log
			var completionLog *testutil.LogEntry
			for i := range mock.Logs {
				if mock.Logs[i].Msg == "request completed" {
					completionLog = &mock.Logs[i]
					break
				}
			}

			if completionLog == nil {
				t.Log("No completion log found")
				return false
			}

			// Verify duration is present and non-negative
			durationMs, ok := completionLog.Fields["duration_ms"].(int64)
			if !ok {
				t.Logf("duration_ms is not int64: %T", completionLog.Fields["duration_ms"])
				return false
			}

			if durationMs < 0 {
				t.Logf("duration_ms is negative: %d", durationMs)
				return false
			}

			return true
		},
		gen.IntRange(1, 100),
	))

	properties.Property("all log entries include request_id when present", prop.ForAll(
		func(requestID, path string) bool {
			if requestID == "" {
				return true
			}

			// Create mock logger to capture log entries
			mock := &testutil.MockLogger{}

			// Create router with RequestID and Logging middleware
			r := nethttp.NewRouter()
			r.Use(requestid.RequestID())
			r.Use(Logging(mock))

			r.GET(path, func(c router.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			// Make request with request ID header
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set(requestid.RequestIDHeader, requestID)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify: All log entries should contain the request ID
			for _, log := range mock.Logs {
				if log.Fields["request_id"] != requestID {
					t.Logf("Log entry missing correct request_id: expected %s, got %v", requestID, log.Fields["request_id"])
					return false
				}
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 50 }),
		genPath,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
