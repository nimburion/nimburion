package redis

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// Metrics captures Redis coordination lock Prometheus collectors.
type Metrics struct {
	opsTotal *prometheus.CounterVec
}

// NewMetrics registers Redis coordination metrics in the provided framework registry.
func NewMetrics(registry *frameworkmetrics.Registry) (*Metrics, error) {
	if registry == nil {
		return &Metrics{}, nil
	}
	opsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nimburion_coordination_redis_operations_total",
		Help: "Total number of coordination redis lock operations.",
	}, []string{"operation", "status"})
	if err := registry.Register(opsTotal); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegistered) {
			return nil, wrapConstructorError("NewMetrics", fmt.Errorf("register coordination redis metrics: %w", err))
		}
		existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec)
		if !ok {
			return nil, wrapConstructorError("NewMetrics", fmt.Errorf("register coordination redis metrics: existing collector has type %T", alreadyRegistered.ExistingCollector))
		}
		opsTotal = existing
	}
	return &Metrics{opsTotal: opsTotal}, nil
}

func (m *Metrics) record(operation string, err error) {
	if m == nil || m.opsTotal == nil {
		return
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	m.opsTotal.WithLabelValues(operation, status).Inc()
}
