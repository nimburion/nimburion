// Package metrics provides Prometheus metrics integration for the framework.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

// Registry manages Prometheus metrics registration and exposure.
// It provides a central place to register custom metrics and includes
// HTTP metrics and Go runtime metrics by default.
//
// Requirements: 13.1, 13.5, 13.6, 13.7
type Registry struct {
	registry *prometheus.Registry
}

// NewRegistry creates a new metrics registry with default collectors.
// It automatically registers:
// - HTTP request metrics (duration, counter, in-flight)
// - Go runtime metrics (goroutines, memory, GC)
//
// Requirements: 13.1, 13.5, 13.6
func NewRegistry() *Registry {
	reg := prometheus.NewRegistry()

	// Register HTTP metrics
	reg.MustRegister(httpRequestDuration)
	reg.MustRegister(httpRequestsTotal)
	reg.MustRegister(httpRequestsInFlight)

	// Register Go runtime metrics (goroutines, memory, GC)
	// Requirements: 13.6
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	return &Registry{
		registry: reg,
	}
}

// Register registers a custom Prometheus collector.
// This allows applications to add their own metrics beyond the default HTTP and runtime metrics.
//
// Example:
//
//	customCounter := prometheus.NewCounter(prometheus.CounterOpts{
//	    Name: "my_custom_counter",
//	    Help: "A custom counter metric",
//	})
//	registry.Register(customCounter)
//
// Requirements: 13.5
func (r *Registry) Register(collector prometheus.Collector) error {
	return r.registry.Register(collector)
}

// MustRegister registers a custom Prometheus collector and panics on error.
// Use this for metrics that must be registered at startup.
//
// Requirements: 13.5
func (r *Registry) MustRegister(collectors ...prometheus.Collector) {
	r.registry.MustRegister(collectors...)
}

// Unregister removes a collector from the registry.
// This is primarily useful for testing.
//
// Requirements: 13.5
func (r *Registry) Unregister(collector prometheus.Collector) bool {
	return r.registry.Unregister(collector)
}

// Handler returns an HTTP handler that exposes metrics in Prometheus format.
// This handler should be mounted on the management server at /metrics.
//
// Example:
//
//	http.Handle("/metrics", registry.Handler())
//
// Requirements: 13.1, 13.7
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Gatherer returns the underlying prometheus.Gatherer.
// This is useful for advanced use cases like custom metric exposition.
func (r *Registry) Gatherer() prometheus.Gatherer {
	return r.registry
}
