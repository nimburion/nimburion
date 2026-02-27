package sqs

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Adapter implements eventbus.EventBus for AWS SQS.
type Adapter struct {
	client *sqs.Client
	logger logger.Logger
	config Config
	mu     sync.RWMutex
	subs   map[string]context.CancelFunc
	closed bool
}

// Config holds SQS adapter configuration.
type Config struct {
	Region            string
	QueueURL          string
	Endpoint          string
	AccessKeyID       string
	SecretAccessKey   string
	SessionToken      string
	OperationTimeout  time.Duration
	WaitTimeSeconds   int32
	MaxMessages       int32
	VisibilityTimeout int32
}

// Cosa fa: crea adapter SQS con supporto endpoint custom e long polling.
// Cosa NON fa: non crea code o policy IAM.
// Esempio minimo: adapter, err := sqs.NewAdapter(cfg, log)
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("aws region is required")
	}
	if cfg.QueueURL == "" {
		return nil, fmt.Errorf("sqs queue URL is required")
	}
	if cfg.OperationTimeout == 0 {
		cfg.OperationTimeout = 30 * time.Second
	}
	if cfg.WaitTimeSeconds == 0 {
		cfg.WaitTimeSeconds = 10
	}
	if cfg.MaxMessages == 0 {
		cfg.MaxMessages = 10
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	var opts []func(*sqs.Options)
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := sqs.NewFromConfig(awsCfg, opts...)
	adapter := &Adapter{
		client: client,
		logger: log,
		config: cfg,
		subs:   make(map[string]context.CancelFunc),
	}
	if err := adapter.HealthCheck(context.Background()); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (a *Adapter) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("sqs adapter is closed")
	}
	a.mu.RUnlock()
	if message == nil {
		return fmt.Errorf("message is required")
	}

	queueURL := a.resolveQueueURL(topic)
	opCtx, cancel := context.WithTimeout(ctx, a.config.OperationTimeout)
	defer cancel()

	_, err := a.client.SendMessage(opCtx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(queueURL),
		MessageBody:       aws.String(string(message.Value)),
		MessageAttributes: toSQSAttributes(message.Headers),
	})
	if err != nil {
		return fmt.Errorf("failed to publish sqs message: %w", err)
	}
	return nil
}

func (a *Adapter) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("sqs adapter is closed")
	}
	a.mu.RUnlock()

	if len(messages) == 0 {
		return nil
	}

	queueURL := a.resolveQueueURL(topic)
	for i := 0; i < len(messages); i += 10 {
		end := i + 10
		if end > len(messages) {
			end = len(messages)
		}
		batch := messages[i:end]
		entries := make([]types.SendMessageBatchRequestEntry, 0, len(batch))
		for idx, m := range batch {
			entries = append(entries, types.SendMessageBatchRequestEntry{
				Id:                aws.String(strconv.Itoa(i + idx)),
				MessageBody:       aws.String(string(m.Value)),
				MessageAttributes: toSQSAttributes(m.Headers),
			})
		}

		opCtx, cancel := context.WithTimeout(ctx, a.config.OperationTimeout)
		_, err := a.client.SendMessageBatch(opCtx, &sqs.SendMessageBatchInput{QueueUrl: aws.String(queueURL), Entries: entries})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to publish sqs batch: %w", err)
		}
	}
	return nil
}

func (a *Adapter) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return fmt.Errorf("sqs adapter is closed")
	}
	if _, ok := a.subs[topic]; ok {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	subCtx, cancel := context.WithCancel(ctx)
	a.subs[topic] = cancel
	queueURL := a.resolveQueueURL(topic)
	go a.pollLoop(subCtx, queueURL, handler)
	return nil
}

func (a *Adapter) pollLoop(ctx context.Context, queueURL string, handler eventbus.MessageHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			recvCtx, cancel := context.WithTimeout(ctx, a.config.OperationTimeout)
			out, err := a.client.ReceiveMessage(recvCtx, &sqs.ReceiveMessageInput{
				QueueUrl:              aws.String(queueURL),
				MaxNumberOfMessages:   a.config.MaxMessages,
				WaitTimeSeconds:       a.config.WaitTimeSeconds,
				VisibilityTimeout:     a.config.VisibilityTimeout,
				MessageAttributeNames: []string{"All"},
			})
			cancel()
			if err != nil {
				a.logger.Error("sqs receive failed", "error", err)
				time.Sleep(200 * time.Millisecond)
				continue
			}

			for _, m := range out.Messages {
				msg := &eventbus.Message{
					ID:      aws.ToString(m.MessageId),
					Value:   []byte(aws.ToString(m.Body)),
					Headers: fromSQSAttributes(m.MessageAttributes),
				}
				if err := handler(ctx, msg); err != nil {
					continue
				}
				if m.ReceiptHandle != nil {
					_, _ = a.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{QueueUrl: aws.String(queueURL), ReceiptHandle: m.ReceiptHandle})
				}
			}
		}
	}
}

func (a *Adapter) Unsubscribe(topic string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	cancel, ok := a.subs[topic]
	if !ok {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}
	cancel()
	delete(a.subs, topic)
	return nil
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return fmt.Errorf("sqs adapter is closed")
	}
	a.mu.RUnlock()

	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := a.client.GetQueueAttributes(hcCtx, &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(a.config.QueueURL),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameQueueArn,
		},
	})
	if err != nil {
		return fmt.Errorf("sqs health check failed: %w", err)
	}
	return nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	for _, cancel := range a.subs {
		cancel()
	}
	a.subs = map[string]context.CancelFunc{}
	return nil
}

func (a *Adapter) resolveQueueURL(topic string) string {
	if topic != "" {
		return topic
	}
	return a.config.QueueURL
}

func toSQSAttributes(headers map[string]string) map[string]types.MessageAttributeValue {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]types.MessageAttributeValue, len(headers))
	for k, v := range headers {
		out[k] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(v)}
	}
	return out
}

func fromSQSAttributes(headers map[string]types.MessageAttributeValue) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = aws.ToString(v.StringValue)
	}
	return out
}
