package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/eventbus/kafka"
	"github.com/nimburion/nimburion/pkg/eventbus/rabbitmq"
	"github.com/nimburion/nimburion/pkg/eventbus/schema"
	"github.com/nimburion/nimburion/pkg/eventbus/sqs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Cosa fa: seleziona e inizializza l'event bus adapter in base alla config.
// Cosa NON fa: non supporta multipli bus attivi nella stessa factory call.
// Esempio minimo: bus, err := eventbus.NewEventBusAdapter(cfg.EventBus, log)
func NewEventBusAdapter(cfg config.EventBusConfig, log logger.Logger) (eventbus.EventBus, error) {
	return NewEventBusAdapterWithValidation(cfg, config.KafkaValidationConfig{}, log)
}

// NewEventBusAdapterWithValidation creates an adapter and optionally applies schema validation hooks.
func NewEventBusAdapterWithValidation(
	cfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (eventbus.EventBus, error) {
	var (
		base eventbus.EventBus
		err  error
	)

	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "kafka":
		base, err = kafka.NewKafkaAdapter(kafka.Config{
			Brokers:          cfg.Brokers,
			OperationTimeout: cfg.OperationTimeout,
			GroupID:          cfg.GroupID,
		}, log)
	case "rabbitmq":
		url := cfg.URL
		if url == "" && len(cfg.Brokers) > 0 {
			url = cfg.Brokers[0]
		}
		base, err = rabbitmq.NewRabbitMQAdapter(rabbitmq.Config{
			URL:              url,
			Exchange:         cfg.Exchange,
			ExchangeType:     cfg.ExchangeType,
			QueueName:        cfg.QueueName,
			RoutingKey:       cfg.RoutingKey,
			OperationTimeout: cfg.OperationTimeout,
			ConsumerTag:      cfg.ConsumerTag,
		}, log)
	case "sqs":
		base, err = sqs.NewSQSAdapter(sqs.Config{
			Region:            cfg.Region,
			QueueURL:          cfg.QueueURL,
			Endpoint:          cfg.Endpoint,
			AccessKeyID:       cfg.AccessKeyID,
			SecretAccessKey:   cfg.SecretAccessKey,
			SessionToken:      cfg.SessionToken,
			OperationTimeout:  cfg.OperationTimeout,
			WaitTimeSeconds:   cfg.WaitTimeSeconds,
			MaxMessages:       cfg.MaxMessages,
			VisibilityTimeout: cfg.VisibilityTimeout,
		}, log)
	default:
		return nil, fmt.Errorf("unsupported eventbus.type %q (supported: kafka, rabbitmq, sqs)", cfg.Type)
	}
	if err != nil {
		return nil, err
	}

	return wrapWithSchemaValidation(base, validationCfg, log)
}

func wrapWithSchemaValidation(
	base eventbus.EventBus,
	cfg config.KafkaValidationConfig,
	log logger.Logger,
) (eventbus.EventBus, error) {
	if base == nil || !cfg.Enabled {
		return base, nil
	}

	registry, err := schema.NewLocalRegistry(cfg.DescriptorPath)
	if err != nil {
		mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
		if mode == "warn" {
			log.Warn("eventbus schema validation disabled: descriptor load failed", "error", err)
			return base, nil
		}
		return nil, err
	}

	mode := schema.ValidationMode(strings.ToLower(strings.TrimSpace(cfg.Mode)))
	producerValidator := schema.NewMessageProducerValidator(registry, mode)
	consumerValidator := schema.NewMessageConsumerValidator(registry, mode)

	return &validatedEventBus{
		base:              base,
		producerValidator: producerValidator,
		consumerValidator: consumerValidator,
	}, nil
}

type validatedEventBus struct {
	base              eventbus.EventBus
	producerValidator schema.ProducerValidator
	consumerValidator schema.ConsumerValidator
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

func (v *validatedEventBus) Unsubscribe(topic string) error {
	return v.base.Unsubscribe(topic)
}

func (v *validatedEventBus) HealthCheck(ctx context.Context) error {
	return v.base.HealthCheck(ctx)
}

func (v *validatedEventBus) Close() error {
	return v.base.Close()
}
