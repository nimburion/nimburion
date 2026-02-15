package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/middleware/logging"
	"github.com/nimburion/nimburion/pkg/middleware/metrics"
	"github.com/nimburion/nimburion/pkg/middleware/recovery"
	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// TestRequestIDAndLoggingIntegration verifies that RequestID middleware
// properly integrates with Logging middleware.
func TestRequestIDAndLoggingIntegration(t *testing.T) {
	// Given: A router with both RequestID and Logging middleware
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Sync()
	
	r := nethttp.NewRouter()
	r.Use(requestid.RequestID(), logging.Logging(log))
	
	var capturedRequestID string
	r.GET("/test", func(c router.Context) error {
		capturedRequestID = requestid.GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "success")
	})
	
	// When: A request is made
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	// Then: Request ID should be captured
	if capturedRequestID == "" {
		t.Error("Expected request ID to be captured")
	}
	
	// And: Response should have X-Request-ID header
	responseRequestID := rec.Header().Get(requestid.RequestIDHeader)
	if responseRequestID != capturedRequestID {
		t.Errorf("Expected response header request ID %s, got %s", capturedRequestID, responseRequestID)
	}
	
	// And: Response should be successful
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestRequestIDPreservationWithLogging verifies that an existing request ID
// is preserved and logged correctly.
func TestRequestIDPreservationWithLogging(t *testing.T) {
	// Given: A router with both RequestID and Logging middleware
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Sync()
	
	r := nethttp.NewRouter()
	r.Use(requestid.RequestID(), logging.Logging(log))
	
	var capturedRequestID string
	r.GET("/test", func(c router.Context) error {
		capturedRequestID = requestid.GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "success")
	})
	
	// When: A request with existing X-Request-ID is made
	existingID := "existing-id-from-client"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(requestid.RequestIDHeader, existingID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	// Then: The existing request ID should be preserved
	if capturedRequestID != existingID {
		t.Errorf("Expected request ID %s, got %s", existingID, capturedRequestID)
	}
	
	// And: Response should have the same request ID
	responseRequestID := rec.Header().Get(requestid.RequestIDHeader)
	if responseRequestID != existingID {
		t.Errorf("Expected response header request ID %s, got %s", existingID, responseRequestID)
	}
	
	// And: Response should be successful
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}
}

// TestMiddlewareOrdering verifies that RequestID middleware must come before
// Logging middleware for proper request ID propagation.
func TestMiddlewareOrdering(t *testing.T) {
	// Given: A router with RequestID before Logging
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Sync()
	
	r := nethttp.NewRouter()
	r.Use(requestid.RequestID(), logging.Logging(log))
	
	var requestIDInHandler string
	r.GET("/test", func(c router.Context) error {
		requestIDInHandler = requestid.GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "success")
	})
	
	// When: A request is made
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	// Then: Request ID should be available in handler
	if requestIDInHandler == "" {
		t.Error("Expected request ID in handler, got empty string")
	}
	
	// And: Request ID should be in response header
	responseRequestID := rec.Header().Get(requestid.RequestIDHeader)
	if responseRequestID != requestIDInHandler {
		t.Errorf("Expected response header request ID %s, got %s", requestIDInHandler, responseRequestID)
	}
}


// TestRecoveryWithRequestIDAndLogging verifies that Recovery middleware
// properly integrates with RequestID and Logging middleware.
func TestRecoveryWithRequestIDAndLogging(t *testing.T) {
	// Given: A router with RequestID, Recovery, and Logging middleware
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Sync()
	
	r := nethttp.NewRouter()
	// Order matters: RequestID -> Recovery -> Logging
	r.Use(requestid.RequestID(), recovery.Recovery(log), logging.Logging(log))
	
	r.GET("/panic", func(c router.Context) error {
		panic("integration test panic")
	})
	
	// When: A request is made to a panicking handler
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(requestid.RequestIDHeader, "integration-test-id")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	// Then: Response should be HTTP 500
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	
	// And: Response should have request ID header
	responseRequestID := rec.Header().Get(requestid.RequestIDHeader)
	if responseRequestID != "integration-test-id" {
		t.Errorf("Expected response header request ID 'integration-test-id', got %s", responseRequestID)
	}
}

// TestMiddlewareStackWithRecovery verifies the complete middleware stack
// including RequestID, Recovery, and Logging works together correctly.
func TestMiddlewareStackWithRecovery(t *testing.T) {
	// Given: A router with full middleware stack
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Sync()
	
	r := nethttp.NewRouter()
	r.Use(requestid.RequestID(), recovery.Recovery(log), logging.Logging(log))
	
	// Normal handler
	r.GET("/normal", func(c router.Context) error {
		return c.String(http.StatusOK, "success")
	})
	
	// Panicking handler
	r.GET("/panic", func(c router.Context) error {
		panic("test panic")
	})
	
	// When: A normal request is made
	req1 := httptest.NewRequest(http.MethodGet, "/normal", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	
	// Then: Normal request should succeed
	if rec1.Code != http.StatusOK {
		t.Errorf("Expected status code %d for normal request, got %d", http.StatusOK, rec1.Code)
	}
	
	// When: A panicking request is made
	req2 := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	
	// Then: Panicking request should be recovered
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d for panicking request, got %d", http.StatusInternalServerError, rec2.Code)
	}
	
	// And: Both requests should have request IDs
	if rec1.Header().Get(requestid.RequestIDHeader) == "" {
		t.Error("Expected request ID in normal response")
	}
	if rec2.Header().Get(requestid.RequestIDHeader) == "" {
		t.Error("Expected request ID in panic response")
	}
}

// TestMetricsWithFullMiddlewareStack verifies that Metrics middleware
// properly integrates with the complete middleware stack.
func TestMetricsWithFullMiddlewareStack(t *testing.T) {
	// Given: A router with full middleware stack including Metrics
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer log.Sync()
	
	r := nethttp.NewRouter()
	// Order: RequestID -> Metrics -> Recovery -> Logging
	r.Use(requestid.RequestID(), metrics.Metrics(), recovery.Recovery(log), logging.Logging(log))
	
	// Normal handler
	r.GET("/normal", func(c router.Context) error {
		return c.String(http.StatusOK, "success")
	})
	
	// Error handler
	r.GET("/error", func(c router.Context) error {
		return c.String(http.StatusInternalServerError, "error")
	})
	
	// When: A normal request is made
	req1 := httptest.NewRequest(http.MethodGet, "/normal", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	
	// Then: Normal request should succeed
	if rec1.Code != http.StatusOK {
		t.Errorf("Expected status code %d for normal request, got %d", http.StatusOK, rec1.Code)
	}
	
	// When: An error request is made
	req2 := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	
	// Then: Error request should return 500
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d for error request, got %d", http.StatusInternalServerError, rec2.Code)
	}
	
	// And: Both requests should have request IDs
	if rec1.Header().Get(requestid.RequestIDHeader) == "" {
		t.Error("Expected request ID in normal response")
	}
	if rec2.Header().Get(requestid.RequestIDHeader) == "" {
		t.Error("Expected request ID in error response")
	}
	
	// Note: Metrics are recorded but cannot be easily verified in unit tests
	// The middleware should have recorded:
	// - httpRequestDuration for both requests
	// - httpRequestsTotal for both requests
	// - httpRequestsInFlight (incremented and decremented for each request)
}
