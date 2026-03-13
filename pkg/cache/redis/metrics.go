package redis

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheRedisOpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimburion_cache_redis_operations_total",
		Help: "Total number of cache redis adapter operations.",
	}, []string{"operation", "status"})
)

func recordCacheRedisOp(operation string, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	cacheRedisOpsTotal.WithLabelValues(operation, status).Inc()
}
