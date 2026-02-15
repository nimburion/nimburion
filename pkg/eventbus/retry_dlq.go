package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/resilience"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DefaultRetryMaxRetries             = 5
	DefaultRetryInitialBackoff         = time.Second
	DefaultRetryMaxBackoff             = 60 * time.Second
	DefaultRetryAttemptTimeout         = 30 * time.Second
	DefaultRetryCircuitBreakerFailures = 5
	DefaultRetryCircuitBreakerReset    = 30 * time.Second
	DefaultDLQTopicSuffix              = ".dlq"
)

// DLQPayload describes the payload forwarded to dead-letter topic.
type DLQPayload struct {
	OriginalTopic  string            `json:"original_topic"`
	OriginalID     string            `json:"original_id"`
	OriginalKey    string            `json:"original_key"`
	OriginalBody   []byte            `json:"original_body"`
	OriginalHeader map[string]string `json:"original_header,omitempty"`
	FailureReason  string            `json:"failure_reason"`
	StackTrace     string            `json:"stack_trace"`
	AttemptCount   int               `json:"attempt_count"`
	FailedAt       time.Time         `json:"failed_at"`
}

// RetryDLQConfig controls retry and dead-letter behavior.
type RetryDLQConfig struct {
	MaxRetries             int
	InitialBackoff         time.Duration
	MaxBackoff             time.Duration
	AttemptTimeout         time.Duration
	CircuitBreakerFailures int
	CircuitBreakerReset    time.Duration
	DLQTopicSuffix         string
}

func (c *RetryDLQConfig) normalize() {
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = DefaultRetryMaxRetries
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = DefaultRetryInitialBackoff
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = DefaultRetryMaxBackoff
	}
	if c.AttemptTimeout <= 0 {
		c.AttemptTimeout = DefaultRetryAttemptTimeout
	}
	if c.CircuitBreakerFailures <= 0 {
		c.CircuitBreakerFailures = DefaultRetryCircuitBreakerFailures
	}
	if c.CircuitBreakerReset <= 0 {
		c.CircuitBreakerReset = DefaultRetryCircuitBreakerReset
	}
	if c.DLQTopicSuffix == "" {
		c.DLQTopicSuffix = DefaultDLQTopicSuffix
	}
}

// RetryDLQMetrics exports retry and DLQ signals.
type RetryDLQMetrics struct {
	retryTotal      *prometheus.CounterVec
	processedTotal  prometheus.Counter
	dlqTotal        prometheus.Counter
	dlqFailureTotal prometheus.Counter
	dlqSizeGauge    prometheus.Gauge
	dlqGrowthGauge  prometheus.Gauge
}

// NewRetryDLQMetrics registers retry + DLQ metrics in a Prometheus registry.
func NewRetryDLQMetrics(registry *prometheus.Registry, namespace string) (*RetryDLQMetrics, error) {
	if registry == nil {
		return nil, errors.New("registry is nil")
	}
	if namespace == "" {
		namespace = "nimburion"
	}

	retryTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "event_consumer",
		Name:      "retry_total",
		Help:      "Total retry attempts grouped by failure reason.",
	}, []string{"reason"})
	processedTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "event_consumer",
		Name:      "processed_total",
		Help:      "Total successfully processed messages.",
	})
	dlqTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "event_consumer",
		Name:      "dlq_total",
		Help:      "Total messages sent to dead-letter queue.",
	})
	dlqFailureTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "event_consumer",
		Name:      "dlq_failure_total",
		Help:      "Total failures while publishing to dead-letter queue.",
	})
	dlqSizeGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "event_consumer",
		Name:      "dlq_size",
		Help:      "Current DLQ size as provided by the caller.",
	})
	dlqGrowthGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "event_consumer",
		Name:      "dlq_growth_rate",
		Help:      "Current DLQ growth rate as provided by the caller.",
	})

	for _, c := range []prometheus.Collector{retryTotal, processedTotal, dlqTotal, dlqFailureTotal, dlqSizeGauge, dlqGrowthGauge} {
		if err := registry.Register(c); err != nil {
			return nil, fmt.Errorf("register retry/dlq metrics failed: %w", err)
		}
	}

	return &RetryDLQMetrics{
		retryTotal:      retryTotal,
		processedTotal:  processedTotal,
		dlqTotal:        dlqTotal,
		dlqFailureTotal: dlqFailureTotal,
		dlqSizeGauge:    dlqSizeGauge,
		dlqGrowthGauge:  dlqGrowthGauge,
	}, nil
}

