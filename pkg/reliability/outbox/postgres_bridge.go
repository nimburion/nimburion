package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nimburion/nimburion/pkg/persistence/relational/postgres"
)

// ErrTransactionMissing is returned when no active PostgreSQL transaction is found in the context.
var ErrTransactionMissing = errors.New("outbox transaction not found in context")

// PostgresTxManager executes functions in a PostgreSQL transaction context.
type PostgresTxManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// PostgresTxExecutor bridges relational PostgreSQL transactions with outbox transactional writes.
type PostgresTxExecutor struct {
	manager PostgresTxManager
	getTx   func(context.Context) (*sql.Tx, bool)
}

// NewPostgresTxExecutor creates an outbox TxExecutor backed by a relational PostgreSQL adapter.
func NewPostgresTxExecutor(manager PostgresTxManager) *PostgresTxExecutor {
	return &PostgresTxExecutor{manager: manager, getTx: postgres.GetTx}
}

// WithTransaction runs business and outbox writes in the same SQL transaction.
func (e *PostgresTxExecutor) WithTransaction(ctx context.Context, fn func(context.Context, Writer) error) error {
	if e == nil || e.manager == nil {
		return errors.New("postgres transaction manager is required")
	}
	if fn == nil {
		return errors.New("transaction callback is required")
	}

	return e.manager.WithTransaction(ctx, func(txCtx context.Context) error {
		tx, ok := e.getTx(txCtx)
		if !ok {
			return ErrTransactionMissing
		}
		writer := &postgresTxWriter{tx: tx}
		return fn(txCtx, writer)
	})
}

type postgresTxWriter struct {
	tx *sql.Tx
}

func (w *postgresTxWriter) Insert(ctx context.Context, entry *Entry) error {
	if w == nil || w.tx == nil {
		return errors.New("postgres transaction is required")
	}
	if entry == nil {
		return errors.New("outbox entry is required")
	}
	if err := entry.Validate(); err != nil {
		return err
	}

	headers := entry.Record.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("marshal outbox headers: %w", err)
	}

	_, err = w.tx.ExecContext(ctx, `
INSERT INTO outbox (
  id,
  topic,
  message_key,
  payload,
  headers,
  content_type,
  created_at,
  available_at,
  published,
  retry_count,
  last_error
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, $9, $10)
`, entry.ID, entry.Topic, entry.Record.Key, entry.Record.Payload, headersJSON, entry.Record.ContentType, entry.CreatedAt, entry.AvailableAt, entry.RetryCount, entry.LastError)
	if err != nil {
		return fmt.Errorf("insert outbox entry: %w", err)
	}
	return nil
}
