package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nimburion/nimburion/internal/rediskit"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	frameworkmetrics "github.com/nimburion/nimburion/pkg/observability/metrics"
)

// RedisStoreConfig configures Redis replay storage.
type RedisStoreConfig struct {
	URL              string
	Prefix           string
	MaxSize          int64
	MetricsRegistry  *frameworkmetrics.Registry
	OperationTimeout time.Duration
	MaxConns         int
}

// RedisStore persists replay history in Redis lists.
type RedisStore struct {
	client    *rediskit.Client
	metrics   *Metrics
	prefix    string
	maxSize   int64
	opTimeout time.Duration
}

// NewRedisStore creates a Redis replay store.
func NewRedisStore(cfg RedisStoreConfig) (*RedisStore, error) {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = "sse:history"
	}
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 256
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 3 * time.Second
	}
	metrics, err := NewMetrics(cfg.MetricsRegistry)
	if err != nil {
		return nil, coreerrors.WrapConstructorError("NewRedisStore", err)
	}
	client, err := rediskit.NewClient(rediskit.Config{
		URL:              cfg.URL,
		MaxConns:         cfg.MaxConns,
		OperationTimeout: cfg.OperationTimeout,
	}, noopLogger{})
	if err != nil {
		metrics.record("store", "get_since", err)
		return nil, coreerrors.WrapConstructorError("NewRedisStore", err)
	}

	return &RedisStore{
		client:    client,
		metrics:   metrics,
		prefix:    prefix,
		maxSize:   cfg.MaxSize,
		opTimeout: cfg.OperationTimeout,
	}, nil
}

// Append stores one event and trims replay list to max size.
func (s *RedisStore) Append(ctx context.Context, event Event) error {
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	key := s.key(event.Channel)
	pipe := s.client.Raw().TxPipeline()
	pipe.RPush(cctx, key, raw)
	pipe.LTrim(cctx, key, -s.maxSize, -1)
	_, err = pipe.Exec(cctx)
	s.metrics.record("store", "append", err)
	return err
}

// GetSince returns chronological replay events newer than lastEventID.
func (s *RedisStore) GetSince(ctx context.Context, channel, lastEventID string, limit int) ([]Event, error) {
	cctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	values, err := s.client.Raw().LRange(cctx, s.key(channel), 0, -1).Result()
	if err != nil {
		s.metrics.record("store", "get_since", err)
		return nil, err
	}
	if limit <= 0 {
		limit = len(values)
	}

	out := make([]Event, 0, minInt(limit, len(values)))
	for _, raw := range values {
		var evt Event
		if err := json.Unmarshal([]byte(raw), &evt); err != nil {
			continue
		}
		if lastEventID == "" || evt.ID > lastEventID {
			out = append(out, evt)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	s.metrics.record("store", "get_since", nil)
	return out, nil
}

// Close closes Redis client.
func (s *RedisStore) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) key(channel string) string {
	return fmt.Sprintf("%s:%s", s.prefix, channel)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
