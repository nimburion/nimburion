package cache

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheResultsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_http_cache_results_total",
			Help: "Total HTTP cache outcomes.",
		},
		[]string{"result"},
	)
	cacheLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nimburion_http_cache_latency_seconds",
			Help:    "HTTP cache operation latency in seconds.",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		},
		[]string{"operation"},
	)
)

func incCacheResult(result string) {
	cacheResultsTotal.WithLabelValues(result).Inc()
}

func observeCacheLatency(operation string, d time.Duration) {
	cacheLatencySeconds.WithLabelValues(operation).Observe(d.Seconds())
}
