package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisPrefix          = "nimburion:scheduler:lock"
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
	client *redis.Client
	log    logger.Logger
	config RedisLockProviderConfig
}

// NewRedisLockProvider creates a Redis-based lock provider.
func NewRedisLockProvider(cfg RedisLockProviderConfig, log logger.Logger) (*RedisLockProvider, error) {
	if log == nil {
		return nil, schedulerError(ErrInvalidArgument, "logger is required")
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, schedulerError(ErrInvalidArgument, "redis url is required")
	}
	cfg.normalize()

	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, errors.Join(schedulerError(ErrValidation, "parse redis url failed"), err)
	}
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.OperationTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, errors.Join(schedulerError(ErrRetryable, "ping redis failed"), err)
	}

	return &RedisLockProvider{
		client: client,
		log:    log,
		config: cfg,
	}, nil
}

// Acquire attempts to acquire a lock key with TTL.
func (p *RedisLockProvider) Acquire(ctx context.Context, key string, ttl time.Duration) (*LockLease, bool, error) {
	if p == nil || p.client == nil {
		return nil, false, schedulerError(ErrNotInitialized, "redis lock provider is not initialized")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, schedulerError(ErrInvalidArgument, "lock key is required")
	}
	if ttl <= 0 {
		return nil, false, schedulerError(ErrInvalidArgument, "ttl must be > 0")
	}

	token := randomRedisToken()
	fullKey := p.fullKey(key)

	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	acquired, err := p.client.SetNX(opCtx, fullKey, token, ttl).Result()
	if err != nil {
		return nil, false, errors.Join(schedulerError(ErrRetryable, "acquire lock failed"), err)
	}
	if !acquired {
		return nil, false, nil
	}

	return &LockLease{
		Key:      key,
		Token:    token,
		ExpireAt: time.Now().UTC().Add(ttl),
	}, true, nil
}

// Renew extends lock expiry when token still matches.
func (p *RedisLockProvider) Renew(ctx context.Context, lease *LockLease, ttl time.Duration) error {
	if p == nil || p.client == nil {
		return schedulerError(ErrNotInitialized, "redis lock provider is not initialized")
	}
	if lease == nil {
		return schedulerError(ErrInvalidArgument, "lease is required")
	}
	if ttl <= 0 {
		return schedulerError(ErrInvalidArgument, "ttl must be > 0")
	}
	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return schedulerError(ErrInvalidArgument, "lease key and token are required")
	}

	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	result, err := renewScript.Run(opCtx, p.client, []string{p.fullKey(key)}, token, ttl.Milliseconds()).Int64()
	if err != nil {
		return errors.Join(schedulerError(ErrRetryable, "renew lock failed"), err)
	}
	if result == 0 {
		return schedulerError(ErrConflict, "lock renew rejected")
	}

	lease.ExpireAt = time.Now().UTC().Add(ttl)
	return nil
}

// Release unlocks the key if the lease token matches.
func (p *RedisLockProvider) Release(ctx context.Context, lease *LockLease) error {
	if p == nil || p.client == nil {
		return schedulerError(ErrNotInitialized, "redis lock provider is not initialized")
	}
	if lease == nil {
		return schedulerError(ErrInvalidArgument, "lease is required")
	}

	key := strings.TrimSpace(lease.Key)
	token := strings.TrimSpace(lease.Token)
	if key == "" || token == "" {
		return schedulerError(ErrInvalidArgument, "lease key and token are required")
	}

	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	result, err := releaseScript.Run(opCtx, p.client, []string{p.fullKey(key)}, token).Int64()
	if err != nil {
		return errors.Join(schedulerError(ErrRetryable, "release lock failed"), err)
	}
	if result == 0 {
		return schedulerError(ErrConflict, "lock release rejected")
	}
	return nil
}

// HealthCheck verifies Redis connectivity.
func (p *RedisLockProvider) HealthCheck(ctx context.Context) error {
	if p == nil || p.client == nil {
		return schedulerError(ErrNotInitialized, "redis lock provider is not initialized")
	}
	opCtx, cancel := context.WithTimeout(ctx, p.config.OperationTimeout)
	defer cancel()
	if err := p.client.Ping(opCtx).Err(); err != nil {
		return errors.Join(schedulerError(ErrRetryable, "redis healthcheck failed"), err)
	}
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
