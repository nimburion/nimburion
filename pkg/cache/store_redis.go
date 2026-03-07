package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Close() error
}

// RedisConfig configures a Redis cache backend.
type RedisConfig struct {
	URL              string
	MaxConns         int
	OperationTimeout time.Duration
	Prefix           string
}

// RedisStore persists cache entries in Redis.
type RedisStore struct {
	client    redisClient
	opTimeout time.Duration
	prefix    string
}

// NewRedisStore creates a Redis-backed cache store.
func NewRedisStore(cfg RedisConfig) (*RedisStore, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("redis cache url is required")
	}
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	if cfg.MaxConns > 0 {
		opts.PoolSize = cfg.MaxConns
	}
	client := redis.NewClient(opts)

	timeout := cfg.OperationTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = "http-cache"
	}

	return &RedisStore{
		client:    client,
		opTimeout: timeout,
		prefix:    prefix,
	}, nil
}

// Get loads an entry from Redis.
func (s *RedisStore) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.opTimeout)
	defer cancel()
	raw, err := s.client.Get(ctx, s.key(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	return []byte(raw), nil
}

// Set stores an entry with TTL.
func (s *RedisStore) Set(key string, value []byte, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.opTimeout)
	defer cancel()
	return s.client.Set(ctx, s.key(key), value, ttl).Err()
}

// Delete removes an entry.
func (s *RedisStore) Delete(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.opTimeout)
	defer cancel()
	return s.client.Del(ctx, s.key(key)).Err()
}

// Close closes Redis client.
func (s *RedisStore) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) key(key string) string {
	return fmt.Sprintf("%s:%s", s.prefix, key)
}
