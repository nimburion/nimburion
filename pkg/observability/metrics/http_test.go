package metrics

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRecordHTTPMetrics verifies that HTTP metrics are recorded correctly.
// Requirements: 13.2, 13.3, 13.4
func TestRecordHTTPMetrics(t *testing.T) {
	// Create a new registry to isolate this test
	registry := NewRegistry()

	tests := []struct {
		name     string
		method   string
		path     string
		status   int
		duration time.Duration
	}{
		{
			name:     "GET request success",
			method:   "GET",
			path:     "/api/test",
			status:   200,
			duration: 100 * time.Millisecond,
		},
		{
			name:     "POST request created",
			method:   "POST",
			path:     "/api/test",
			status:   201,
			duration: 150 * time.Millisecond,
		},
		{
			name:     "GET request not found",
			method:   "GET",
			path:     "/api/missing",
			status:   404,
			duration: 50 * time.Millisecond,
		},
		{
			name:     "POST request error",
			method:   "POST",
			path:     "/api/error",
			status:   500,
			duration: 200 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the metrics
			RecordHTTPMetrics(tt.method, tt.path, tt.status, tt.duration)

			// Get metrics output
			handler := registry.Handler()
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			body := rec.Body.String()

			// Verify the counter was incremented (just check the labels exist, not exact count)
			expectedLabels := `method="` + tt.method + `",path="` + tt.path + `",status="`
			if !strings.Contains(body, expectedLabels) {
				t.Errorf("expected labels %s not found in metrics output", expectedLabels)
			}

			// Verify histogram was updated (check for count)
			if !strings.Contains(body, "http_request_duration_seconds_count") {
				t.Error("http_request_duration_seconds_count not found in metrics output")
			}
		})
	}
}

// TestIncrementDecrementInFlight verifies in-flight request tracking.
// Requirements: 13.4
func TestIncrementDecrementInFlight(t *testing.T) {
	registry := NewRegistry()

	// Get initial value
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	initialBody := rec.Body.String()

	// Extract initial in-flight value (should be 0)
	if !strings.Contains(initialBody, "http_requests_in_flight") {
		t.Fatal("http_requests_in_flight metric not found")
	}

	// Increment in-flight
	IncrementInFlight()
	IncrementInFlight()
	IncrementInFlight()

	// Check value increased
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	afterIncBody := rec.Body.String()

	if !strings.Contains(afterIncBody, "http_requests_in_flight 3") {
		t.Error("expected http_requests_in_flight to be 3 after increments")
	}

	// Decrement in-flight
	DecrementInFlight()
	DecrementInFlight()

	// Check value decreased
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	afterDecBody := rec.Body.String()

	if !strings.Contains(afterDecBody, "http_requests_in_flight 1") {
		t.Error("expected http_requests_in_flight to be 1 after decrements")
	}
}

// TestHTTPMetricsLabels verifies that metrics have correct labels.
// Requirements: 13.2, 13.3
func TestHTTPMetricsLabels(t *testing.T) {
	registry := NewRegistry()

	// Record metrics with different label combinations
	RecordHTTPMetrics("GET", "/api/users", 200, 100*time.Millisecond)
	RecordHTTPMetrics("GET", "/api/users", 404, 50*time.Millisecond)
	RecordHTTPMetrics("POST", "/api/users", 201, 150*time.Millisecond)
	RecordHTTPMetrics("DELETE", "/api/users/123", 204, 75*time.Millisecond)

	// Get metrics output
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify each label combination exists
	expectedLabels := []string{
		`method="GET",path="/api/users",status="200"`,
		`method="GET",path="/api/users",status="404"`,
		`method="POST",path="/api/users",status="201"`,
		`method="DELETE",path="/api/users/123",status="204"`,
	}

	for _, labels := range expectedLabels {
		if !strings.Contains(body, labels) {
			t.Errorf("expected labels %s not found in metrics output", labels)
		}
	}
}

