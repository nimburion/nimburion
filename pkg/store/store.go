package store

import "context"

// Adapter is the minimal lifecycle and health contract for storage adapters.
type Adapter interface {
	HealthCheck(ctx context.Context) error
	Close() error
}
