// Package middleware provides HTTP middleware components for the framework.
package metrics

import (
	"time"

	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// Metrics creates middleware that records Prometheus metrics for HTTP requests.
// It tracks:
// - HTTP request duration histogram (by method, path, status)
// - HTTP request counter (by method, path, status)
// - In-flight requests gauge
//
// Requirements: 13.2, 13.3, 13.4
func Metrics() router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Increment in-flight requests gauge
			metrics.IncrementInFlight()
			defer metrics.DecrementInFlight()

			// Record start time
			start := time.Now()

			// Call next handler
			err := next(c)

			// Calculate duration
			duration := time.Since(start)

			// Get response status
			status := c.Response().Status()

			// Record metrics
			metrics.RecordHTTPMetrics(
				c.Request().Method,
				c.Request().URL.Path,
				status,
				duration,
			)

			return err
		}
	}
}
