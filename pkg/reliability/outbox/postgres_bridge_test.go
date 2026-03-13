package outbox

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type txManagerStub struct {
	db  *sql.DB
	key any
}

func (m txManagerStub) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txCtx := context.WithValue(ctx, m.key, tx)
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

func TestPostgresTxExecutor_WithTransactionUsesSameTxForBusinessAndOutbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	key := struct{}{}
	manager := txManagerStub{db: db, key: key}
	executor := NewPostgresTxExecutor(manager)
	executor.getTx = func(ctx context.Context) (*sql.Tx, bool) {
		tx, ok := ctx.Value(key).(*sql.Tx)
		return tx, ok
	}

	entry := &Entry{
		ID:          "outbox-1",
		Topic:       "orders.created",
		Record:      &Record{ID: "record-1", Key: "k-1", Payload: []byte("payload"), Headers: map[string]string{"trace": "abc"}, ContentType: "application/json"},
		CreatedAt:   time.Now().UTC(),
		AvailableAt: time.Now().UTC(),
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO business_events (id) VALUES ($1)")).
		WithArgs("biz-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox (")).
		WithArgs(
			entry.ID,
			entry.Topic,
			entry.Record.Key,
			entry.Record.Payload,
			sqlmock.AnyArg(),
			entry.Record.ContentType,
			entry.CreatedAt,
			entry.AvailableAt,
			entry.RetryCount,
			entry.LastError,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var businessTx *sql.Tx
	var writerTx *sql.Tx
	err = executor.WithTransaction(context.Background(), func(ctx context.Context, writer Writer) error {
		tx, ok := executor.getTx(ctx)
		if !ok {
			return errors.New("missing tx for business logic")
		}
		businessTx = tx
		if _, err := tx.ExecContext(ctx, "INSERT INTO business_events (id) VALUES ($1)", "biz-1"); err != nil {
			return err
		}

		w, ok := writer.(*postgresTxWriter)
		if !ok {
			return errors.New("unexpected writer type")
		}
		writerTx = w.tx
		return writer.Insert(ctx, entry)
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}
	if businessTx == nil || writerTx == nil {
		t.Fatal("expected non-nil transaction handles")
	}
	if businessTx != writerTx {
		t.Fatal("expected business logic and outbox write to use the same transaction")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
