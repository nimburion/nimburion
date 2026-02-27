package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/middleware"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestRequestID_GeneratesUUID(t *testing.T) {
	// Given: A request without X-Request-ID header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	
	r := nethttp.NewRouter()
	var capturedRequestID string
	
	// When: RequestID middleware is applied
	r.Use(RequestID())
	r.GET("/test", func(c router.Context) error {
		capturedRequestID = GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})
	
	r.ServeHTTP(rec, req)
	
	// Then: A UUID should be generated
	if capturedRequestID == "" {
		t.Error("Expected request ID to be generated, got empty string")
	}
	
	// And: Response header should contain the request ID
	responseRequestID := rec.Header().Get(RequestIDHeader)
	if responseRequestID != capturedRequestID {
		t.Errorf("Expected response header request ID %s, got %s", capturedRequestID, responseRequestID)
	}
	
	// And: Request ID should be a valid UUID format (36 characters with dashes)
	if len(capturedRequestID) != 36 {
		t.Errorf("Expected UUID format (36 chars), got %d chars: %s", len(capturedRequestID), capturedRequestID)
	}
}

func TestRequestID_PreservesExistingHeader(t *testing.T) {
	// Given: A request with existing X-Request-ID header
	existingID := "existing-request-id-123"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)
	rec := httptest.NewRecorder()
	
	r := nethttp.NewRouter()
	var capturedRequestID string
	
	// When: RequestID middleware is applied
	r.Use(RequestID())
	r.GET("/test", func(c router.Context) error {
		capturedRequestID = GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})
	
	r.ServeHTTP(rec, req)
	
	// Then: The existing request ID should be preserved
	if capturedRequestID != existingID {
		t.Errorf("Expected request ID %s, got %s", existingID, capturedRequestID)
	}
	
	// And: Response header should contain the same request ID
	responseRequestID := rec.Header().Get(RequestIDHeader)
	if responseRequestID != existingID {
		t.Errorf("Expected response header request ID %s, got %s", existingID, responseRequestID)
	}
}

func TestRequestID_AddsToResponseHeaders(t *testing.T) {
	// Given: A request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	
	r := nethttp.NewRouter()
	
	// When: RequestID middleware is applied
	r.Use(RequestID())
	r.GET("/test", func(c router.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	
	r.ServeHTTP(rec, req)
	
	// Then: Response should have X-Request-ID header
	responseRequestID := rec.Header().Get(RequestIDHeader)
	if responseRequestID == "" {
		t.Error("Expected X-Request-ID header in response, got empty string")
	}
}

func TestRequestID_AddsToContext(t *testing.T) {
	// Given: A request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	
	r := nethttp.NewRouter()
	var contextRequestID string
	
	// When: RequestID middleware is applied
	r.Use(RequestID())
	r.GET("/test", func(c router.Context) error {
		// Extract from context
		contextRequestID = GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})
	
	r.ServeHTTP(rec, req)
	
	// Then: Request ID should be available in context
	if contextRequestID == "" {
		t.Error("Expected request ID in context, got empty string")
	}
}

func TestRequestID_PropagatesAcrossMiddleware(t *testing.T) {
	// Given: Multiple middleware in the chain
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	
	r := nethttp.NewRouter()
	var requestIDInMiddleware string
	var requestIDInHandler string
	
	// Custom middleware that reads request ID
	customMiddleware := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			requestIDInMiddleware = GetRequestID(c.Request().Context())
			return next(c)
		}
	}
	
	// When: RequestID middleware is applied first, then custom middleware
	r.Use(RequestID(), customMiddleware)
	r.GET("/test", func(c router.Context) error {
		requestIDInHandler = GetRequestID(c.Request().Context())
		return c.String(http.StatusOK, "ok")
	})
	
	r.ServeHTTP(rec, req)
	
	// Then: Request ID should be the same across all middleware and handler
	if requestIDInMiddleware == "" {
		t.Error("Expected request ID in middleware, got empty string")
	}
	
	if requestIDInHandler == "" {
		t.Error("Expected request ID in handler, got empty string")
	}
	
	if requestIDInMiddleware != requestIDInHandler {
		t.Errorf("Request ID mismatch: middleware=%s, handler=%s", requestIDInMiddleware, requestIDInHandler)
	}
}

func TestGetRequestID_WithValidContext(t *testing.T) {
	// Given: A context with request ID
	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "test-request-123")
	
	// When: GetRequestID is called
	requestID := GetRequestID(ctx)
	
	// Then: Should return the request ID
	if requestID != "test-request-123" {
		t.Errorf("Expected request ID 'test-request-123', got '%s'", requestID)
	}
}

func TestGetRequestID_WithNilContext(t *testing.T) {
	// Given: A nil context
	var ctx context.Context = nil
	
	// When: GetRequestID is called
	requestID := GetRequestID(ctx)
	
	// Then: Should return empty string
	if requestID != "" {
		t.Errorf("Expected empty string for nil context, got '%s'", requestID)
	}
}

func TestGetRequestID_WithoutRequestID(t *testing.T) {
	// Given: A context without request ID
	ctx := context.Background()
	
	// When: GetRequestID is called
	requestID := GetRequestID(ctx)
	
	// Then: Should return empty string
	if requestID != "" {
		t.Errorf("Expected empty string for context without request ID, got '%s'", requestID)
	}
}

func TestGetRequestID_WithWrongType(t *testing.T) {
	// Given: A context with wrong type for request ID
	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, 12345)
	
	// When: GetRequestID is called
	requestID := GetRequestID(ctx)
	
	// Then: Should return empty string
	if requestID != "" {
		t.Errorf("Expected empty string for wrong type, got '%s'", requestID)
	}
}

func TestRequestID_UniqueIDsForMultipleRequests(t *testing.T) {
	// Given: Multiple requests without X-Request-ID header
	r := nethttp.NewRouter()
	r.Use(RequestID())
	
	var requestIDs []string
	r.GET("/test", func(c router.Context) error {
		requestIDs = append(requestIDs, GetRequestID(c.Request().Context()))
		return c.String(http.StatusOK, "ok")
	})
	
	// When: Multiple requests are made
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
	
	// Then: All request IDs should be unique
	seen := make(map[string]bool)
	for _, id := range requestIDs {
		if seen[id] {
			t.Errorf("Duplicate request ID found: %s", id)
		}
		seen[id] = true
	}
	
	if len(requestIDs) != 5 {
		t.Errorf("Expected 5 request IDs, got %d", len(requestIDs))
	}
}
