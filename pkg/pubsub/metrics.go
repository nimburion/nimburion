package pubsub

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var pubsubRedisStoreOpsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "nimburion_pubsub_redis_store_operations_total",
	Help: "Total number of pubsub redis store operations.",
}, []string{"operation", "status"})

func recordPubsubRedisStoreOp(operation string, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	pubsubRedisStoreOpsTotal.WithLabelValues(operation, status).Inc()
}
