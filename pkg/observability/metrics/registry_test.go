package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestNewRegistry verifies that a new registry is created with default collectors.
// Requirements: 13.1, 13.5, 13.6
func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.registry == nil {
		t.Fatal("registry.registry is nil")
	}
}

// TestRegistry_Handler verifies that the Handler returns a valid HTTP handler.
// Requirements: 13.1, 13.7
func TestRegistry_Handler(t *testing.T) {
	registry := NewRegistry()
	handler := registry.Handler()

	if handler == nil {
		t.Fatal("Handler returned nil")
	}

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}

	// Verify content type
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "application/openmetrics-text") {
		t.Errorf("unexpected content type: %s", contentType)
	}
}

// TestRegistry_HTTPMetricsExposed verifies that HTTP metrics are exposed.
// Requirements: 13.1, 13.2, 13.3, 13.4
func TestRegistry_HTTPMetricsExposed(t *testing.T) {
	registry := NewRegistry()
	handler := registry.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify HTTP metrics are present
	expectedMetrics := []string{
		"http_request_duration_seconds",
		"http_requests_total",
		"http_requests_in_flight",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("expected metric %s not found in output", metric)
		}
	}
}

// TestRegistry_GoRuntimeMetricsExposed verifies that Go runtime metrics are exposed.
// Requirements: 13.6
func TestRegistry_GoRuntimeMetricsExposed(t *testing.T) {
	registry := NewRegistry()
	handler := registry.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify Go runtime metrics are present
	expectedMetrics := []string{
		"go_goroutines",
		"go_threads",
		"go_memstats",
		"go_gc_duration_seconds",
		"process_cpu_seconds_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("expected Go runtime metric %s not found in output", metric)
		}
	}
}

// TestRegistry_RegisterCustomMetric verifies custom metric registration.
// Requirements: 13.5
func TestRegistry_RegisterCustomMetric(t *testing.T) {
	registry := NewRegistry()

	// Create a custom counter
	customCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_custom_counter",
		Help: "A test custom counter",
	})

	// Register the custom metric
	err := registry.Register(customCounter)
	if err != nil {
		t.Fatalf("failed to register custom metric: %v", err)
	}

	// Increment the counter
	customCounter.Inc()

	// Verify the metric is exposed
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "test_custom_counter") {
		t.Error("custom metric not found in output")
	}

	// Verify the value
	if !strings.Contains(body, "test_custom_counter 1") {
		t.Error("custom metric value not correct")
	}
}

// TestRegistry_MustRegisterCustomMetric verifies MustRegister for custom metrics.
// Requirements: 13.5
func TestRegistry_MustRegisterCustomMetric(t *testing.T) {
	registry := NewRegistry()

	// Create custom metrics
	customGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_custom_gauge",
		Help: "A test custom gauge",
	})

	customHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "test_custom_histogram",
		Help: "A test custom histogram",
	})

	// Register using MustRegister
	registry.MustRegister(customGauge, customHistogram)

	// Set values
	customGauge.Set(42.5)
	customHistogram.Observe(1.23)

	// Verify metrics are exposed
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	if !strings.Contains(body, "test_custom_gauge") {
		t.Error("custom gauge not found in output")
	}

	if !strings.Contains(body, "test_custom_histogram") {
		t.Error("custom histogram not found in output")
	}
}

// TestRegistry_MustRegisterPanicsOnDuplicate verifies that MustRegister panics on duplicate registration.
// Requirements: 13.5
func TestRegistry_MustRegisterPanicsOnDuplicate(t *testing.T) {
	registry := NewRegistry()

	customCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_duplicate_counter",
		Help: "A test counter for duplicate registration",
	})

	// First registration should succeed
	registry.MustRegister(customCounter)

	// Second registration should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration, but didn't panic")
		}
	}()

	registry.MustRegister(customCounter)
}

// TestRegistry_Unregister verifies that metrics can be unregistered.
// Requirements: 13.5
func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()

	customCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_unregister_counter",
		Help: "A test counter for unregistration",
	})

	// Register the metric
	err := registry.Register(customCounter)
	if err != nil {
		t.Fatalf("failed to register metric: %v", err)
	}

	// Verify it's present
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "test_unregister_counter") {
		t.Error("metric not found after registration")
	}

	// Unregister the metric
	ok := registry.Unregister(customCounter)
	if !ok {
		t.Error("Unregister returned false")
	}

	// Verify it's no longer present
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body = rec.Body.String()
	if strings.Contains(body, "test_unregister_counter") {
		t.Error("metric still found after unregistration")
	}
}

