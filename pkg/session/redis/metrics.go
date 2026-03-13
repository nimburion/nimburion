package redis

import "github.com/prometheus/client_golang/prometheus/promauto"
import "github.com/prometheus/client_golang/prometheus"

var (
	sessionRedisOpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimburion_session_redis_operations_total",
		Help: "Total number of session redis adapter operations.",
	}, []string{"operation", "status"})
)

func recordSessionRedisOp(operation string, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	sessionRedisOpsTotal.WithLabelValues(operation, status).Inc()
}
