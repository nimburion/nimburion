package session

import (
	"context"
	"encoding/json"
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
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Close() error
}

// RedisConfig configures the Redis-backed session store.
type RedisConfig struct {
	URL              string
	MaxConns         int
	OperationTimeout time.Duration
	Prefix           string
}

// RedisStore persists session payloads in Redis.
type RedisStore struct {
	client    redisClient
	opTimeout time.Duration
	prefix    string
}

// NewRedisStore creates a Redis-backed session store.
func NewRedisStore(cfg RedisConfig) (*RedisStore, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("redis session store url is required")
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
		prefix = "session"
	}

	return &RedisStore{
		client:    client,
		opTimeout: timeout,
		prefix:    prefix,
	}, nil
}

// Load fetches a session by id.
func (s *RedisStore) Load(ctx context.Context, id string) (map[string]string, error) {
	innerCtx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	raw, err := s.client.Get(innerCtx, s.key(id)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("decode session payload: %w", err)
	}
	return payload, nil
}

// Save persists a session payload.
func (s *RedisStore) Save(ctx context.Context, id string, data map[string]string, ttl time.Duration) error {
	innerCtx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode session payload: %w", err)
	}
	return s.client.Set(innerCtx, s.key(id), raw, ttl).Err()
}

// Delete removes a session payload.
func (s *RedisStore) Delete(ctx context.Context, id string) error {
	innerCtx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()
	return s.client.Del(innerCtx, s.key(id)).Err()
}

// Touch extends the TTL of an existing session.
func (s *RedisStore) Touch(ctx context.Context, id string, ttl time.Duration) error {
	innerCtx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()
	ok, err := s.client.Expire(innerCtx, s.key(id), ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// Close releases the underlying Redis client.
func (s *RedisStore) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) key(id string) string {
	return fmt.Sprintf("%s:%s", s.prefix, id)
}
