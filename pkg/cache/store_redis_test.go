package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/redis/go-redis/v9"
)

type fakeRedisClient struct {
	getFn   func(ctx context.Context, key string) *redis.StringCmd
	setFn   func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	delFn   func(ctx context.Context, keys ...string) *redis.IntCmd
	closeFn func() error
}

func (f *fakeRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	return f.getFn(ctx, key)
}
func (f *fakeRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return f.setFn(ctx, key, value, expiration)
}
func (f *fakeRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return f.delFn(ctx, keys...)
}
func (f *fakeRedisClient) Close() error { return f.closeFn() }

func TestRedisStore_KeyAndMappings(t *testing.T) {
	store := &RedisStore{
		client: &fakeRedisClient{
			getFn: func(_ context.Context, _ string) *redis.StringCmd {
				return redis.NewStringResult("", redis.Nil)
			},
			setFn: func(_ context.Context, _ string, _ interface{}, _ time.Duration) *redis.StatusCmd {
				return redis.NewStatusResult("OK", nil)
			},
			delFn: func(_ context.Context, _ ...string) *redis.IntCmd {
				return redis.NewIntResult(1, nil)
			},
			closeFn: func() error { return nil },
		},
		opTimeout: time.Second,
		prefix:    "http-cache",
	}

	if key := store.key("abc"); key != "http-cache:abc" {
		t.Fatalf("unexpected key: %s", key)
	}
	if _, err := store.Get("abc"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss from get mapping, got %v", err)
	}
}

func TestRedisStore_NewValidationAndClose(t *testing.T) {
	if _, err := NewRedisStore(RedisConfig{}); err == nil {
		t.Fatal("expected validation error for empty URL")
	} else {
		var appErr *coreerrors.AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != "validation.cache.redis.url.required" {
			t.Fatalf("Code = %q", appErr.Code)
		}
	}

	store := &RedisStore{
		client: &fakeRedisClient{
			getFn: func(context.Context, string) *redis.StringCmd { return redis.NewStringResult("", nil) },
			setFn: func(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
				return redis.NewStatusResult("OK", nil)
			},
			delFn:   func(context.Context, ...string) *redis.IntCmd { return redis.NewIntResult(0, nil) },
			closeFn: func() error { return errors.New("boom") },
		},
		opTimeout: time.Second,
		prefix:    "http-cache",
	}
	if err := store.Close(); err == nil || err.Error() != "boom" {
		t.Fatalf("expected close error propagation, got %v", err)
	}
}
