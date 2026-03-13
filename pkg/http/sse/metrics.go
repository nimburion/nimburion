package sse

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	sseRedisOpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimburion_http_sse_redis_operations_total",
		Help: "Total number of SSE redis bus/store operations.",
	}, []string{"component", "operation", "status"})
)

func recordSSERedisOp(component, operation string, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	sseRedisOpsTotal.WithLabelValues(component, operation, status).Inc()
}
