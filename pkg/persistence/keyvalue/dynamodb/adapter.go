package dynamodb

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
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

// NewAdapter creates a DynamoDB storage adapter.
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	if cfg.Region == "" {
		return nil, coreerrors.NewValidationWithCode("validation.keyvalue.dynamodb.region.required", "aws region is required", nil, nil)
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
		return nil, coreerrors.NewRetryable("failed to load aws config", err).
			WithDetails(map[string]interface{}{"family": "keyvalue_dynamodb"})
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

// Client returns the underlying DynamoDB client.
func (a *Adapter) Client() *dynamodb.Client {
	return a.client
}

// Ping checks basic connectivity to DynamoDB.
func (a *Adapter) Ping(ctx context.Context) error {
	if err := a.ensureOpen(); err != nil {
		return err
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	_, err := a.client.ListTables(opCtx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	if err != nil {
		return coreerrors.NewRetryable("dynamodb ping failed", err).
			WithDetails(map[string]interface{}{"family": "keyvalue_dynamodb"})
	}
	return nil
}

// HealthCheck verifies the adapter is operational.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.Ping(hcCtx); err != nil {
		a.logger.Error("DynamoDB health check failed", "error", err)
		return coreerrors.NewUnavailable("dynamodb health check failed", err).
			WithDetails(map[string]interface{}{"family": "keyvalue_dynamodb"})
	}
	return nil
}

// Close marks the adapter as closed.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return nil
}

// PutItem proxies a PutItem request to DynamoDB.
func (a *Adapter) PutItem(ctx context.Context, input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, err
	}
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.PutItem(opCtx, input)
}

// GetItem proxies a GetItem request to DynamoDB.
func (a *Adapter) GetItem(ctx context.Context, input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, err
	}
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.GetItem(opCtx, input)
}

// UpdateItem proxies an UpdateItem request to DynamoDB.
func (a *Adapter) UpdateItem(ctx context.Context, input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, err
	}
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.UpdateItem(opCtx, input)
}

// DeleteItem proxies a DeleteItem request to DynamoDB.
func (a *Adapter) DeleteItem(ctx context.Context, input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, err
	}
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.DeleteItem(opCtx, input)
}

// Query proxies a Query request to DynamoDB.
func (a *Adapter) Query(ctx context.Context, input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, err
	}
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	return a.client.Query(opCtx, input)
}

func (a *Adapter) ensureOpen() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return coreerrors.NewClosed("dynamodb adapter is closed", nil).
			WithDetails(map[string]interface{}{"family": "keyvalue_dynamodb"})
	}
	return nil
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

// IsThrottlingError reports whether err is a DynamoDB throttling error.
func IsThrottlingError(err error) bool {
	if err == nil {
		return false
	}
	var pte *types.ProvisionedThroughputExceededException
	return errors.As(err, &pte)
}
