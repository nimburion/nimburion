// Package redis provides a Redis-backed coordination lock provider.
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nimburion/nimburion/internal/rediskit"
	"github.com/nimburion/nimburion/pkg/coordination"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

func coordinationError(kind error, message string) error {
	switch {
	case errors.Is(kind, coordination.ErrValidation):
		return coreerrors.New("validation.coordination", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, coordination.ErrInvalidArgument):
		return coreerrors.New("argument.coordination.invalid", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(400)
	case errors.Is(kind, coordination.ErrConflict):
		return coreerrors.New("coordination.conflict", nil, kind).
			WithMessage(messageOrDefault(message, kind.Error())).
			WithHTTPStatus(409)
	case errors.Is(kind, coordination.ErrRetryable):
		return coreerrors.NewRetryable(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "coordination"})
	case errors.Is(kind, coordination.ErrNotInitialized):
		return coreerrors.NewNotInitialized(messageOrDefault(message, kind.Error()), kind).
			WithDetails(map[string]interface{}{"family": "coordination"})
	default:
		if message == "" {
			return kind
		}
		return fmt.Errorf("%w: %s", kind, message)
	}
}

const (
	defaultRedisPrefix          = "nimburion:coordination:lock"
	defaultRedisOperationTimout = 3 * time.Second
)

var (
	releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

	renewScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)
)

// RedisLockProviderConfig configures distributed locks backed by Redis.
type RedisLockProviderConfig struct {
	URL              string
	MetricsRegistry  *frameworkmetrics.Registry
	Prefix           string
	OperationTimeout time.Duration
}

func (c *RedisLockProviderConfig) normalize() {
	if strings.TrimSpace(c.Prefix) == "" {
		c.Prefix = defaultRedisPrefix
	}
	if c.OperationTimeout <= 0 {
		c.OperationTimeout = defaultRedisOperationTimout
	}
}

// RedisLockProvider is a distributed lock provider using Redis SET NX PX semantics.
type RedisLockProvider struct {
	client  *rediskit.Client
	log     logger.Logger
	metrics *Metrics
	config  RedisLockProviderConfig
}

// NewRedisLockProvider creates a Redis-based lock provider.
func NewRedisLockProvider(cfg RedisLockProviderConfig, log logger.Logger) (*RedisLockProvider, error) {
	if log == nil {
		return nil, coreerrors.WrapConstructorError("NewRedisLockProvider", coordinationError(coordination.ErrInvalidArgument, "logger is required"))
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, coreerrors.WrapConstructorError("NewRedisLockProvider", coordinationError(coordination.ErrInvalidArgument, "redis url is required"))
	}
	cfg.normalize()
	metrics, err := NewMetrics(cfg.MetricsRegistry)
	if err != nil {
		return nil, coreerrors.WrapConstructorError("NewRedisLockProvider", err)
	}

	client, err := rediskit.NewClient(rediskit.Config{
		URL:              cfg.URL,
		OperationTimeout: cfg.OperationTimeout,
	}, log)
	if err != nil {
		if strings.Contains(err.Error(), "parse redis URL") {
			return nil, coreerrors.WrapConstructorError("NewRedisLockProvider", errors.Join(coordinationError(coordination.ErrValidation, "parse redis url failed"), err))
		}
		return nil, coreerrors.WrapConstructorError("NewRedisLockProvider", errors.Join(coordinationError(coordination.ErrRetryable, "ping redis failed"), err))
	}

	return &RedisLockProvider{
		client:  client,
		log:     log,
		metrics: metrics,
		config:  cfg,
	}, nil
}