// TestRegistry_Gatherer verifies that the Gatherer method returns a valid gatherer.
// Requirements: 13.1
func TestRegistry_Gatherer(t *testing.T) {
	registry := NewRegistry()
	gatherer := registry.Gatherer()

	if gatherer == nil {
		t.Fatal("Gatherer returned nil")
	}

	// Verify we can gather metrics
	metricFamilies, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	if len(metricFamilies) == 0 {
		t.Error("expected non-zero metric families")
	}
}

// TestRegistry_HTTPMetricsUpdated verifies that HTTP metrics are updated correctly.
// Requirements: 13.2, 13.3, 13.4
func TestRegistry_HTTPMetricsUpdated(t *testing.T) {
	registry := NewRegistry()

	// Record some HTTP metrics
	RecordHTTPMetrics("GET", "/api/users", 200, 100_000_000) // 100ms
	RecordHTTPMetrics("POST", "/api/users", 201, 150_000_000) // 150ms
	RecordHTTPMetrics("GET", "/api/users", 404, 50_000_000)   // 50ms

	IncrementInFlight()
	IncrementInFlight()
	DecrementInFlight()

	// Get metrics output
	handler := registry.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify request counter labels exist (not checking exact counts due to global metrics)
	expectedLabels := []string{
		`method="GET",path="/api/users",status="200"`,
		`method="POST",path="/api/users",status="201"`,
		`method="GET",path="/api/users",status="404"`,
	}

	for _, labels := range expectedLabels {
		if !strings.Contains(body, labels) {
			t.Errorf("expected labels %s not found in metrics", labels)
		}
	}

	// Verify in-flight gauge exists
	if !strings.Contains(body, "http_requests_in_flight") {
		t.Error("http_requests_in_flight not found")
	}

	// Verify histogram has entries (just check it exists with some count)
	if !strings.Contains(body, "http_request_duration_seconds_count") {
		t.Error("http_request_duration_seconds histogram not found")
	}
}

// TestRegistry_MultipleInstances verifies that multiple registry instances are independent.
// Requirements: 13.1, 13.5
func TestRegistry_MultipleInstances(t *testing.T) {
	registry1 := NewRegistry()
	registry2 := NewRegistry()

	// Register different custom metrics in each registry
	counter1 := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_registry1_counter",
		Help: "Counter for registry 1",
	})

	counter2 := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_registry2_counter",
		Help: "Counter for registry 2",
	})

	registry1.MustRegister(counter1)
	registry2.MustRegister(counter2)

	// Verify registry1 has counter1 but not counter2
	handler1 := registry1.Handler()
	req1 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec1 := httptest.NewRecorder()
	handler1.ServeHTTP(rec1, req1)
	body1 := rec1.Body.String()

	if !strings.Contains(body1, "test_registry1_counter") {
		t.Error("registry1 missing its own counter")
	}
	if strings.Contains(body1, "test_registry2_counter") {
		t.Error("registry1 has registry2's counter")
	}

	// Verify registry2 has counter2 but not counter1
	handler2 := registry2.Handler()
	req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec2 := httptest.NewRecorder()
	handler2.ServeHTTP(rec2, req2)
	body2 := rec2.Body.String()

	if !strings.Contains(body2, "test_registry2_counter") {
		t.Error("registry2 missing its own counter")
	}
	if strings.Contains(body2, "test_registry1_counter") {
		t.Error("registry2 has registry1's counter")
	}
}

// TestRegistry_HandlerContentType verifies the content type of the metrics endpoint.
// Requirements: 13.7
func TestRegistry_HandlerContentType(t *testing.T) {
	registry := NewRegistry()
	handler := registry.Handler()

	tests := []struct {
		name           string
		acceptHeader   string
		wantStatusCode int
	}{
		{
			name:           "no accept header",
			acceptHeader:   "",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "accept text/plain",
			acceptHeader:   "text/plain",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "accept application/openmetrics-text",
			acceptHeader:   "application/openmetrics-text",
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("expected status %d, got %d", tt.wantStatusCode, rec.Code)
			}

			body, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}

			if len(body) == 0 {
				t.Error("expected non-empty response body")
			}
		})
	}
}
