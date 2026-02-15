package metrics_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// ExampleNewRegistry demonstrates creating a metrics registry and exposing metrics.
func ExampleNewRegistry() {
	// Create a new metrics registry
	registry := metrics.NewRegistry()

	// Expose metrics on an HTTP endpoint
	http.Handle("/metrics", registry.Handler())

	fmt.Println("Metrics registry created and handler registered")
	// Output: Metrics registry created and handler registered
}

// ExampleRegistry_Register demonstrates registering custom metrics.
func ExampleRegistry_Register() {
	registry := metrics.NewRegistry()

	// Create a custom counter
	ordersProcessed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orders_processed_total",
		Help: "Total number of orders processed",
	})

	// Register the custom metric
	err := registry.Register(ordersProcessed)
	if err != nil {
		fmt.Printf("Failed to register metric: %v\n", err)
		return
	}

	// Use the metric
	ordersProcessed.Inc()

	fmt.Println("Custom metric registered and incremented")
	// Output: Custom metric registered and incremented
}

// ExampleRegistry_MustRegister demonstrates registering multiple custom metrics.
func ExampleRegistry_MustRegister() {
	registry := metrics.NewRegistry()

	// Create custom metrics
	requestsProcessed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "requests_processed_total",
		Help: "Total number of requests processed",
	})

	activeConnections := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "active_connections",
		Help: "Number of active connections",
	})

	// Register multiple metrics at once
	registry.MustRegister(requestsProcessed, activeConnections)

	// Use the metrics
	requestsProcessed.Inc()
	activeConnections.Set(42)

	fmt.Println("Multiple custom metrics registered")
	// Output: Multiple custom metrics registered
}

// ExampleRecordHTTPMetrics demonstrates recording HTTP request metrics.
func ExampleRecordHTTPMetrics() {
	// Record metrics for an HTTP request
	method := "GET"
	path := "/api/users"
	status := 200
	duration := 150 * time.Millisecond

	metrics.RecordHTTPMetrics(method, path, status, duration)

	fmt.Println("HTTP metrics recorded")
	// Output: HTTP metrics recorded
}

// ExampleIncrementInFlight demonstrates tracking in-flight requests.
func ExampleIncrementInFlight() {
	// Increment when request starts
	metrics.IncrementInFlight()

	// Simulate request processing
	// ... handle request ...

	// Decrement when request completes
	defer metrics.DecrementInFlight()

	fmt.Println("In-flight request tracked")
	// Output: In-flight request tracked
}
