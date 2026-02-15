package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type redisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
}

// RedisRateLimiter implements a simple distributed counter backed by Redis.
type RedisRateLimiter struct {
	client    redisClient
	limit     int
	burst     int
	window    time.Duration
	opTimeout time.Duration
	prefix    string
	log       logger.Logger
}

// NewRedisRateLimiter creates a Redis-backed rate limiter.
func NewRedisRateLimiter(
	cfg config.RateLimitRedisConfig,
	window time.Duration,
	requestsPerSecond, burst int,
	log logger.Logger,
) (*RedisRateLimiter, error) {
	if cfg.URL == "" {
		return nil, errors.New("redis URL is required for distributed rate limiting")
	}
	if requestsPerSecond <= 0 {
		return nil, errors.New("requests_per_second must be greater than zero")
	}
	if burst < 0 {
		return nil, errors.New("burst cannot be negative")
	}
	if window <= 0 {
		window = time.Second
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}
	if cfg.MaxConns > 0 {
		opts.PoolSize = cfg.MaxConns
	}
	timeout := cfg.OperationTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	opts.ReadTimeout = timeout
	opts.WriteTimeout = timeout

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis rate limiter ping failed: %w", err)
	}

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "ratelimit"
	}

	log.Info("redis rate limiter connected",
		"limit", requestsPerSecond,
		"burst", burst,
		"window", window,
		"prefix", prefix,
	)

	return newRedisRateLimiterFromClient(client, window, requestsPerSecond, burst, timeout, prefix, log), nil
}

func newRedisRateLimiterFromClient(
	client redisClient,
	window time.Duration,
	requestsPerSecond, burst int,
	timeout time.Duration,
	prefix string,
	log logger.Logger,
) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:    client,
		limit:     requestsPerSecond,
		burst:     burst,
		window:    window,
		opTimeout: timeout,
		prefix:    prefix,
		log:       log,
	}
}

// Allow determines whether the request identified by key can proceed.
func (r *RedisRateLimiter) Allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.opTimeout)
	defer cancel()

	redisKey := r.redisKey(key)

	count, err := r.client.Incr(ctx, redisKey).Result()
	if err != nil {
		r.log.Error("redis rate limiter increment failed", "error", err)
		// Fail-open to avoid blocking traffic when Redis is unavailable.
		return true
	}

	if count == 1 {
		if err := r.client.Expire(ctx, redisKey, r.window).Err(); err != nil {
			r.log.Warn("redis rate limiter failed to set TTL", "error", err)
		}
	}

	limit := int64(r.limit + r.burst)
	return limit == 0 || count <= limit
}

// Close shuts down the Redis client.
func (r *RedisRateLimiter) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (r *RedisRateLimiter) redisKey(key string) string {
	if r.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", r.prefix, key)
}
