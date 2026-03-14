package sse

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// Metrics captures Redis-backed SSE Prometheus collectors.
type Metrics struct {
	opsTotal *prometheus.CounterVec
}

// NewMetrics registers Redis SSE metrics in the provided framework registry.
func NewMetrics(registry *frameworkmetrics.Registry) (*Metrics, error) {
	if registry == nil {
		return &Metrics{}, nil
	}
	opsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nimburion_http_sse_redis_operations_total",
		Help: "Total number of SSE redis bus/store operations.",
	}, []string{"component", "operation", "status"})
	if err := registry.Register(opsTotal); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegistered) {
			return nil, coreerrors.WrapConstructorError("NewMetrics", fmt.Errorf("register sse redis metrics: %w", err))
		}
		existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec)
		if !ok {
			return nil, coreerrors.WrapConstructorError("NewMetrics", fmt.Errorf("register sse redis metrics: existing collector has type %T", alreadyRegistered.ExistingCollector))
		}
		opsTotal = existing
	}
	return &Metrics{opsTotal: opsTotal}, nil
}

func (m *Metrics) record(component, operation string, err error) {
	if m == nil || m.opsTotal == nil {
		return
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	m.opsTotal.WithLabelValues(component, operation, status).Inc()
}
