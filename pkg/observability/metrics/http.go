// Package metrics provides Prometheus metrics for HTTP requests.
package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// httpRequestDuration tracks HTTP request duration in seconds.
	// Labels: method, path, status
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestsTotal tracks total number of HTTP requests.
	// Labels: method, path, status
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestsInFlight tracks current number of HTTP requests being processed.
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
	)
)

// RecordHTTPMetrics records HTTP request metrics.
// It updates the duration histogram and request counter with the provided labels.
//
// Requirements: 13.2, 13.3, 13.4
func RecordHTTPMetrics(method, path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)
	httpRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
}

// IncrementInFlight increments the in-flight requests gauge.
func IncrementInFlight() {
	httpRequestsInFlight.Inc()
}

// DecrementInFlight decrements the in-flight requests gauge.
func DecrementInFlight() {
	httpRequestsInFlight.Dec()
}
