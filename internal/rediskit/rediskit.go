// Package rediskit provides low-level helpers shared by Redis-backed adapters.
package rediskit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Config holds shared Redis connectivity settings for role-specific adapters.
type Config struct {
	URL              string
	MaxConns         int
	OperationTimeout time.Duration
}

// Client wraps a configured Redis client plus shared lifecycle helpers.
type Client struct {
	client *redis.Client
	logger logger.Logger
	config Config
}

// NewClient creates and validates a Redis client for a role-specific adapter.
func NewClient(cfg Config, log logger.Logger) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("redis URL is required")
	}

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	opts.PoolSize = cfg.MaxConns
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = cfg.OperationTimeout
	opts.WriteTimeout = cfg.OperationTimeout

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			return nil, errors.Join(
				fmt.Errorf("failed to ping redis: %w", err),
				fmt.Errorf("failed to close redis client after ping failure: %w", closeErr),
			)
		}
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	log.Info("Redis connection established",
		"max_conns", cfg.MaxConns,
		"operation_timeout", cfg.OperationTimeout,
	)

	return &Client{
		client: client,
		logger: log,
		config: cfg,
	}, nil
}

// Raw returns the underlying Redis client.
func (c *Client) Raw() *redis.Client {
	return c.client
}

// Ping verifies the Redis connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// HealthCheck verifies the Redis connection with a short timeout.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := c.client.Ping(ctx).Err(); err != nil {
		c.logger.Error("Redis health check failed", "error", err)
		return fmt.Errorf("redis health check failed: %w", err)
	}

	return nil
}

// Close gracefully closes the Redis client.
func (c *Client) Close() error {
	c.logger.Info("closing Redis connection")

	if err := c.client.Close(); err != nil {
		c.logger.Error("failed to close Redis connection", "error", err)
		return fmt.Errorf("failed to close redis connection: %w", err)
	}

	c.logger.Info("Redis connection closed successfully")
	return nil
}
