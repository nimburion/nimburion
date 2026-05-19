// Package metrics provides HTTP metrics middleware.
package metrics

import (
	"time"

	"github.com/nimburion/nimburion/pkg/http/router"
	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// Metrics creates middleware that records Prometheus metrics for HTTP requests.
// It tracks:
// - HTTP request duration histogram (by method, path, status)
// - HTTP request counter (by method, path, status)
// - In-flight requests gauge
//
// Requirements: 13.2, 13.3, 13.4
func Metrics() router.MiddlewareFunc {
	return MetricsWithRegistry(frameworkmetrics.DefaultRegistry())
}

// MetricsWithRegistry creates middleware that records Prometheus metrics for HTTP requests
// on the provided registry. When registry is nil, the default registry is used.
func MetricsWithRegistry(registry *frameworkmetrics.Registry) router.MiddlewareFunc {
	if registry == nil {
		registry = frameworkmetrics.DefaultRegistry()
	}

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			registry.IncrementInFlight()
			defer registry.DecrementInFlight()

			start := time.Now()
			err := next(c)
			duration := time.Since(start)
			status := c.Response().Status()

			registry.RecordHTTPMetrics(
				c.Request().Method,
				c.Request().URL.Path,
				status,
				duration,
			)

			return err
		}
	}
}
