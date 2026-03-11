package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

const (
	// EventBusTypeKafka selects Kafka as the event bus backend.
	EventBusTypeKafka = "kafka"
	// EventBusTypeRabbitMQ selects RabbitMQ as the event bus backend.
	EventBusTypeRabbitMQ = "rabbitmq"
	// EventBusTypeSQS selects Amazon SQS as the event bus backend.
	EventBusTypeSQS = "sqs"
)

// Config configures durable event bus connections.
type Config struct {
	Type              string        `mapstructure:"type"`
	Brokers           []string      `mapstructure:"brokers"`
	Serializer        string        `mapstructure:"serializer"`
	OperationTimeout  time.Duration `mapstructure:"operation_timeout"`
	GroupID           string        `mapstructure:"group_id"`
	URL               string        `mapstructure:"url"`
	Exchange          string        `mapstructure:"exchange"`
	ExchangeType      string        `mapstructure:"exchange_type"`
	QueueName         string        `mapstructure:"queue_name"`
	RoutingKey        string        `mapstructure:"routing_key"`
	ConsumerTag       string        `mapstructure:"consumer_tag"`
	Region            string        `mapstructure:"region"`
	QueueURL          string        `mapstructure:"queue_url"`
	Endpoint          string        `mapstructure:"endpoint"`
	AccessKeyID       string        `mapstructure:"access_key_id"`
	SecretAccessKey   string        `mapstructure:"secret_access_key"`
	SessionToken      string        `mapstructure:"session_token"`
	WaitTimeSeconds   int32         `mapstructure:"wait_time_seconds"`
	MaxMessages       int32         `mapstructure:"max_messages"`
	VisibilityTimeout int32         `mapstructure:"visibility_timeout"`
}

// Extension contributes the eventbus config section as family-owned config surface.
type Extension struct {
	EventBus Config `mapstructure:"eventbus"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"eventbus"} }

// ApplyDefaults registers default event bus configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("eventbus.serializer", "json")
	v.SetDefault("eventbus.operation_timeout", 30*time.Second)
	v.SetDefault("eventbus.group_id", "default-consumer-group")
	v.SetDefault("eventbus.exchange", "events")
	v.SetDefault("eventbus.exchange_type", "topic")
	v.SetDefault("eventbus.wait_time_seconds", int32(10))
	v.SetDefault("eventbus.max_messages", int32(10))
	v.SetDefault("eventbus.visibility_timeout", int32(0))
}

// BindEnv binds event bus configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"eventbus.type", "EVENTBUS_TYPE",
		"eventbus.brokers", "EVENTBUS_BROKERS",
		"eventbus.serializer", "EVENTBUS_SERIALIZER",
		"eventbus.operation_timeout", "EVENTBUS_OPERATION_TIMEOUT",
		"eventbus.group_id", "EVENTBUS_GROUP_ID",
		"eventbus.url", "EVENTBUS_URL",
		"eventbus.exchange", "EVENTBUS_EXCHANGE",
		"eventbus.exchange_type", "EVENTBUS_EXCHANGE_TYPE",
		"eventbus.queue_name", "EVENTBUS_QUEUE_NAME",
		"eventbus.routing_key", "EVENTBUS_ROUTING_KEY",
		"eventbus.consumer_tag", "EVENTBUS_CONSUMER_TAG",
		"eventbus.region", "EVENTBUS_REGION",
		"eventbus.queue_url", "EVENTBUS_QUEUE_URL",
		"eventbus.endpoint", "EVENTBUS_ENDPOINT",
		"eventbus.access_key_id", "EVENTBUS_ACCESS_KEY_ID",
		"eventbus.secret_access_key", "EVENTBUS_SECRET_ACCESS_KEY",
		"eventbus.session_token", "EVENTBUS_SESSION_TOKEN",
		"eventbus.wait_time_seconds", "EVENTBUS_WAIT_TIME_SECONDS",
		"eventbus.max_messages", "EVENTBUS_MAX_MESSAGES",
		"eventbus.visibility_timeout", "EVENTBUS_VISIBILITY_TIMEOUT",
	)
}

// Validate checks that event bus configuration is coherent for the selected backend.
func (e Extension) Validate() error {
	if e.EventBus.Type == "" {
		return nil
	}
	validTypes := []string{EventBusTypeKafka, EventBusTypeRabbitMQ, EventBusTypeSQS}
	if !contains(validTypes, e.EventBus.Type) {
		return validationErrorf("validation.eventbus.type.invalid", "invalid eventbus.type: %s (must be one of: %v)", e.EventBus.Type, validTypes)
	}
	switch e.EventBus.Type {
	case EventBusTypeKafka:
		if len(e.EventBus.Brokers) == 0 {
			return validationError("validation.eventbus.brokers.required", "eventbus.brokers is required when eventbus.type is specified")
		}
	case EventBusTypeRabbitMQ:
		if strings.TrimSpace(e.EventBus.URL) == "" && len(e.EventBus.Brokers) == 0 {
			return validationError("validation.eventbus.url.required", "eventbus.url (or eventbus.brokers[0]) is required when eventbus.type is rabbitmq")
		}
	case EventBusTypeSQS:
		if strings.TrimSpace(e.EventBus.Region) == "" {
			return validationError("validation.eventbus.region.required", "eventbus.region is required when eventbus.type is sqs")
		}
		if strings.TrimSpace(e.EventBus.QueueURL) == "" {
			return validationError("validation.eventbus.queue_url.required", "eventbus.queue_url is required when eventbus.type is sqs")
		}
	}
	validSerializers := []string{"json", "protobuf", "avro"}
	if !contains(validSerializers, e.EventBus.Serializer) {
		return validationErrorf("validation.eventbus.serializer.invalid", "invalid eventbus.serializer: %s (must be one of: %v)", e.EventBus.Serializer, validSerializers)
	}
	return nil
}

func validationError(code, message string) error {
	return coreerrors.NewValidationWithCode(code, message, nil, nil)
}

func validationErrorf(code, format string, args ...any) error {
	return validationError(code, fmt.Sprintf(format, args...))
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	if len(values)%2 != 0 {
		return fmt.Errorf("bindEnvPairs requires even number of values, got %d", len(values))
	}
	for len(values) > 0 {
		key, suffix := values[0], values[1]
		if err := v.BindEnv(key, prefixedEnv(prefix, suffix)); err != nil {
			return err
		}
		values = values[2:]
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.TrimSpace(prefix) + "_" + suffix
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