// Acquire attempts to acquire a lock key with TTL.
func (p *RedisLockProvider) Acquire(ctx context.Context, key string, ttl time.Duration) (*coordination.LockLease, bool, error) {
	if p == nil || p.client == nil {
		return nil, false, coordinationError(coordination.ErrNotInitialized, "redis lock provider is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, coordinationError(coordination.ErrInvalidArgument, "lock key is required")
	}
	if ttl <= 0 {
		return nil, false, coordinationError(coordination.ErrInvalidArgument, "ttl must be > 0")
	}

	token := randomRedisToken()
	fullKey := p.fullKey(key)

	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	status, err := p.client.Raw().SetArgs(opCtx, fullKey, token, redis.SetArgs{Mode: "NX", TTL: ttl}).Result()
	if err != nil {
		p.metrics.record("acquire", err)
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, errors.Join(coordinationError(coordination.ErrRetryable, "acquire lock failed"), err)
	}
	if status == "" {
		p.metrics.record("acquire", nil)
		return nil, false, nil
	}

	p.metrics.record("acquire", nil)
	return &coordination.LockLease{
		Key:      key,
		Token:    token,
		ExpireAt: time.Now().UTC().Add(ttl),
	}, true, nil
}

// Renew extends lock expiry when token still matches.
func (p *RedisLockProvider) Renew(ctx context.Context, lease *coordination.LockLease, ttl time.Duration) error {
	if p == nil || p.client == nil {
		return coordinationError(coordination.ErrNotInitialized, "redis lock provider is not initialized")
	}
	if lease == nil {
		return coordinationError(coordination.ErrInvalidArgument, "lease is required")
	}
	if ttl <= 0 {
		return coordinationError(coordination.ErrInvalidArgument, "ttl must be > 0")
	}
	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return coordinationError(coordination.ErrInvalidArgument, "lease key and token are required")
	}

	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	result, err := renewScript.Run(opCtx, p.client.Raw(), []string{p.fullKey(key)}, token, ttl.Milliseconds()).Int64()
	if err != nil {
		p.metrics.record("renew", err)
		return errors.Join(coordinationError(coordination.ErrRetryable, "renew lock failed"), err)
	}
	if result == 0 {
		err = coordinationError(coordination.ErrConflict, "lock renew rejected")
		p.metrics.record("renew", err)
		return err
	}

	lease.ExpireAt = time.Now().UTC().Add(ttl)
	p.metrics.record("renew", nil)
	return nil
}

// Release unlocks the key if the lease token matches.
func (p *RedisLockProvider) Release(ctx context.Context, lease *coordination.LockLease) error {
	if p == nil || p.client == nil {
		return coordinationError(coordination.ErrNotInitialized, "redis lock provider is not initialized")
	}
	if lease == nil {
		return coordinationError(coordination.ErrInvalidArgument, "lease is required")
	}

	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return coordinationError(coordination.ErrInvalidArgument, "lease key and token are required")
	}

	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	result, err := releaseScript.Run(opCtx, p.client.Raw(), []string{p.fullKey(key)}, token).Int64()
	if err != nil {
		p.metrics.record("release", err)
		return errors.Join(coordinationError(coordination.ErrRetryable, "release lock failed"), err)
	}
	if result == 0 {
		err = coordinationError(coordination.ErrConflict, "lock release rejected")
		p.metrics.record("release", err)
		return err
	}
	p.metrics.record("release", nil)
	return nil
}

// HealthCheck verifies Redis connectivity.
func (p *RedisLockProvider) HealthCheck(ctx context.Context) error {
	if p == nil || p.client == nil {
		return coordinationError(coordination.ErrNotInitialized, "redis lock provider is not initialized")
	}
	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	if err := p.client.HealthCheck(opCtx); err != nil {
		p.metrics.record("healthcheck", err)
		return errors.Join(coordinationError(coordination.ErrRetryable, "redis healthcheck failed"), err)
	}
	p.metrics.record("healthcheck", nil)
	return nil
}

// Close closes Redis client connections.
func (p *RedisLockProvider) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}

func (p *RedisLockProvider) fullKey(key string) string {
	return strings.TrimRight(p.config.Prefix, ":") + ":" + strings.TrimSpace(key)
}

func randomRedisToken() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func messageOrDefault(message, fallback string) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	return fallback
}
