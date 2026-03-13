package pubsub

import "context"

// Store persists a best-effort replay history for published pub/sub messages.
type Store interface {
	Append(ctx context.Context, msg Message) error
	Recent(ctx context.Context, topic Topic, limit int) ([]Message, error)
	Close() error
}
