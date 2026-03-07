// Package cache provides shared cache backend contracts and adapters.
package cache

import (
	"errors"
	"time"
)

// ErrCacheMiss indicates that a cache key was not found.
var ErrCacheMiss = errors.New("cache key not found")

// Store defines a pluggable byte-oriented cache backend.
type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Close() error
}
