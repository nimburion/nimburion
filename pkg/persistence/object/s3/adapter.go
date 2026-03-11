package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Config defines S3 adapter configuration.
type Config struct {
	Bucket           string
	Region           string
	Endpoint         string
	AccessKeyID      string
	SecretAccessKey  string
	SessionToken     string
	UsePathStyle     bool
	OperationTimeout time.Duration
	PresignExpiry    time.Duration
}

// ObjectInfo represents a minimal S3 object descriptor for list responses.
type ObjectInfo struct {
	Key          string
	ETag         string
	Size         int64
	LastModified time.Time
}

type s3API interface {
	HeadBucket(ctx context.Context, params *awss3.HeadBucketInput, optFns ...func(*awss3.Options)) (*awss3.HeadBucketOutput, error)
	PutObject(ctx context.Context, params *awss3.PutObjectInput, optFns ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *awss3.DeleteObjectInput, optFns ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *awss3.ListObjectsV2Input, optFns ...func(*awss3.Options)) (*awss3.ListObjectsV2Output, error)
}

type presignAPI interface {
	PresignGetObject(ctx context.Context, params *awss3.GetObjectInput, optFns ...func(*awss3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// Adapter provides object storage operations backed by AWS S3 API.
type Adapter struct {
	client  s3API
	presign presignAPI
	logger  logger.Logger
	config  Config

	mu     sync.RWMutex
	closed bool
}

// NewAdapter creates a new S3 adapter and verifies bucket accessibility.
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.object.s3.bucket.required", "s3 bucket is required", nil, nil)
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, coreerrors.NewValidationWithCode("validation.object.s3.region.required", "aws region is required", nil, nil)
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}
	if cfg.PresignExpiry <= 0 {
		cfg.PresignExpiry = 15 * time.Minute
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
			WithDetails(map[string]interface{}{"family": "object_s3"})
	}

	clientOptions := make([]func(*awss3.Options), 0, 2)
	if cfg.Endpoint != "" {
		clientOptions = append(clientOptions, func(o *awss3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}
	if cfg.UsePathStyle {
		clientOptions = append(clientOptions, func(o *awss3.Options) {
			o.UsePathStyle = true
		})
	}

	client := awss3.NewFromConfig(awsCfg, clientOptions...)
	adapter := &Adapter{
		client:  client,
		presign: awss3.NewPresignClient(client),
		logger:  log,
		config:  cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
	if err := adapter.Ping(ctx); err != nil {
		return nil, err
	}

	log.Info("S3 adapter initialized", "bucket", cfg.Bucket, "region", cfg.Region, "endpoint", cfg.Endpoint)
	return adapter, nil
}

// Ping verifies that the configured bucket is accessible.
func (a *Adapter) Ping(ctx context.Context) error {
	if err := a.ensureOpen(); err != nil {
		return err
	}
	_, err := a.client.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: aws.String(a.config.Bucket),
	})
	if err != nil {
		return coreerrors.NewRetryable("s3 ping failed", err).
			WithDetails(map[string]interface{}{"family": "object_s3"})
	}
	return nil
}

// Upload stores an object and returns its ETag (without quotes when present).
func (a *Adapter) Upload(ctx context.Context, key string, body io.Reader, contentType string, metadata map[string]string) (string, error) {
	if err := a.ensureOpen(); err != nil {
		return "", err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", coreerrors.NewInvalidArgument("object key is required", nil)
	}
	if body == nil {
		return "", coreerrors.NewInvalidArgument("object body is required", nil)
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()

	input := &awss3.PutObjectInput{
		Bucket: aws.String(a.config.Bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if strings.TrimSpace(contentType) != "" {
		input.ContentType = aws.String(contentType)
	}
	if len(metadata) > 0 {
		input.Metadata = metadata
	}

	resp, err := a.client.PutObject(opCtx, input)
	if err != nil {
		return "", coreerrors.NewRetryable(fmt.Sprintf("failed to upload object %q", key), err).
			WithDetails(map[string]interface{}{"family": "object_s3", "key": key})
	}
	return strings.Trim(strings.TrimSpace(aws.ToString(resp.ETag)), "\""), nil
}

// UploadBytes stores an object from an in-memory byte slice.
func (a *Adapter) UploadBytes(ctx context.Context, key string, payload []byte, contentType string, metadata map[string]string) (string, error) {
	return a.Upload(ctx, key, bytes.NewReader(payload), contentType, metadata)
}

// Download fetches an object payload and returns bytes + content type.
func (a *Adapter) Download(ctx context.Context, key string) ([]byte, string, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, "", err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, "", coreerrors.NewInvalidArgument("object key is required", nil)
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()

	resp, err := a.client.GetObject(opCtx, &awss3.GetObjectInput{
		Bucket: aws.String(a.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", coreerrors.NewRetryable(fmt.Sprintf("failed to download object %q", key), err).
			WithDetails(map[string]interface{}{"family": "object_s3", "key": key})
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			ignoreCloseError(closeErr)
		}
	}()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", coreerrors.NewRetryable(fmt.Sprintf("failed to read object %q", key), err).
			WithDetails(map[string]interface{}{"family": "object_s3", "key": key})
	}

	return payload, aws.ToString(resp.ContentType), nil
}

func ignoreCloseError(_ error) {}

// Delete removes an object by key.
func (a *Adapter) Delete(ctx context.Context, key string) error {
	if err := a.ensureOpen(); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return coreerrors.NewInvalidArgument("object key is required", nil)
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()

	_, err := a.client.DeleteObject(opCtx, &awss3.DeleteObjectInput{
		Bucket: aws.String(a.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return coreerrors.NewRetryable(fmt.Sprintf("failed to delete object %q", key), err).
			WithDetails(map[string]interface{}{"family": "object_s3", "key": key})
	}
	return nil
}

// List returns object metadata for a prefix.
func (a *Adapter) List(ctx context.Context, prefix string, maxKeys int32) ([]ObjectInfo, error) {
	if err := a.ensureOpen(); err != nil {
		return nil, err
	}
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()

	resp, err := a.client.ListObjectsV2(opCtx, &awss3.ListObjectsV2Input{
		Bucket:  aws.String(a.config.Bucket),
		Prefix:  aws.String(strings.TrimSpace(prefix)),
		MaxKeys: aws.Int32(maxKeys),
	})
	if err != nil {
		return nil, coreerrors.NewRetryable(fmt.Sprintf("failed to list objects with prefix %q", prefix), err).
			WithDetails(map[string]interface{}{"family": "object_s3", "prefix": prefix})
	}

	out := make([]ObjectInfo, 0, len(resp.Contents))
	for _, item := range resp.Contents {
		out = append(out, toObjectInfo(item))
	}
	return out, nil
}

// PresignGetURL generates a temporary download URL.
func (a *Adapter) PresignGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if err := a.ensureOpen(); err != nil {
		return "", err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", coreerrors.NewInvalidArgument("object key is required", nil)
	}
	if expiry <= 0 {
		expiry = a.config.PresignExpiry
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()

	resp, err := a.presign.PresignGetObject(opCtx, &awss3.GetObjectInput{
		Bucket: aws.String(a.config.Bucket),
		Key:    aws.String(key),
	}, func(opts *awss3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", coreerrors.NewRetryable(fmt.Sprintf("failed to presign object %q", key), err).
			WithDetails(map[string]interface{}{"family": "object_s3", "key": key})
	}
	return resp.URL, nil
}

// HealthCheck verifies the adapter can reach the bucket within a short timeout.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	hcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.Ping(hcCtx); err != nil {
		a.logger.Error("S3 health check failed", "error", err)
		return coreerrors.NewUnavailable("s3 health check failed", err).
			WithDetails(map[string]interface{}{"family": "object_s3"})
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

func (a *Adapter) withOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.config.OperationTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, a.config.OperationTimeout)
}

func (a *Adapter) ensureOpen() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return coreerrors.NewClosed("s3 adapter is closed", nil).
			WithDetails(map[string]interface{}{"family": "object_s3"})
	}
	return nil
}

// Optional helper for callers that need AWS object attributes.
func toObjectInfo(item awss3types.Object) ObjectInfo {
	return ObjectInfo{
		Key:          aws.ToString(item.Key),
		ETag:         strings.Trim(strings.TrimSpace(aws.ToString(item.ETag)), "\""),
		Size:         aws.ToInt64(item.Size),
		LastModified: aws.ToTime(item.LastModified),
	}
}
