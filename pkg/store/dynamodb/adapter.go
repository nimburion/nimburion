package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Adapter provides DynamoDB connectivity.
type Adapter struct {
	client  *dynamodb.Client
	logger  logger.Logger
	timeout time.Duration
	mu      sync.RWMutex
	closed  bool
}

// Config holds DynamoDB adapter configuration.
type Config struct {
	Region           string
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	SessionToken     string
	OperationTimeout time.Duration
}

// Cosa fa: costruisce client DynamoDB (AWS SDK v2) con supporto endpoint custom.
// Cosa NON fa: non crea tabelle o throughput policy.
// Esempio minimo: adapter, err := dynamodb.NewAdapter(cfg, log)
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("aws region is required")
	}
	if cfg.OperationTimeout == 0 {
		cfg.OperationTimeout = 5 * time.Second
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

	var opts []func(*dynamodb.Options)
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := dynamodb.NewFromConfig(awsCfg, opts...)
	adapter := &Adapter{client: client, logger: log, timeout: cfg.OperationTimeout}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
	if err := adapter.Ping(ctx); err != nil {
		return nil, err
	}

	log.Info("DynamoDB adapter initialized", "region", cfg.Region, "endpoint", cfg.Endpoint)
	return adapter, nil
}

func (a *Adapter) Client() *dynamodb.Client {
	return a.client
}

func (a *Adapter) Ping(ctx context.Context) error {
	a.mu.RLock()
	closed := a.closed
	a.mu.RUnlock()
	if closed {
		return fmt.Errorf("dynamodb adapter is closed")
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	_, err := a.client.ListTables(opCtx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	if err != nil {
		return fmt.Errorf("dynamodb ping failed: %w", err)
	}
	return nil
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.Ping(hcCtx); err != nil {
		a.logger.Error("DynamoDB health check failed", "error", err)
		return fmt.Errorf("dynamodb health check failed: %w", err)
	}
	return nil
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return nil
}

func (a *Adapter) PutItem(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.PutItem(opCtx, input)
}

func (a *Adapter) GetItem(ctx context.Context, input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.GetItem(opCtx, input)
}

func (a *Adapter) UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.UpdateItem(opCtx, input)
}

func (a *Adapter) DeleteItem(ctx context.Context, input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.DeleteItem(opCtx, input)
}

func (a *Adapter) Query(ctx context.Context, input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.Query(opCtx, input)
}

func (a *Adapter) withOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, a.timeout)
}

func IsThrottlingError(err error) bool {
	if err == nil {
		return false
	}
	var pte *types.ProvisionedThroughputExceededException
	return errors.As(err, &pte)
}