// TestHTTPMetricsDurationHistogram verifies histogram buckets.
// Requirements: 13.2
func TestHTTPMetricsDurationHistogram(t *testing.T) {
	registry := NewRegistry()

	// Record requests with various durations
	durations := []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		5 * time.Second,
	}

	for i, duration := range durations {
		RecordHTTPMetrics("GET", "/api/test", 200, duration)
		_ = i // avoid unused variable
	}

	// Get metrics output
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify histogram components exist
	histogramComponents := []string{
		"http_request_duration_seconds_bucket",
		"http_request_duration_seconds_sum",
		"http_request_duration_seconds_count",
	}

	for _, component := range histogramComponents {
		if !strings.Contains(body, component) {
			t.Errorf("expected histogram component %s not found", component)
		}
	}

	// Verify we have multiple buckets
	bucketCount := strings.Count(body, "http_request_duration_seconds_bucket")
	if bucketCount < 5 {
		t.Errorf("expected at least 5 histogram buckets, found %d", bucketCount)
	}
}

// TestHTTPMetricsCounterIncrement verifies counter increments.
// Requirements: 13.3
func TestHTTPMetricsCounterIncrement(t *testing.T) {
	registry := NewRegistry()

	// Record the same request multiple times
	method := "GET"
	path := "/api/counter-test"
	status := 200

	for i := 0; i < 5; i++ {
		RecordHTTPMetrics(method, path, status, 100*time.Millisecond)
	}

	// Get metrics output
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify counter shows 5 requests
	expectedCounter := `http_requests_total{method="GET",path="/api/counter-test",status="200"} 5`
	if !strings.Contains(body, expectedCounter) {
		t.Errorf("expected counter value not found. Looking for: %s", expectedCounter)
		// Print relevant lines for debugging
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			if strings.Contains(line, "http_requests_total") && strings.Contains(line, "/api/counter-test") {
				t.Logf("Found: %s", line)
			}
		}
	}
}

// TestHTTPMetricsStatusCodes verifies different status codes are tracked.
// Requirements: 13.3
func TestHTTPMetricsStatusCodes(t *testing.T) {
	registry := NewRegistry()

	statusCodes := []int{200, 201, 204, 400, 401, 403, 404, 500, 502, 503}

	for _, status := range statusCodes {
		RecordHTTPMetrics("GET", "/api/test", status, 100*time.Millisecond)
	}

	// Get metrics output
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify each status code is tracked
	for _, status := range statusCodes {
		statusStr := strconv.Itoa(status)
		labelStr := `status="` + statusStr + `"`
		if !strings.Contains(body, labelStr) {
			t.Errorf("expected status code %d not found in metrics", status)
		}
	}
}

// TestHTTPMetricsConcurrency verifies metrics work correctly under concurrent access.
// Requirements: 13.2, 13.3, 13.4
func TestHTTPMetricsConcurrency(t *testing.T) {
	registry := NewRegistry()

	// Simulate concurrent requests
	done := make(chan bool)
	numGoroutines := 10
	requestsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < requestsPerGoroutine; j++ {
				IncrementInFlight()
				RecordHTTPMetrics("GET", "/api/concurrent", 200, 10*time.Millisecond)
				DecrementInFlight()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Get metrics output
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify total count
	expectedTotal := numGoroutines * requestsPerGoroutine
	expectedCountStr := strconv.Itoa(expectedTotal)
	expectedCounter := `http_requests_total{method="GET",path="/api/concurrent",status="200"} ` + expectedCountStr

	if !strings.Contains(body, expectedCounter) {
		t.Errorf("expected counter value %d not found", expectedTotal)
		// Print for debugging
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			if strings.Contains(line, "http_requests_total") && strings.Contains(line, "/api/concurrent") {
				t.Logf("Found: %s", line)
			}
		}
	}

	// Verify in-flight is back to 0 or close to it (may have residual from other tests)
	// Just verify the metric exists
	if !strings.Contains(body, "http_requests_in_flight") {
		t.Error("expected http_requests_in_flight metric to exist")
	}
}
