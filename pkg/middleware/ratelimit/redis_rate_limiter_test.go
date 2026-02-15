package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiter_AllowsWithinLimitAndResetsWindow(t *testing.T) {
	t.Parallel()

	log := newTestLogger(t)
	client := newFakeRedisClient()
	limiter := newRedisRateLimiterFromClient(client, 200*time.Millisecond, 3, 2, 100*time.Millisecond, "rl-test", log)
	defer limiter.Close()

	key := "user-42"
	limit := 5 // requestsPerSecond (3) + burst (2)
	for i := 0; i < limit; i++ {
		if !limiter.Allow(key) {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	if limiter.Allow(key) {
		t.Fatalf("expected request beyond limit to be rejected")
	}

	time.Sleep(250 * time.Millisecond)

	if !limiter.Allow(key) {
		t.Fatalf("expected limiter to reset after window")
	}
}

func newTestLogger(t *testing.T) logger.Logger {
	t.Helper()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.DebugLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	t.Cleanup(func() {
		_ = log.Sync()
	})
	return log
}

type fakeRedisClient struct {
	data      map[string]int64
	expires   map[string]time.Time
	closeHook func() error
}

func newFakeRedisClient() *fakeRedisClient {
	return &fakeRedisClient{
		data:    make(map[string]int64),
		expires: make(map[string]time.Time),
	}
}

func (c *fakeRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	if exp, ok := c.expires[key]; ok && time.Now().After(exp) {
		delete(c.data, key)
		delete(c.expires, key)
	}
	value := c.data[key] + 1
	c.data[key] = value
	return redis.NewIntResult(value, nil)
}

func (c *fakeRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	c.expires[key] = time.Now().Add(expiration)
	return redis.NewBoolResult(true, nil)
}

func (c *fakeRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (c *fakeRedisClient) Close() error {
	if c.closeHook != nil {
		return c.closeHook()
	}
	return nil
}
