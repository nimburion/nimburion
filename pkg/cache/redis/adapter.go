package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/internal/rediskit"
	"github.com/nimburion/nimburion/pkg/cache"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// Adapter provides Redis cache connectivity with connection pooling
type Adapter struct {
	client  *rediskit.Client
	metrics *Metrics
	timeout time.Duration
}

// Config holds Redis connection configuration
type Config struct {
	URL              string
	MaxConns         int
	MetricsRegistry  *frameworkmetrics.Registry
	OperationTimeout time.Duration
}

// NewAdapter creates a new Redis adapter with connection pooling
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	timeout := cfg.OperationTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
		cfg.OperationTimeout = timeout
	}
	metrics, err := NewMetrics(cfg.MetricsRegistry)
	if err != nil {
		return nil, wrapConstructorError("NewAdapter", err)
	}
	client, err := rediskit.NewClient(rediskit.Config{
		URL:              cfg.URL,
		MaxConns:         cfg.MaxConns,
		OperationTimeout: cfg.OperationTimeout,
	}, log)
	if err != nil {
		return nil, wrapConstructorError("NewAdapter", err)
	}
	return &Adapter{client: client, metrics: metrics, timeout: timeout}, nil
}

// Client returns the underlying *redis.Client for direct access when needed
func (a *Adapter) Client() *redis.Client {
	if a == nil || a.client == nil {
		return nil
	}
	return a.client.Raw()
}

// Ping verifies the Redis connection is alive
func (a *Adapter) Ping(ctx context.Context) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	err := a.client.Ping(opCtx)
	a.metrics.record("ping", err)
	return err
}

// Get retrieves a value from Redis by key
func (a *Adapter) Get(ctx context.Context, key string) (string, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	val, err := a.client.Raw().Get(opCtx, key).Result()
	if err != nil {
		mappedErr := a.mapGetError(key, err)
		a.metrics.record("get", mappedErr)
		return "", mappedErr
	}
	a.metrics.record("get", nil)
	return val, nil
}

// Set stores a key-value pair in Redis without expiration
func (a *Adapter) Set(ctx context.Context, key string, value interface{}) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	err := a.client.Raw().Set(opCtx, key, value, 0).Err()
	if err != nil {
		err = fmt.Errorf("failed to set key %s: %w", key, err)
	}
	a.metrics.record("set", err)
	return err
}

// SetWithTTL stores a key-value pair in Redis with expiration
func (a *Adapter) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	err := a.client.Raw().Set(opCtx, key, value, ttl).Err()
	if err != nil {
		err = fmt.Errorf("failed to set key %s with TTL: %w", key, err)
	}
	a.metrics.record("set_with_ttl", err)
	return err
}

// Delete removes a key from Redis
func (a *Adapter) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		a.metrics.record("delete", nil)
		return nil
	}

	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	err := a.client.Raw().Del(opCtx, keys...).Err()
	if err != nil {
		err = fmt.Errorf("failed to delete keys: %w", err)
	}
	a.metrics.record("delete", err)
	return err
}

// Incr atomically increments the value of a key by 1
func (a *Adapter) Incr(ctx context.Context, key string) (int64, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	val, err := a.client.Raw().Incr(opCtx, key).Result()
	if err != nil {
		err = fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	a.metrics.record("incr", err)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// IncrBy atomically increments the value of a key by the specified amount
func (a *Adapter) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	val, err := a.client.Raw().IncrBy(opCtx, key, value).Result()
	if err != nil {
		err = fmt.Errorf("failed to increment key %s by %d: %w", key, value, err)
	}
	a.metrics.record("incr_by", err)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Decr atomically decrements the value of a key by 1
func (a *Adapter) Decr(ctx context.Context, key string) (int64, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	val, err := a.client.Raw().Decr(opCtx, key).Result()
	if err != nil {
		err = fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	a.metrics.record("decr", err)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// DecrBy atomically decrements the value of a key by the specified amount
func (a *Adapter) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	val, err := a.client.Raw().DecrBy(opCtx, key, value).Result()
	if err != nil {
		err = fmt.Errorf("failed to decrement key %s by %d: %w", key, value, err)
	}
	a.metrics.record("decr_by", err)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// HealthCheck verifies the Redis connection is healthy with a timeout
func (a *Adapter) HealthCheck(ctx context.Context) error {
	opCtx, cancel := a.withOperationTimeout(ctx)
	defer cancel()
	err := a.client.HealthCheck(opCtx)
	a.metrics.record("healthcheck", err)
	return err
}

// Close gracefully closes the Redis connection
func (a *Adapter) Close() error {
	if a == nil || a.client == nil {
		return nil
	}
	return a.client.Close()
}

func (a *Adapter) withOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a == nil || a.timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	// #nosec G118 -- the cancel function is returned to the caller, which defers it immediately.
	return context.WithTimeout(ctx, a.timeout)
}

func (a *Adapter) mapGetError(key string, err error) error {
	if errors.Is(err, redis.Nil) {
		return cache.ErrCacheMiss
	}
	return fmt.Errorf("failed to get key %s: %w", key, err)
}
