// Package metrics provides Prometheus metrics for HTTP requests.
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	httpRequestDurationName  = "http_request_duration_seconds"
	httpRequestsTotalName    = "http_requests_total"
	httpRequestsInFlightName = "http_requests_in_flight"
)

func newHTTPRequestDurationCollector() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    httpRequestDurationName,
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
}

func newHTTPRequestsTotalCollector() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: httpRequestsTotalName,
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
}

func newHTTPRequestsInFlightCollector() prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: httpRequestsInFlightName,
			Help: "Current number of HTTP requests being processed",
		},
	)
}

var defaultRegistry = NewRegistry()

// DefaultRegistry returns the framework-wide default metrics registry.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// RecordHTTPMetrics records HTTP request metrics on the default registry.
func RecordHTTPMetrics(method, path string, status int, duration time.Duration) {
	defaultRegistry.RecordHTTPMetrics(method, path, status, duration)
}

// IncrementInFlight increments the in-flight requests gauge on the default registry.
func IncrementInFlight() {
	defaultRegistry.IncrementInFlight()
}

// DecrementInFlight decrements the in-flight requests gauge on the default registry.
func DecrementInFlight() {
	defaultRegistry.DecrementInFlight()
}

// RecordHTTPMetrics records HTTP request metrics on this registry.
func (r *Registry) RecordHTTPMetrics(method, path string, status int, duration time.Duration) {
	if r == nil {
		return
	}
	statusStr := strconv.Itoa(status)
	r.httpRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	r.httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
}

// IncrementInFlight increments the in-flight requests gauge on this registry.
func (r *Registry) IncrementInFlight() {
	if r == nil {
		return
	}
	r.httpRequestsInFlight.Inc()
}

// DecrementInFlight decrements the in-flight requests gauge on this registry.
func (r *Registry) DecrementInFlight() {
	if r == nil {
		return
	}
	r.httpRequestsInFlight.Dec()
}
