package jobs

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	jobsEnqueuedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_jobs_enqueued_total",
			Help: "Total number of jobs enqueued",
		},
		[]string{"backend", "queue", "job_name"},
	)

	jobsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_jobs_processed_total",
			Help: "Total number of jobs processed by workers",
		},
		[]string{"queue", "job_name", "status"},
	)

	jobsRetryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_jobs_retry_total",
			Help: "Total number of job retries scheduled by workers",
		},
		[]string{"queue", "job_name"},
	)

	jobsDLQTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nimburion_jobs_dlq_total",
			Help: "Total number of jobs moved to dead-letter queues",
		},
		[]string{"queue", "job_name"},
	)

	jobsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nimburion_jobs_inflight",
			Help: "Current number of in-flight jobs being processed by workers",
		},
		[]string{"queue"},
	)
)

func recordJobEnqueued(backend string, job *Job) {
	if job == nil {
		return
	}
	jobsEnqueuedTotal.WithLabelValues(
		normalizeMetricLabel(backend, "unknown"),
		normalizeMetricLabel(job.Queue, "unknown"),
		normalizeMetricLabel(job.Name, "unknown"),
	).Inc()
}

func recordJobProcessed(queue, jobName, status string) {
	jobsProcessedTotal.WithLabelValues(
		normalizeMetricLabel(queue, "unknown"),
		normalizeMetricLabel(jobName, "unknown"),
		normalizeMetricLabel(status, "unknown"),
	).Inc()
}

func recordJobRetry(queue, jobName string) {
	jobsRetryTotal.WithLabelValues(
		normalizeMetricLabel(queue, "unknown"),
		normalizeMetricLabel(jobName, "unknown"),
	).Inc()
}

func recordJobDLQ(queue, jobName string) {
	jobsDLQTotal.WithLabelValues(
		normalizeMetricLabel(queue, "unknown"),
		normalizeMetricLabel(jobName, "unknown"),
	).Inc()
}

func incrementJobInFlight(queue string) {
	jobsInFlight.WithLabelValues(normalizeMetricLabel(queue, "unknown")).Inc()
}

func decrementJobInFlight(queue string) {
	jobsInFlight.WithLabelValues(normalizeMetricLabel(queue, "unknown")).Dec()
}

func normalizeMetricLabel(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
