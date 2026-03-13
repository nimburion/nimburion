package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/rediskit"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const defaultRedisStorePrefix = "pubsub:history"

// RedisStoreConfig configures the Redis-backed pub/sub replay store.
type RedisStoreConfig struct {
	URL              string
	Prefix           string
	MaxSize          int64
	OperationTimeout time.Duration
	MaxConns         int
}

// RedisStore persists recent published messages in Redis lists by topic.
type RedisStore struct {
	client    *rediskit.Client
	prefix    string
	maxSize   int64
	opTimeout time.Duration
}

// NewRedisStore creates a Redis-backed store using shared rediskit connectivity helpers.
func NewRedisStore(cfg RedisStoreConfig, log logger.Logger) (*RedisStore, error) {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = defaultRedisStorePrefix
	}
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 256
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 3 * time.Second
	}
	if log == nil {
		log = noopLogger{}
	}

	client, err := rediskit.NewClient(rediskit.Config{
		URL:              cfg.URL,
		MaxConns:         cfg.MaxConns,
		OperationTimeout: cfg.OperationTimeout,
	}, log)
	if err != nil {
		return nil, err
	}

	return &RedisStore{
		client:    client,
		prefix:    prefix,
		maxSize:   cfg.MaxSize,
		opTimeout: cfg.OperationTimeout,
	}, nil
}

// Append stores one message in replay history and trims to configured max size.
func (s *RedisStore) Append(ctx context.Context, msg Message) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	cctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	key := s.key(msg.Topic)
	pipe := s.client.Raw().TxPipeline()
	pipe.RPush(cctx, key, raw)
	pipe.LTrim(cctx, key, -s.maxSize, -1)
	_, err = pipe.Exec(cctx)
	if err != nil {
		err = fmt.Errorf("append message to redis history: %w", err)
		recordPubsubRedisStoreOp("append", err)
		return err
	}
	recordPubsubRedisStoreOp("append", nil)
	return nil
}

// Recent returns up to limit recent messages for the topic in chronological order.
func (s *RedisStore) Recent(ctx context.Context, topic Topic, limit int) ([]Message, error) {
	cctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	values, err := s.client.Raw().LRange(cctx, s.key(topic), 0, -1).Result()
	if err != nil {
		err = fmt.Errorf("read redis history: %w", err)
		recordPubsubRedisStoreOp("recent", err)
		return nil, err
	}
	if limit <= 0 {
		limit = len(values)
	}

	messages := make([]Message, 0, minInt(limit, len(values)))
	for _, raw := range values {
		var msg Message
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	recordPubsubRedisStoreOp("recent", nil)
	return messages, nil
}

// Close closes the underlying redis client.
func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) key(topic Topic) string {
	return fmt.Sprintf("%s:%s", s.prefix, topic)
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any)                        {}
func (noopLogger) Info(string, ...any)                         {}
func (noopLogger) Warn(string, ...any)                         {}
func (noopLogger) Error(string, ...any)                        {}
func (n noopLogger) With(...any) logger.Logger                 { return n }
func (n noopLogger) WithContext(context.Context) logger.Logger { return n }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
