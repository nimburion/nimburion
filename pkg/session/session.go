// Package session provides shared session backend contracts and adapters.
package session

import (
	"context"
	"errors"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// ErrNotFound indicates that a session does not exist in the backend store.
var ErrNotFound = errors.New("session not found")

func init() {
	coreerrors.RegisterCanonicalizer(func(err error) (*coreerrors.AppError, bool) {
		if errors.Is(err, ErrNotFound) {
			return coreerrors.New("session.not_found", nil, err).WithMessage(err.Error()).WithHTTPStatus(404), true
		}
		return nil, false
	})
}

// Store defines a pluggable session backend.
type Store interface {
	Load(ctx context.Context, id string) (map[string]string, error)
	Save(ctx context.Context, id string, data map[string]string, ttl time.Duration) error
	Delete(ctx context.Context, id string) error
	Touch(ctx context.Context, id string, ttl time.Duration) error
	Close() error
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
