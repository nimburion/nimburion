package factory

import (
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/eventbus"
	eventbusfactory "github.com/nimburion/nimburion/pkg/eventbus/factory"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	BackendEventBus = config.JobsBackendEventBus
)

// Config configures jobs runtime adapter selection.
type Config = config.JobsConfig

type eventBusAdapterFactory func(
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (eventbus.EventBus, error)

// NewRuntime creates a jobs runtime adapter from config.
// Default backend is eventbus to remain aligned with existing Nimburion async integrations.
func NewRuntime(cfg Config, eventBusCfg config.EventBusConfig, log logger.Logger) (jobs.Runtime, error) {
	return newRuntime(cfg, eventBusCfg, config.KafkaValidationConfig{}, log, eventbusfactory.NewEventBusAdapterWithValidation)
}

// NewRuntimeWithValidation creates a jobs runtime and applies eventbus schema validation when enabled.
func NewRuntimeWithValidation(
	cfg Config,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Runtime, error) {
	return newRuntime(cfg, eventBusCfg, validationCfg, log, eventbusfactory.NewEventBusAdapterWithValidation)
}

func newRuntime(
	cfg Config,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
	newEventBus eventBusAdapterFactory,
) (jobs.Runtime, error) {
	if newEventBus == nil {
		return nil, fmt.Errorf("eventbus factory is required")
	}

	backend := strings.ToLower(strings.TrimSpace(cfg.Backend))
	if backend == "" {
		backend = BackendEventBus
	}

	switch backend {
	case BackendEventBus:
		bus, err := newEventBus(eventBusCfg, validationCfg, log)
		if err != nil {
			return nil, err
		}
		return jobs.NewEventBusBridge(bus)
	default:
		return nil, fmt.Errorf("unsupported jobs.backend %q (supported: %s)", cfg.Backend, BackendEventBus)
	}
}
