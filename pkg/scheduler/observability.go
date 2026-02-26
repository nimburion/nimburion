package scheduler

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	schedulerDispatchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_scheduler_dispatch_total",
			Help: "Total number of scheduler dispatch attempts",
		},
		[]string{"task", "status"},
	)

	schedulerDispatchInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nimburion_scheduler_dispatch_inflight",
			Help: "Current number of in-flight scheduler dispatch operations",
		},
		[]string{"task"},
	)

	schedulerLockRenewTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_scheduler_lock_renew_total",
			Help: "Total number of scheduler lock renew operations",
		},
		[]string{"task", "status"},
	)
)

func recordSchedulerDispatch(taskName, status string) {
	schedulerDispatchTotal.WithLabelValues(
		normalizeSchedulerLabel(taskName),
		normalizeSchedulerLabel(status),
	).Inc()
}

func incrementSchedulerDispatchInFlight(taskName string) {
	schedulerDispatchInFlight.WithLabelValues(normalizeSchedulerLabel(taskName)).Inc()
}

func decrementSchedulerDispatchInFlight(taskName string) {
	schedulerDispatchInFlight.WithLabelValues(normalizeSchedulerLabel(taskName)).Dec()
}

func recordSchedulerLockRenew(taskName, status string) {
	schedulerLockRenewTotal.WithLabelValues(
		normalizeSchedulerLabel(taskName),
		normalizeSchedulerLabel(status),
	).Inc()
}

func normalizeSchedulerLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
