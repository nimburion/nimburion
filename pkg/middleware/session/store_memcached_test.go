package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMemcachedClient struct {
	getFn    func(ctx context.Context, key string) ([]byte, error)
	setFn    func(ctx context.Context, key string, value []byte, ttl time.Duration) error
	deleteFn func(ctx context.Context, key string) error
	touchFn  func(ctx context.Context, key string, ttl time.Duration) error
	closeFn  func() error
}

func (f *fakeMemcachedClient) Get(ctx context.Context, key string) ([]byte, error) {
	return f.getFn(ctx, key)
}
func (f *fakeMemcachedClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return f.setFn(ctx, key, value, ttl)
}
func (f *fakeMemcachedClient) Delete(ctx context.Context, key string) error {
	return f.deleteFn(ctx, key)
}
func (f *fakeMemcachedClient) Touch(ctx context.Context, key string, ttl time.Duration) error {
	return f.touchFn(ctx, key, ttl)
}
func (f *fakeMemcachedClient) Close() error { return f.closeFn() }

func TestMemcachedStore_LoadSaveDeleteTouch(t *testing.T) {
	client := &fakeMemcachedClient{
		getFn: func(_ context.Context, key string) ([]byte, error) {
			if key != "session:s1" {
				return nil, errors.New("unexpected key")
			}
			return []byte(`{"a":"b"}`), nil
		},
		setFn: func(_ context.Context, key string, _ []byte, _ time.Duration) error {
			if key != "session:s1" {
				return errors.New("unexpected key")
			}
			return nil
		},
		deleteFn: func(_ context.Context, _ string) error { return nil },
		touchFn:  func(_ context.Context, _ string, _ time.Duration) error { return nil },
		closeFn:  func() error { return nil },
	}

	s, err := NewMemcachedStore(client, "")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx := context.Background()

	if err := s.Save(ctx, "s1", map[string]string{"a": "b"}, time.Minute); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load(ctx, "s1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got["a"] != "b" {
		t.Fatalf("unexpected loaded value: %v", got)
	}
	if err := s.Touch(ctx, "s1", time.Minute); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if err := s.Delete(ctx, "s1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestMemcachedStore_NotFoundMappings(t *testing.T) {
	ctx := context.Background()
	client := &fakeMemcachedClient{
		getFn:    func(context.Context, string) ([]byte, error) { return nil, errors.New("not found") },
		setFn:    func(context.Context, string, []byte, time.Duration) error { return nil },
		deleteFn: func(context.Context, string) error { return errors.New("not found") },
		touchFn:  func(context.Context, string, time.Duration) error { return errors.New("not found") },
		closeFn:  func() error { return nil },
	}
	s, err := NewMemcachedStore(client, "session")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	if _, err := s.Load(ctx, "x"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound from load, got %v", err)
	}
	if err := s.Delete(ctx, "x"); err != nil {
		t.Fatalf("expected nil from delete not found, got %v", err)
	}
	if err := s.Touch(ctx, "x", time.Minute); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound from touch, got %v", err)
	}
}
