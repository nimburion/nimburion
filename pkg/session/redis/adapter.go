package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/internal/rediskit"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Adapter provides Redis cache connectivity with connection pooling
type Adapter struct {
	client *rediskit.Client
}

// Config holds Redis connection configuration
type Config struct {
	URL              string
	MaxConns         int
	OperationTimeout time.Duration
}

// NewAdapter creates a new Redis adapter with connection pooling
func NewAdapter(cfg Config, log logger.Logger) (*Adapter, error) {
	client, err := rediskit.NewClient(rediskit.Config(cfg), log)
	if err != nil {
		return nil, err
	}
	return &Adapter{client: client}, nil
}

// Client returns the underlying *redis.Client for direct access when needed
func (a *Adapter) Client() *redis.Client {
	return a.client.Raw()
}

// Ping verifies the Redis connection is alive
func (a *Adapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

// Get retrieves a value from Redis by key
func (a *Adapter) Get(ctx context.Context, key string) (string, error) {
	val, err := a.client.Raw().Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Set stores a key-value pair in Redis without expiration
func (a *Adapter) Set(ctx context.Context, key string, value interface{}) error {
	if err := a.client.Raw().Set(ctx, key, value, 0).Err(); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

// SetWithTTL stores a key-value pair in Redis with expiration
func (a *Adapter) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := a.client.Raw().Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key %s with TTL: %w", key, err)
	}
	return nil
}

// Delete removes a key from Redis
func (a *Adapter) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := a.client.Raw().Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}
	return nil
}

// Incr atomically increments the value of a key by 1
func (a *Adapter) Incr(ctx context.Context, key string) (int64, error) {
	val, err := a.client.Raw().Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return val, nil
}

// IncrBy atomically increments the value of a key by the specified amount
func (a *Adapter) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	val, err := a.client.Raw().IncrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s by %d: %w", key, value, err)
	}
	return val, nil
}

// Decr atomically decrements the value of a key by 1
func (a *Adapter) Decr(ctx context.Context, key string) (int64, error) {
	val, err := a.client.Raw().Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	return val, nil
}

// DecrBy atomically decrements the value of a key by the specified amount
func (a *Adapter) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	val, err := a.client.Raw().DecrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s by %d: %w", key, value, err)
	}
	return val, nil
}

// HealthCheck verifies the Redis connection is healthy with a timeout
func (a *Adapter) HealthCheck(ctx context.Context) error {
	return a.client.HealthCheck(ctx)
}

// Close gracefully closes the Redis connection
func (a *Adapter) Close() error {
	return a.client.Close()
}
