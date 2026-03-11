// Package cache provides shared cache backend contracts and adapters.
package cache

import (
	"errors"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// ErrCacheMiss indicates that a cache key was not found.
var ErrCacheMiss = errors.New("cache key not found")

func init() {
	coreerrors.RegisterCanonicalizer(func(err error) (*coreerrors.AppError, bool) {
		if errors.Is(err, ErrCacheMiss) {
			return coreerrors.New("cache.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404), true
		}
		return nil, false
	})
}

// Store defines a pluggable byte-oriented cache backend.
type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Close() error
}
