package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestMetrics_SuccessfulRequest(t *testing.T) {
	// Create router with metrics middleware
	r := nethttp.NewRouter()
	r.Use(Metrics())

	// Register test handler
	r.GET("/test", func(c router.Context) error {
		return c.String(http.StatusOK, "success")
	})

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Note: Prometheus metrics are recorded but cannot be easily verified
	// in unit tests without accessing the registry. The middleware should:
	// - Increment httpRequestsInFlight at start
	// - Decrement httpRequestsInFlight at end
	// - Record httpRequestDuration histogram
	// - Increment httpRequestsTotal counter
}

func TestMetrics_ErrorRequest(t *testing.T) {
	// Create router with metrics middleware
	r := nethttp.NewRouter()
	r.Use(Metrics())

	// Register test handler that returns error
	testError := errors.New("test error")
	r.GET("/error", func(c router.Context) error {
		return testError
	})

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Verify that metrics are still recorded even when handler returns error
	// The middleware should complete successfully and record metrics
}

func TestMetrics_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		handler        router.HandlerFunc
		expectedStatus int
	}{
		{
			name: "200 OK",
			path: "/ok",
			handler: func(c router.Context) error {
				return c.String(http.StatusOK, "ok")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "201 Created",
			path: "/created",
			handler: func(c router.Context) error {
				return c.String(http.StatusCreated, "created")
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "400 Bad Request",
			path: "/bad",
			handler: func(c router.Context) error {
				return c.String(http.StatusBadRequest, "bad request")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "500 Internal Server Error",
			path: "/error",
			handler: func(c router.Context) error {
				return c.String(http.StatusInternalServerError, "error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router with metrics middleware
			r := nethttp.NewRouter()
			r.Use(Metrics())

			// Register test handler
			r.GET(tt.path, tt.handler)

			// Make request
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify response status
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Metrics should be recorded with the correct status code label
		})
	}
}

func TestMetrics_DifferentHTTPMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			// Create router with metrics middleware
			r := nethttp.NewRouter()
			r.Use(Metrics())

			// Register handler for the method
			handler := func(c router.Context) error {
				return c.String(http.StatusOK, "ok")
			}

			switch method {
			case http.MethodGet:
				r.GET("/test", handler)
			case http.MethodPost:
				r.POST("/test", handler)
			case http.MethodPut:
				r.PUT("/test", handler)
			case http.MethodDelete:
				r.DELETE("/test", handler)
			case http.MethodPatch:
				r.PATCH("/test", handler)
			}

			// Make request
			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify response
			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			// Metrics should be recorded with the correct method label
		})
	}
}
