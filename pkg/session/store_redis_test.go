package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisClient struct {
	getFn    func(ctx context.Context, key string) *redis.StringCmd
	setFn    func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	delFn    func(ctx context.Context, keys ...string) *redis.IntCmd
	expireFn func(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	closeFn  func() error
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
func (f *fakeRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	return f.expireFn(ctx, key, expiration)
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
			expireFn: func(_ context.Context, _ string, _ time.Duration) *redis.BoolCmd {
				return redis.NewBoolResult(false, nil)
			},
			closeFn: func() error { return nil },
		},
		opTimeout: time.Second,
		prefix:    "session",
	}

	if key := store.key("abc"); key != "session:abc" {
		t.Fatalf("unexpected key: %s", key)
	}
	if _, err := store.Load(context.Background(), "abc"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound from load mapping, got %v", err)
	}
	if err := store.Touch(context.Background(), "abc", time.Minute); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound from touch mapping, got %v", err)
	}
}

func TestRedisStore_NewValidationAndClose(t *testing.T) {
	if _, err := NewRedisStore(RedisConfig{}); err == nil {
		t.Fatal("expected validation error for empty URL")
	}

	store := &RedisStore{
		client: &fakeRedisClient{
			getFn: func(context.Context, string) *redis.StringCmd { return redis.NewStringResult("", nil) },
			setFn: func(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
				return redis.NewStatusResult("OK", nil)
			},
			delFn:    func(context.Context, ...string) *redis.IntCmd { return redis.NewIntResult(0, nil) },
			expireFn: func(context.Context, string, time.Duration) *redis.BoolCmd { return redis.NewBoolResult(true, nil) },
			closeFn:  func() error { return errors.New("boom") },
		},
		opTimeout: time.Second,
		prefix:    "session",
	}
	if err := store.Close(); err == nil || err.Error() != "boom" {
		t.Fatalf("expected close error propagation, got %v", err)
	}
}
