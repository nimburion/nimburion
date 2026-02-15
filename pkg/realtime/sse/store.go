package sse

import "context"

// Store persists a short replay history for Last-Event-ID recovery.
type Store interface {
	Append(ctx context.Context, event Event) error
	GetSince(ctx context.Context, channel, lastEventID string, limit int) ([]Event, error)
	Close() error
}
