package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/segmentio/kafka-go"
)

// KafkaAdapter implements the eventbus.EventBus interface for Apache Kafka.
// It manages a single producer for publishing messages and multiple consumers
// for subscribing to topics.
type KafkaAdapter struct {
	producer  *kafka.Writer
	consumers map[string]*kafka.Reader
	logger    logger.Logger
	config    Config
	mu        sync.RWMutex
	closed    bool
}

// Config holds the configuration for the Kafka adapter.
type Config struct {
	// Brokers is the list of Kafka broker addresses (e.g., ["localhost:9092"])
	Brokers []string

	// OperationTimeout is the timeout for publish and subscribe operations
	OperationTimeout time.Duration

	// MaxRetries is the maximum number of retry attempts for failed operations
	MaxRetries int

	// RetryBackoff is the initial backoff duration for retries (exponential backoff)
	RetryBackoff time.Duration

	// GroupID is the consumer group ID for subscriptions
	GroupID string
}

// NewKafkaAdapter creates a new Kafka adapter with the specified configuration.
// It initializes a producer for publishing messages and prepares for consumer subscriptions.
//
// Parameters:
//   - cfg: Configuration for the Kafka adapter
//   - logger: Logger instance for structured logging
//
// Returns:
//   - *KafkaAdapter: The initialized Kafka adapter
//   - error: An error if initialization fails
func NewKafkaAdapter(cfg Config, logger logger.Logger) (*KafkaAdapter, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker address is required")
	}

	if cfg.OperationTimeout == 0 {
		cfg.OperationTimeout = 30 * time.Second
	}

	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 100 * time.Millisecond
	}

	if cfg.GroupID == "" {
		cfg.GroupID = "default-consumer-group"
	}

	// Create Kafka writer (producer)
	producer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		MaxAttempts:  cfg.MaxRetries,
		WriteTimeout: cfg.OperationTimeout,
		ReadTimeout:  cfg.OperationTimeout,
		// Enable async writes for better performance
		Async: false,
	}

	logger.Info("kafka adapter initialized",
		"brokers", cfg.Brokers,
		"group_id", cfg.GroupID,
		"operation_timeout", cfg.OperationTimeout,
	)

	return &KafkaAdapter{
		producer:  producer,
		consumers: make(map[string]*kafka.Reader),
		logger:    logger,
		config:    cfg,
		closed:    false,
	}, nil
}

// Publish sends a single message to the specified topic.
// It converts the eventbus.Message to a Kafka message and publishes it.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - topic: The topic to publish to
//   - message: The message to publish
//
// Returns:
//   - error: An error if the publish operation fails
func (a *KafkaAdapter) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("kafka adapter is closed")
	}
	a.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, a.config.OperationTimeout)
	defer cancel()

	kafkaMsg := kafka.Message{
		Topic:   topic,
		Key:     []byte(message.Key),
		Value:   message.Value,
		Headers: convertHeaders(message.Headers),
		Time:    message.Timestamp,
	}

	err := a.producer.WriteMessages(ctx, kafkaMsg)
	if err != nil {
		a.logger.Error("failed to publish message",
			"topic", topic,
			"message_id", message.ID,
			"error", err,
		)
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	a.logger.Debug("message published",
		"topic", topic,
		"message_id", message.ID,
		"key", message.Key,
	)

	return nil
}

// PublishBatch sends multiple messages to the specified topic in a single operation.
// Batch operations are more efficient for high-throughput scenarios.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - topic: The topic to publish to
//   - messages: The messages to publish
//
// Returns:
//   - error: An error if any message in the batch fails to publish
func (a *KafkaAdapter) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("kafka adapter is closed")
	}
	a.mu.RUnlock()

	if len(messages) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, a.config.OperationTimeout)
	defer cancel()

	kafkaMessages := make([]kafka.Message, len(messages))
	for i, msg := range messages {
		kafkaMessages[i] = kafka.Message{
			Topic:   topic,
			Key:     []byte(msg.Key),
			Value:   msg.Value,
			Headers: convertHeaders(msg.Headers),
			Time:    msg.Timestamp,
		}
	}

	err := a.producer.WriteMessages(ctx, kafkaMessages...)
	if err != nil {
		a.logger.Error("failed to publish batch",
			"topic", topic,
			"batch_size", len(messages),
			"error", err,
		)
		return fmt.Errorf("failed to publish batch to topic %s: %w", topic, err)
	}

	a.logger.Debug("batch published",
		"topic", topic,
		"batch_size", len(messages),
	)

	return nil
}

