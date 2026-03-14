package pubsub

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

func TestMetrics_RegisterInCustomRegistryOnly(t *testing.T) {
	registry := frameworkmetrics.NewRegistry()

	metrics, err := NewMetrics(registry)
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	metricsAgain, err := NewMetrics(registry)
	if err != nil {
		t.Fatalf("NewMetrics() second call error = %v", err)
	}

	metrics.record("append", nil)
	metricsAgain.record("recent", nil)

	assertMetricPresent(t, registry.Gatherer(), "nimburion_pubsub_redis_store_operations_total")
	assertMetricAbsent(t, prometheus.DefaultGatherer, "nimburion_pubsub_redis_store_operations_total")
}

func assertMetricPresent(t *testing.T, gatherer prometheus.Gatherer, name string) {
	t.Helper()

	metricFamilies, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range metricFamilies {
		if family.GetName() == name {
			return
		}
	}
	t.Fatalf("expected metric %q in custom registry", name)
}

func assertMetricAbsent(t *testing.T, gatherer prometheus.Gatherer, name string) {
	t.Helper()

	metricFamilies, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range metricFamilies {
		if family.GetName() == name {
			t.Fatalf("metric %q unexpectedly registered in default gatherer", name)
		}
	}
}
