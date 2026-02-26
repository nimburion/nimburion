package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQAdapter implements eventbus.EventBus for RabbitMQ.
type RabbitMQAdapter struct {
	conn   *amqp.Connection
	pubCh  *amqp.Channel
	logger logger.Logger
	config Config
	subs   map[string]*subscription
	mu     sync.RWMutex
	closed bool
}

type subscription struct {
	channel *amqp.Channel
	queue   string
	cancel  context.CancelFunc
}

// Config holds RabbitMQ adapter configuration.
type Config struct {
	URL              string
	Exchange         string
	ExchangeType     string
	QueueName        string
	RoutingKey       string
	OperationTimeout time.Duration
	ConsumerTag      string
}

// Cosa fa: crea connessione/channel RabbitMQ e prepara publish/subscribe.
// Cosa NON fa: non crea policy broker o dead-letter exchange.
// Esempio minimo: adapter, err := rabbitmq.NewRabbitMQAdapter(cfg, log)
func NewRabbitMQAdapter(cfg Config, log logger.Logger) (*RabbitMQAdapter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("rabbitmq URL is required")
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "events"
	}
	if cfg.ExchangeType == "" {
		cfg.ExchangeType = "topic"
	}
	if cfg.OperationTimeout == 0 {
		cfg.OperationTimeout = 30 * time.Second
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to create rabbitmq channel: %w", err)
	}

	if err := pubCh.ExchangeDeclare(cfg.Exchange, cfg.ExchangeType, true, false, false, false, nil); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	a := &RabbitMQAdapter{
		conn:   conn,
		pubCh:  pubCh,
		logger: log,
		config: cfg,
		subs:   make(map[string]*subscription),
	}

	if err := a.HealthCheck(context.Background()); err != nil {
		_ = a.Close()
		return nil, err
	}

	return a, nil
}

// Publish sends a message to the specified topic.
func (a *RabbitMQAdapter) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("rabbitmq adapter is closed")
	}
	a.mu.RUnlock()

	if message == nil {
		return fmt.Errorf("message is required")
	}

	routingKey := topic
	if routingKey == "" {
		routingKey = a.config.RoutingKey
	}

	ctx, cancel := context.WithTimeout(ctx, a.config.OperationTimeout)
	defer cancel()

	publishing := amqp.Publishing{
		MessageId:   message.ID,
		ContentType: message.ContentType,
		Body:        message.Value,
		Timestamp:   message.Timestamp,
		Headers:     toAMQPHeaders(message.Headers),
	}

	if err := a.pubCh.PublishWithContext(ctx, a.config.Exchange, routingKey, false, false, publishing); err != nil {
		return fmt.Errorf("failed to publish rabbitmq message: %w", err)
	}
	return nil
}

// PublishBatch TODO: add description
func (a *RabbitMQAdapter) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	for _, msg := range messages {
		if err := a.Publish(ctx, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe creates a queue, binds it to the topic, and processes messages in a background goroutine.
// The handler is invoked for each message. Messages are auto-acked on success or nacked on error.
func (a *RabbitMQAdapter) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("rabbitmq adapter is closed")
	}
	if _, exists := a.subs[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	ch, err := a.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create consumer channel: %w", err)
	}

	if err := ch.ExchangeDeclare(a.config.Exchange, a.config.ExchangeType, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	qName := a.config.QueueName
	if qName == "" {
		qName = ""
	}
	q, err := ch.QueueDeclare(qName, true, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	bindKey := topic
	if bindKey == "" {
		bindKey = a.config.RoutingKey
	}
	if err := ch.QueueBind(q.Name, bindKey, a.config.Exchange, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	deliveries, err := ch.Consume(q.Name, a.config.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	a.subs[topic] = &subscription{channel: ch, queue: q.Name, cancel: cancel}
	go a.consumeLoop(subCtx, topic, deliveries, handler)

	return nil
}

func (a *RabbitMQAdapter) consumeLoop(ctx context.Context, topic string, deliveries <-chan amqp.Delivery, handler eventbus.MessageHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			msg := &eventbus.Message{
				ID:          d.MessageId,
				Key:         topic,
				Value:       d.Body,
				Headers:     fromAMQPHeaders(d.Headers),
				ContentType: d.ContentType,
				Timestamp:   d.Timestamp,
			}

			if err := handler(ctx, msg); err != nil {
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

// Unsubscribe removes the subscription for the specified topic.
func (a *RabbitMQAdapter) Unsubscribe(topic string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	sub, ok := a.subs[topic]
	if !ok {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}
	sub.cancel()
	if err := sub.channel.Close(); err != nil {
		return fmt.Errorf("failed to close subscription channel: %w", err)
	}
	delete(a.subs, topic)
	return nil
}

// HealthCheck verifies the component is operational and can perform its intended function.
func (a *RabbitMQAdapter) HealthCheck(ctx context.Context) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("rabbitmq adapter is closed")
	}
	conn := a.conn
	a.mu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection is closed")
	}

	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq health check failed: %w", err)
	}
	_ = ch.Close()
	select {
	case <-hcCtx.Done():
		return fmt.Errorf("rabbitmq health check timeout: %w", hcCtx.Err())
	default:
		return nil
	}
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (a *RabbitMQAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	var errs []error
	for topic, sub := range a.subs {
		sub.cancel()
		if err := sub.channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close subscription %s: %w", topic, err))
		}
	}
	a.subs = map[string]*subscription{}

	if a.pubCh != nil {
		if err := a.pubCh.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close publish channel: %w", err))
		}
	}
	if a.conn != nil {
		if err := a.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close connection: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rabbitmq close errors: %v", errs)
	}
	return nil
}

func toAMQPHeaders(headers map[string]string) amqp.Table {
	if len(headers) == 0 {
		return nil
	}
	t := amqp.Table{}
	for k, v := range headers {
		t[k] = v
	}
	return t
}

func fromAMQPHeaders(headers amqp.Table) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}