// Subscribe registers a message handler for the specified topic.
// It creates a new Kafka consumer for the topic and starts consuming messages
// in a background goroutine.
//
// Parameters:
//   - ctx: Context for cancellation
//   - topic: The topic to subscribe to
//   - handler: The message handler function
//
// Returns:
//   - error: An error if the subscription fails
func (a *KafkaAdapter) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("kafka adapter is closed")
	}

	if _, exists := a.consumers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	// Create Kafka reader (consumer)
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        a.config.Brokers,
		Topic:          topic,
		GroupID:        a.config.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
		MaxWait:        500 * time.Millisecond,
	})

	a.consumers[topic] = reader

	// Start consuming messages in a background goroutine
	go a.consumeMessages(ctx, topic, reader, handler)

	a.logger.Info("subscribed to topic",
		"topic", topic,
		"group_id", a.config.GroupID,
	)

	return nil
}

// Unsubscribe removes the subscription for the specified topic.
// It closes the consumer and removes it from the consumer map.
//
// Parameters:
//   - topic: The topic to unsubscribe from
//
// Returns:
//   - error: An error if the unsubscribe operation fails
func (a *KafkaAdapter) Unsubscribe(topic string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	reader, exists := a.consumers[topic]
	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	if err := reader.Close(); err != nil {
		a.logger.Error("failed to close consumer",
			"topic", topic,
			"error", err,
		)
		return fmt.Errorf("failed to close consumer for topic %s: %w", topic, err)
	}

	delete(a.consumers, topic)

	a.logger.Info("unsubscribed from topic", "topic", topic)

	return nil
}

// Close gracefully shuts down the Kafka adapter.
// It closes the producer and all active consumers.
//
// Returns:
//   - error: An error if the shutdown fails
func (a *KafkaAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}

	a.closed = true

	var errs []error

	// Close producer
	if err := a.producer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close producer: %w", err))
	}

	// Close all consumers
	for topic, reader := range a.consumers {
		if err := reader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close consumer for topic %s: %w", topic, err))
		}
	}

	a.consumers = make(map[string]*kafka.Reader)

	a.logger.Info("kafka adapter closed")

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// HealthCheck verifies connectivity to the Kafka brokers.
// It attempts to list topics to verify the connection is working.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//
// Returns:
//   - error: An error if the health check fails
func (a *KafkaAdapter) HealthCheck(ctx context.Context) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("kafka adapter is closed")
	}
	a.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Create a temporary connection to check broker health
	conn, err := kafka.DialContext(ctx, "tcp", a.config.Brokers[0])
	if err != nil {
		return fmt.Errorf("failed to connect to kafka broker: %w", err)
	}
	defer conn.Close()

	// Try to fetch broker metadata
	_, err = conn.Brokers()
	if err != nil {
		return fmt.Errorf("failed to fetch broker metadata: %w", err)
	}

	return nil
}

// consumeMessages is a background goroutine that consumes messages from a topic
// and invokes the message handler for each message.
func (a *KafkaAdapter) consumeMessages(ctx context.Context, topic string, reader *kafka.Reader, handler eventbus.MessageHandler) {
	a.logger.Info("started consuming messages", "topic", topic)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("stopping message consumption", "topic", topic)
			return
		default:
			// Fetch message with timeout
			msg, err := reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				a.logger.Error("failed to fetch message",
					"topic", topic,
					"error", err,
				)
				continue
			}

			// Convert Kafka message to eventbus.Message
			eventMsg := &eventbus.Message{
				Key:       string(msg.Key),
				Value:     msg.Value,
				Headers:   convertKafkaHeaders(msg.Headers),
				Timestamp: msg.Time,
			}

			// Invoke message handler
			if err := handler(ctx, eventMsg); err != nil {
				a.logger.Error("message handler failed",
					"topic", topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"error", err,
				)
				// Don't commit on error - message will be redelivered
				continue
			}

			// Commit message offset
			if err := reader.CommitMessages(ctx, msg); err != nil {
				a.logger.Error("failed to commit message",
					"topic", topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"error", err,
				)
			}
		}
	}
}

// convertHeaders converts eventbus headers to Kafka headers
func convertHeaders(headers map[string]string) []kafka.Header {
	if headers == nil {
		return nil
	}

	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for key, value := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{
			Key:   key,
			Value: []byte(value),
		})
	}

	return kafkaHeaders
}

// convertKafkaHeaders converts Kafka headers to eventbus headers
func convertKafkaHeaders(headers []kafka.Header) map[string]string {
	if headers == nil {
		return nil
	}

	eventHeaders := make(map[string]string, len(headers))
	for _, header := range headers {
		eventHeaders[header.Key] = string(header.Value)
	}

	return eventHeaders
}