func (m *RetryDLQMetrics) incRetry(reason string) {
	if m == nil {
		return
	}
	if reason == "" {
		reason = "unknown"
	}
	m.retryTotal.WithLabelValues(reason).Inc()
}

func (m *RetryDLQMetrics) incProcessed() {
	if m == nil {
		return
	}
	m.processedTotal.Inc()
}

func (m *RetryDLQMetrics) incDLQ() {
	if m == nil {
		return
	}
	m.dlqTotal.Inc()
}

func (m *RetryDLQMetrics) incDLQFailure() {
	if m == nil {
		return
	}
	m.dlqFailureTotal.Inc()
}

// SetDLQHealth updates gauge metrics for DLQ size and growth rate.
func (m *RetryDLQMetrics) SetDLQHealth(size, growthRate float64) {
	if m == nil {
		return
	}
	m.dlqSizeGauge.Set(size)
	m.dlqGrowthGauge.Set(growthRate)
}

// ConsumeWithRetry runs a message handler with retries, exponential backoff and DLQ fallback.
func ConsumeWithRetry(
	ctx context.Context,
	consumerTopic string,
	message *Message,
	handler MessageHandler,
	producer Producer,
	config RetryDLQConfig,
	log logger.Logger,
	metrics *RetryDLQMetrics,
) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if consumerTopic == "" {
		return errors.New("consumer topic is required")
	}
	if message == nil {
		return errors.New("message is nil")
	}
	if handler == nil {
		return errors.New("handler is required")
	}
	if producer == nil {
		return errors.New("producer is required")
	}
	if log == nil {
		return errors.New("logger is required")
	}

	config.normalize()
	breaker := resilience.NewCircuitBreaker(config.CircuitBreakerFailures, config.CircuitBreakerReset)

	var lastErr error
	attempts := config.MaxRetries + 1

	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = breaker.Execute(func() error {
			return resilience.WithTimeout(ctx, config.AttemptTimeout, func(attemptCtx context.Context) error {
				return handler(attemptCtx, message)
			})
		})

		if lastErr == nil {
			metrics.incProcessed()
			return nil
		}

		metrics.incRetry(lastErr.Error())
		if attempt == attempts {
			break
		}

		backoff := exponentialBackoff(attempt, config.InitialBackoff, config.MaxBackoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	dlqTopic := consumerTopic + config.DLQTopicSuffix
	if err := sendToDLQ(ctx, producer, dlqTopic, consumerTopic, message, lastErr, attempts); err != nil {
		metrics.incDLQFailure()
		return fmt.Errorf("handler failed after retries (%v) and dlq publish failed: %w", lastErr, err)
	}

	metrics.incDLQ()
	log.Error("message routed to dead-letter queue",
		"original_topic", consumerTopic,
		"dlq_topic", dlqTopic,
		"message_id", message.ID,
		"attempts", attempts,
		"error", lastErr,
	)
	return fmt.Errorf("handler failed after %d attempts: %w", attempts, lastErr)
}

func sendToDLQ(
	ctx context.Context,
	producer Producer,
	dlqTopic, originalTopic string,
	message *Message,
	failure error,
	attempts int,
) error {
	if producer == nil {
		return errors.New("producer is nil")
	}
	if message == nil {
		return errors.New("message is nil")
	}
	if failure == nil {
		failure = errors.New("unknown failure")
	}

	payload := DLQPayload{
		OriginalTopic:  originalTopic,
		OriginalID:     message.ID,
		OriginalKey:    message.Key,
		OriginalBody:   message.Value,
		OriginalHeader: message.Headers,
		FailureReason:  failure.Error(),
		StackTrace:     string(debug.Stack()),
		AttemptCount:   attempts,
		FailedAt:       time.Now().UTC(),
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dlq payload failed: %w", err)
	}

	dlqMessage := &Message{
		ID:          message.ID,
		Key:         message.Key,
		Value:       encoded,
		Headers:     map[string]string{"dlq": "true", "original_topic": originalTopic},
		ContentType: "application/json",
		Timestamp:   time.Now().UTC(),
	}

	if err := producer.Publish(ctx, dlqTopic, dlqMessage); err != nil {
		return fmt.Errorf("publish dlq message failed: %w", err)
	}
	return nil
}
