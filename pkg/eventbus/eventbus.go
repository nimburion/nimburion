package eventbus

import (
	"context"
	"time"
)

// Producer defines the interface for publishing messages to topics.
// Implementations must support both single and batch publishing operations.
type Producer interface {
	// Publish sends a single message to the specified topic.
	// Returns an error if the publish operation fails.
	Publish(ctx context.Context, topic string, message *Message) error

	// PublishBatch sends multiple messages to the specified topic in a single operation.
	// Batch operations are more efficient for high-throughput scenarios.
	// Returns an error if any message in the batch fails to publish.
	PublishBatch(ctx context.Context, topic string, messages []*Message) error

	// Close gracefully shuts down the producer, flushing any pending messages.
	// Returns an error if the shutdown fails.
	Close() error
}

// Consumer defines the interface for subscribing to topics and consuming messages.
// Implementations must support multiple concurrent subscriptions.
type Consumer interface {
	// Subscribe registers a message handler for the specified topic.
	// The handler will be invoked for each message received on the topic.
	// Returns an error if the subscription fails.
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error

	// Unsubscribe removes the subscription for the specified topic.
	// Returns an error if the unsubscribe operation fails.
	Unsubscribe(topic string) error

	// Close gracefully shuts down the consumer, acknowledging any pending messages.
	// Returns an error if the shutdown fails.
	Close() error
}

// EventBus combines Producer and Consumer interfaces with health checking.
// This is the primary interface for event bus adapters (Kafka, RabbitMQ, SQS).
type EventBus interface {
	Producer
	Consumer

	// HealthCheck verifies connectivity to the message broker.
	// Returns an error if the broker is unreachable or unhealthy.
	HealthCheck(ctx context.Context) error
}

// Message represents a message to be published or consumed from a topic.
// It includes metadata such as headers, content type, and timestamp.
type Message struct {
	// ID is a unique identifier for the message.
	ID string

	// Key is used for partitioning in systems like Kafka.
	// Messages with the same key are guaranteed to be delivered to the same partition.
	Key string

	// Value is the serialized message payload.
	Value []byte

	// Headers contains arbitrary key-value metadata for the message.
	Headers map[string]string

	// ContentType indicates the serialization format (e.g., "application/json", "application/protobuf").
	ContentType string

	// Timestamp is when the message was created.
	Timestamp time.Time
}

// MessageHandler is a function type for processing consumed messages.
// Implementations should return an error if message processing fails,
// which may trigger retry logic or dead letter queue handling.
type MessageHandler func(ctx context.Context, msg *Message) error
