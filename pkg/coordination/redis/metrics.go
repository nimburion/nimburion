package redis

import "github.com/prometheus/client_golang/prometheus/promauto"
import "github.com/prometheus/client_golang/prometheus"

var (
	coordRedisOpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nimburion_coordination_redis_operations_total",
		Help: "Total number of coordination redis lock operations.",
	}, []string{"operation", "status"})
)

func recordCoordRedisOp(operation string, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	coordRedisOpsTotal.WithLabelValues(operation, status).Inc()
}
