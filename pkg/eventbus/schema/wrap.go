package schema

import (
	"context"
	"strings"

	"github.com/nimburion/nimburion/pkg/eventbus"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Wrap decorates an event bus with schema validation when enabled.
func Wrap(base eventbus.EventBus, cfg schemavalidationconfig.KafkaValidationConfig, log logger.Logger) (eventbus.EventBus, error) {
	if base == nil || !cfg.Enabled {
		return base, nil
	}
	registry, err := NewLocalRegistry(cfg.DescriptorPath)
	if err != nil {
		mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
		if mode == "warn" {
			log.Warn("eventbus schema validation disabled: descriptor load failed", "error", err)
			return base, nil
		}
		return nil, err
	}
	mode := ValidationMode(strings.ToLower(strings.TrimSpace(cfg.Mode)))
	producerValidator := NewMessageProducerValidator(registry, mode)
	consumerValidator := NewMessageConsumerValidator(registry, mode)
	return &validatedEventBus{base: base, producerValidator: producerValidator, consumerValidator: consumerValidator}, nil
}

type validatedEventBus struct {
	base              eventbus.EventBus
	producerValidator ProducerValidator
	consumerValidator ConsumerValidator
}

func (v *validatedEventBus) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	if v.producerValidator != nil {
		if err := v.producerValidator.ValidateBeforePublish(ctx, topic, message); err != nil {
			return err
		}
	}
	return v.base.Publish(ctx, topic, message)
}

func (v *validatedEventBus) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	if v.producerValidator != nil {
		for _, message := range messages {
			if err := v.producerValidator.ValidateBeforePublish(ctx, topic, message); err != nil {
				return err
			}
		}
	}
	return v.base.PublishBatch(ctx, topic, messages)
}

func (v *validatedEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	if v.consumerValidator == nil || handler == nil {
		return v.base.Subscribe(ctx, topic, handler)
	}
	return v.base.Subscribe(ctx, topic, func(handlerCtx context.Context, msg *eventbus.Message) error {
		if err := v.consumerValidator.ValidateAfterConsume(handlerCtx, topic, msg); err != nil {
			return err
		}
		return handler(handlerCtx, msg)
	})
}

func (v *validatedEventBus) Unsubscribe(topic string) error        { return v.base.Unsubscribe(topic) }
func (v *validatedEventBus) HealthCheck(ctx context.Context) error { return v.base.HealthCheck(ctx) }
func (v *validatedEventBus) Close() error                          { return v.base.Close() }
