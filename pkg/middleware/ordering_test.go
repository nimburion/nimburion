package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/middleware/logging"
	"github.com/nimburion/nimburion/pkg/middleware/recovery"
	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/middleware/testutil"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// TestMiddlewareOrdering_ExecutionOrder verifies that middleware executes in the configured order.
// Middleware should execute in order for incoming requests (first to last) and in reverse order
// for responses (last to first).
func TestMiddlewareOrdering_ExecutionOrder(t *testing.T) {
	// Given: A router with multiple middleware in a specific order
	r := nethttp.NewRouter()
	
	var executionOrder []string
	
	// Create middleware that tracks execution order
	middleware1 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "middleware1-before")
			err := next(c)
			executionOrder = append(executionOrder, "middleware1-after")
			return err
		}
	}
	
	middleware2 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "middleware2-before")
			err := next(c)
			executionOrder = append(executionOrder, "middleware2-after")
			return err
		}
	}
	
	middleware3 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "middleware3-before")
			err := next(c)
			executionOrder = append(executionOrder, "middleware3-after")
			return err
		}
	}
	
	// When: Middleware is applied in order: 1, 2, 3
	r.Use(middleware1, middleware2, middleware3)
	
	r.GET("/test", func(c router.Context) error {
		executionOrder = append(executionOrder, "handler")
		return c.String(http.StatusOK, "ok")
	})
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	// Then: Middleware should execute in order for request (1, 2, 3, handler)
	// and in reverse order for response (3, 2, 1)
	expectedOrder := []string{
		"middleware1-before",
		"middleware2-before",
		"middleware3-before",
		"handler",
		"middleware3-after",
		"middleware2-after",
		"middleware1-after",
	}
	
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected %d execution steps, got %d", len(expectedOrder), len(executionOrder))
	}
	
	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("execution order mismatch at step %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
}

// TestMiddlewareOrdering_GlobalAndRouteSpecific verifies that global middleware
// executes before route-specific middleware.
func TestMiddlewareOrdering_GlobalAndRouteSpecific(t *testing.T) {
	// Given: A router with global middleware and route-specific middleware
	r := nethttp.NewRouter()
	
	var executionOrder []string
	
	globalMiddleware := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "global")
			return next(c)
		}
	}
	
	routeMiddleware := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "route")
			return next(c)
		}
	}
	
	// When: Global middleware is applied, then route-specific middleware
	r.Use(globalMiddleware)
	r.GET("/test", func(c router.Context) error {
		executionOrder = append(executionOrder, "handler")
		return c.String(http.StatusOK, "ok")
	}, routeMiddleware)
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	// Then: Global middleware should execute before route-specific middleware
	expectedOrder := []string{"global", "route", "handler"}
	
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected %d execution steps, got %d", len(expectedOrder), len(executionOrder))
	}
	
	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("execution order mismatch at step %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
}

// TestMiddlewareOrdering_MultipleRoutes verifies that middleware ordering
// is consistent across different routes.
func TestMiddlewareOrdering_MultipleRoutes(t *testing.T) {
	// Given: A router with middleware applied to multiple routes
	r := nethttp.NewRouter()
	
	var route1Order []string
	var route2Order []string
	
	middleware1 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Get existing order or create new slice
			var order []string
			if existing := c.Get("order"); existing != nil {
				order = existing.([]string)
			}
			order = append(order, "m1")
			c.Set("order", order)
			return next(c)
		}
	}
	
	middleware2 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Get existing order or create new slice
			var order []string
			if existing := c.Get("order"); existing != nil {
				order = existing.([]string)
			}
			order = append(order, "m2")
			c.Set("order", order)
			return next(c)
		}
	}
	
	// When: Middleware is applied in the same order to multiple routes
	r.Use(middleware1, middleware2)
	
	r.GET("/route1", func(c router.Context) error {
		order := c.Get("order").([]string)
		route1Order = order
		return c.String(http.StatusOK, "ok")
	})
	
	r.GET("/route2", func(c router.Context) error {
		order := c.Get("order").([]string)
		route2Order = order
		return c.String(http.StatusOK, "ok")
	})
	
	// Then: Both routes should execute middleware in the same order
	req1 := httptest.NewRequest(http.MethodGet, "/route1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	
	req2 := httptest.NewRequest(http.MethodGet, "/route2", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	
	// Verify route1 order
	if len(route1Order) != 2 || route1Order[0] != "m1" || route1Order[1] != "m2" {
		t.Errorf("route1: expected [m1, m2], got %v", route1Order)
	}
	
	// Verify route2 order
	if len(route2Order) != 2 || route2Order[0] != "m1" || route2Order[1] != "m2" {
		t.Errorf("route2: expected [m1, m2], got %v", route2Order)
	}
}

// TestMiddlewareOrdering_ShortCircuit verifies that middleware can short-circuit
// the chain by not calling next().
func TestMiddlewareOrdering_ShortCircuit(t *testing.T) {
	// Given: A router with middleware that short-circuits
	r := nethttp.NewRouter()
	
	var executionOrder []string
	
	middleware1 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "middleware1")
			return next(c)
		}
	}
	
	shortCircuitMiddleware := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "short-circuit")
			// Don't call next() - short circuit the chain
			return c.String(http.StatusUnauthorized, "unauthorized")
		}
	}
	
	middleware3 := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "middleware3")
			return next(c)
		}
	}
	
	// When: Middleware is applied with a short-circuit in the middle
	r.Use(middleware1, shortCircuitMiddleware, middleware3)
	
	r.GET("/test", func(c router.Context) error {
		executionOrder = append(executionOrder, "handler")
		return c.String(http.StatusOK, "ok")
	})
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	// Then: Only middleware before the short-circuit should execute
	expectedOrder := []string{"middleware1", "short-circuit"}
	
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected %d execution steps, got %d", len(expectedOrder), len(executionOrder))
	}
	
	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("execution order mismatch at step %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
	
	// And: The response should be from the short-circuit middleware
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestMiddlewareOrdering_RealWorldStack verifies a realistic middleware stack
// with RequestID, Logging, Recovery, and custom middleware.
func TestMiddlewareOrdering_RealWorldStack(t *testing.T) {
	// Given: A router with a realistic middleware stack
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	
	var executionOrder []string
	
	customMiddleware := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			executionOrder = append(executionOrder, "custom")
			return next(c)
		}
	}
	
	// When: Middleware is applied in a typical order:
	// RequestID -> Logging -> Recovery -> Custom
	r.Use(requestid.RequestID(), logging.Logging(log), recovery.Recovery(log), customMiddleware)
	
	r.GET("/test", func(c router.Context) error {
		executionOrder = append(executionOrder, "handler")
		// Verify request ID is available (set by RequestID middleware)
		requestID := requestid.GetRequestID(c.Request().Context())
		if requestID == "" {
			t.Error("expected request ID to be set")
		}
		return c.String(http.StatusOK, "ok")
	})
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	// Then: Custom middleware should execute after RequestID, Logging, and Recovery
	expectedOrder := []string{"custom", "handler"}
	
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected %d execution steps, got %d", len(expectedOrder), len(executionOrder))
	}
	
	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("execution order mismatch at step %d: expected %s, got %s", i, expected, executionOrder[i])
		}
	}
	
	// And: Request ID should be in response headers
	if w.Header().Get(requestid.RequestIDHeader) == "" {
		t.Error("expected X-Request-ID header in response")
	}
	
	// And: Logging middleware should have logged the request
	if len(log.Logs) < 2 {
		t.Errorf("expected at least 2 log entries, got %d", len(log.Logs))
	}
}
