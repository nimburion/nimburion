package logging

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/middleware/testutil"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestLogging_RequestStartAndCompletion(t *testing.T) {
	// Create mock logger
	mock := &testutil.MockLogger{}

	// Create router with logging middleware
	r := nethttp.NewRouter()
	r.Use(Logging(mock))

	// Register test handler
	r.GET("/test", func(c router.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Add request ID to context
	ctx := context.WithValue(req.Context(), "request_id", "test-req-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify logs
	if len(mock.Logs) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(mock.Logs))
	}

	// Verify request started log
	startLog := mock.Logs[0]
	if startLog.Msg != "request started" {
		t.Errorf("expected message 'request started', got %q", startLog.Msg)
	}
	if startLog.Fields["request_id"] != "test-req-123" {
		t.Errorf("expected request_id 'test-req-123', got %v", startLog.Fields["request_id"])
	}
	if startLog.Fields["method"] != "GET" {
		t.Errorf("expected method 'GET', got %v", startLog.Fields["method"])
	}
	if startLog.Fields["path"] != "/test" {
		t.Errorf("expected path '/test', got %v", startLog.Fields["path"])
	}
	if startLog.Fields["remote_addr"] != "192.168.1.1:12345" {
		t.Errorf("expected remote_addr '192.168.1.1:12345', got %v", startLog.Fields["remote_addr"])
	}

	// Verify request completed log
	completedLog := mock.Logs[1]
	if completedLog.Msg != "request completed" {
		t.Errorf("expected message 'request completed', got %q", completedLog.Msg)
	}
	if completedLog.Fields["request_id"] != "test-req-123" {
		t.Errorf("expected request_id 'test-req-123', got %v", completedLog.Fields["request_id"])
	}
	if completedLog.Fields["status"] != 200 {
		t.Errorf("expected status 200, got %v", completedLog.Fields["status"])
	}
	if _, ok := completedLog.Fields["duration_ms"]; !ok {
		t.Error("expected duration_ms field in completed log")
	}
}

func TestLogging_RequestFailure(t *testing.T) {
	// Create mock logger
	mock := &testutil.MockLogger{}

	// Create router with logging middleware
	r := nethttp.NewRouter()
	r.Use(Logging(mock))

	// Register test handler that returns error
	testError := errors.New("test error")
	r.GET("/error", func(c router.Context) error {
		return testError
	})

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	ctx := context.WithValue(req.Context(), "request_id", "error-req-456")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify logs
	if len(mock.Logs) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(mock.Logs))
	}

	// Verify request failed log
	failedLog := mock.Logs[1]
	if failedLog.Msg != "request failed" {
		t.Errorf("expected message 'request failed', got %q", failedLog.Msg)
	}
	if failedLog.Fields["request_id"] != "error-req-456" {
		t.Errorf("expected request_id 'error-req-456', got %v", failedLog.Fields["request_id"])
	}
	if failedLog.Level != "error" {
		t.Errorf("expected level 'error', got %q", failedLog.Level)
	}
	if failedLog.Fields["error"] != testError {
		t.Errorf("expected error %v, got %v", testError, failedLog.Fields["error"])
	}
}

func TestLogging_WithoutRequestID(t *testing.T) {
	// Create mock logger
	mock := &testutil.MockLogger{}

	// Create router with logging middleware
	r := nethttp.NewRouter()
	r.Use(Logging(mock))

	// Register test handler
	r.GET("/test", func(c router.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// Make request without request ID in context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify logs
	if len(mock.Logs) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(mock.Logs))
	}

	// Verify logs still work without request ID (should be empty string)
	startLog := mock.Logs[0]
	if startLog.Msg != "request started" {
		t.Errorf("expected message 'request started', got %q", startLog.Msg)
	}
	// request_id should be empty string
	if rid := startLog.Fields["request_id"]; rid != "" {
		t.Errorf("expected empty request_id, got %v", rid)
	}
}

func TestLogging_DurationTracking(t *testing.T) {
	// Create mock logger
	mock := &testutil.MockLogger{}

	// Create router with logging middleware
	r := nethttp.NewRouter()
	r.Use(Logging(mock))

	// Register test handler with delay
	r.GET("/slow", func(c router.Context) error {
		time.Sleep(50 * time.Millisecond)
		return c.String(http.StatusOK, "done")
	})

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	ctx := context.WithValue(req.Context(), "request_id", "slow-req")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify logs
	if len(mock.Logs) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(mock.Logs))
	}

	// Verify duration is tracked
	completedLog := mock.Logs[1]
	durationMs, ok := completedLog.Fields["duration_ms"].(int64)
	if !ok {
		t.Fatalf("expected duration_ms to be int64, got %T", completedLog.Fields["duration_ms"])
	}

	// Duration should be at least 50ms
	if durationMs < 50 {
		t.Errorf("expected duration >= 50ms, got %vms", durationMs)
	}
}

func TestLoggingWithConfig_ExcludedPath(t *testing.T) {
	mock := &testutil.MockLogger{}

	r := nethttp.NewRouter()
	r.Use(LoggingWithConfig(mock, Config{
		Enabled:              true,
		LogStart:             true,
		ExcludedPathPrefixes: []string{"/health"},
	}))

	r.GET("/health/live", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(mock.Logs) != 0 {
		t.Fatalf("expected no log entries, got %d", len(mock.Logs))
	}
}

func TestLoggingWithConfig_PathPolicyMinimal(t *testing.T) {
	mock := &testutil.MockLogger{}

	r := nethttp.NewRouter()
	r.Use(LoggingWithConfig(mock, Config{
		Enabled:  true,
		LogStart: true,
		PathPolicies: []PathPolicy{
			{Prefix: "/api", Mode: ModeMinimal},
		},
	}))

	r.GET("/api/v1/members", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/members", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(mock.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(mock.Logs))
	}
	if mock.Logs[0].Msg != "request completed" {
		t.Fatalf("expected completion log, got %q", mock.Logs[0].Msg)
	}
}

func TestLoggingWithConfig_UsesConfiguredFields(t *testing.T) {
	mock := &testutil.MockLogger{}

	r := nethttp.NewRouter()
	r.Use(LoggingWithConfig(mock, Config{
		Enabled: true,
		Fields: []string{
			"request_method",
			"request_uri",
			"status",
			"request_time",
			"http_user_agent",
		},
	}))

	r.GET("/api/v1/orders", func(c router.Context) error {
		return c.String(http.StatusCreated, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?limit=1", nil)
	req.Header.Set("User-Agent", "nimburion-tests")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(mock.Logs) == 0 {
		t.Fatal("expected at least one log entry")
	}
	completed := mock.Logs[len(mock.Logs)-1]
	if completed.Msg != "request completed" {
		t.Fatalf("expected completion log, got %q", completed.Msg)
	}
	if completed.Fields["request_method"] != http.MethodGet {
		t.Fatalf("expected request_method GET, got %v", completed.Fields["request_method"])
	}
	if completed.Fields["request_uri"] != "/api/v1/orders?limit=1" {
		t.Fatalf("expected request_uri with query string, got %v", completed.Fields["request_uri"])
	}
	if completed.Fields["status"] != http.StatusCreated {
		t.Fatalf("expected status 201, got %v", completed.Fields["status"])
	}
	if completed.Fields["http_user_agent"] != "nimburion-tests" {
		t.Fatalf("expected http_user_agent nimburion-tests, got %v", completed.Fields["http_user_agent"])
	}
	if _, ok := completed.Fields["method"]; ok {
		t.Fatalf("did not expect legacy method field when not configured, got %v", completed.Fields["method"])
	}
}
