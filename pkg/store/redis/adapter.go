package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// RedisAdapter provides Redis cache connectivity with connection pooling
type RedisAdapter struct {
	client *redis.Client
	logger logger.Logger
	config Config
}

// Config holds Redis connection configuration
type Config struct {
	URL              string
	MaxConns         int
	OperationTimeout time.Duration
}

// NewRedisAdapter creates a new Redis adapter with connection pooling
func NewRedisAdapter(cfg Config, log logger.Logger) (*RedisAdapter, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("redis URL is required")
	}

	// Parse Redis URL
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	// Configure connection pool
	opts.PoolSize = cfg.MaxConns
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = cfg.OperationTimeout
	opts.WriteTimeout = cfg.OperationTimeout

	// Create Redis client
	client := redis.NewClient(opts)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	log.Info("Redis connection established",
		"max_conns", cfg.MaxConns,
		"operation_timeout", cfg.OperationTimeout,
	)

	return &RedisAdapter{
		client: client,
		logger: log,
		config: cfg,
	}, nil
}

// Client returns the underlying *redis.Client for direct access when needed
func (a *RedisAdapter) Client() *redis.Client {
	return a.client
}

// Ping verifies the Redis connection is alive
func (a *RedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

// Get retrieves a value from Redis by key
func (a *RedisAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := a.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Set stores a key-value pair in Redis without expiration
func (a *RedisAdapter) Set(ctx context.Context, key string, value interface{}) error {
	if err := a.client.Set(ctx, key, value, 0).Err(); err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}
	return nil
}

// SetWithTTL stores a key-value pair in Redis with expiration
func (a *RedisAdapter) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := a.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set key %s with TTL: %w", key, err)
	}
	return nil
}

// Delete removes a key from Redis
func (a *RedisAdapter) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	
	if err := a.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}
	return nil
}

// Incr atomically increments the value of a key by 1
func (a *RedisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	val, err := a.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return val, nil
}

// IncrBy atomically increments the value of a key by the specified amount
func (a *RedisAdapter) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	val, err := a.client.IncrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s by %d: %w", key, value, err)
	}
	return val, nil
}

// Decr atomically decrements the value of a key by 1
func (a *RedisAdapter) Decr(ctx context.Context, key string) (int64, error) {
	val, err := a.client.Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	return val, nil
}

// DecrBy atomically decrements the value of a key by the specified amount
func (a *RedisAdapter) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	val, err := a.client.DecrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s by %d: %w", key, value, err)
	}
	return val, nil
}

// HealthCheck verifies the Redis connection is healthy with a timeout
func (a *RedisAdapter) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := a.client.Ping(ctx).Err(); err != nil {
		a.logger.Error("Redis health check failed", "error", err)
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// Close gracefully closes the Redis connection
func (a *RedisAdapter) Close() error {
	a.logger.Info("closing Redis connection")
	
	if err := a.client.Close(); err != nil {
		a.logger.Error("failed to close Redis connection", "error", err)
		return fmt.Errorf("failed to close redis connection: %w", err)
	}

	a.logger.Info("Redis connection closed successfully")
	return nil
}
