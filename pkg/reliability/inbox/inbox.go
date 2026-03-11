// Package inbox provides inbox processing primitives for reliable consumers.
package inbox

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CreateTablePostgres is the PostgreSQL schema for inbox state storage.
const CreateTablePostgres = `
CREATE TABLE IF NOT EXISTS inbox (
  consumer_name TEXT NOT NULL,
  message_id TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  handled_at TIMESTAMPTZ,
  status TEXT NOT NULL,
  PRIMARY KEY (consumer_name, message_id)
);

CREATE INDEX IF NOT EXISTS idx_inbox_status_received_at ON inbox (status, received_at);
`

const (
	// StatusReceived marks an inbox message as received but not yet handled.
	StatusReceived = "received"
	// StatusHandled marks an inbox message as fully handled.
	StatusHandled = "handled"
)

// Entry represents one inbox message state record.
type Entry struct {
	ConsumerName string
	MessageID    string
	ReceivedAt   time.Time
	HandledAt    *time.Time
	Status       string
}

// Validate checks that the inbox entry contains the required fields.
func (e *Entry) Validate() error {
	if e == nil {
		return errors.New("inbox entry is nil")
	}
	if e.ConsumerName == "" {
		return errors.New("inbox consumer name is required")
	}
	if e.MessageID == "" {
		return errors.New("inbox message id is required")
	}
	switch e.Status {
	case "", StatusReceived, StatusHandled:
		return nil
	default:
		return fmt.Errorf("unsupported inbox status %q", e.Status)
	}
}

// Store persists inbox message state.
type Store interface {
	InsertReceived(ctx context.Context, entry *Entry) error
	IsHandled(ctx context.Context, consumerName, messageID string) (bool, error)
	MarkHandled(ctx context.Context, consumerName, messageID string, handledAt time.Time) error
	CleanupHandledBefore(ctx context.Context, before time.Time, limit int) (int, error)
}

// Writer exposes inbox operations inside a transaction boundary.
type Writer interface {
	InsertReceived(ctx context.Context, entry *Entry) error
	MarkHandled(ctx context.Context, consumerName, messageID string, handledAt time.Time) error
	IsHandled(ctx context.Context, consumerName, messageID string) (bool, error)
}

// TxExecutor executes inbox operations in a transaction boundary.
type TxExecutor interface {
	WithTransaction(ctx context.Context, fn func(context.Context, Writer) error) error
}

// ExecuteTransactional records inbox state and runs handler atomically.
func ExecuteTransactional(ctx context.Context, executor TxExecutor, entry *Entry, handler func(context.Context) error) (bool, error) {
	if executor == nil {
		return false, errors.New("transaction executor is required")
	}
	if entry == nil {
		return false, errors.New("inbox entry is required")
	}
	if err := entry.Validate(); err != nil {
		return false, err
	}
	if handler == nil {
		return false, errors.New("handler is required")
	}

	handled := false
	err := executor.WithTransaction(ctx, func(txCtx context.Context, writer Writer) error {
		alreadyHandled, err := writer.IsHandled(txCtx, entry.ConsumerName, entry.MessageID)
		if err != nil {
			return fmt.Errorf("check inbox state failed: %w", err)
		}
		if alreadyHandled {
			handled = false
			return nil
		}

		entryCopy := *entry
		if entryCopy.Status == "" {
			entryCopy.Status = StatusReceived
		}
		if entryCopy.ReceivedAt.IsZero() {
			entryCopy.ReceivedAt = time.Now().UTC()
		}
		if err := writer.InsertReceived(txCtx, &entryCopy); err != nil {
			return fmt.Errorf("record inbox receipt failed: %w", err)
		}
		if err := handler(txCtx); err != nil {
			return err
		}
		handledAt := time.Now().UTC()
		if err := writer.MarkHandled(txCtx, entry.ConsumerName, entry.MessageID, handledAt); err != nil {
			return fmt.Errorf("mark inbox handled failed: %w", err)
		}
		handled = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return handled, nil
}
