package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

type txOutboxState struct {
	businessRows int
	outboxRows   int
}

type txStateKey struct{}

type fakeTxExecutor struct {
	state txOutboxState
}

func (e *fakeTxExecutor) WithTransaction(ctx context.Context, fn func(ctx context.Context, writer OutboxWriter) error) error {
	working := e.state
	writer := &fakeTxOutboxWriter{state: &working}
	txCtx := context.WithValue(ctx, txStateKey{}, &working)

	if err := fn(txCtx, writer); err != nil {
		return err
	}

	e.state = working
	return nil
}

func txStateFromContext(ctx context.Context) *txOutboxState {
	state, _ := ctx.Value(txStateKey{}).(*txOutboxState)
	return state
}

type fakeTxOutboxWriter struct {
	state *txOutboxState
}

func (w *fakeTxOutboxWriter) Insert(_ context.Context, _ *OutboxEntry) error {
	w.state.outboxRows++
	return nil
}

// TestProperty_OutboxAtomicity validates Property 3: Outbox Atomicity.
//
// For any business transaction, either both business state and outbox insert
// are committed, or neither of them is committed.
//
// **Validates: Requirements 18.1**
func TestProperty_OutboxAtomicity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("business row and outbox row commit atomically", prop.ForAll(
		func(shouldBusinessFail bool, invalidEntry bool) bool {
			executor := &fakeTxExecutor{}

			entry := &OutboxEntry{
				ID:          "evt_1",
				Topic:       "events.users",
				Message:     &Message{ID: "msg_1", Key: "key_1", Value: []byte("payload")},
				CreatedAt:   time.Now().UTC(),
				AvailableAt: time.Now().UTC(),
			}
			if invalidEntry {
				entry.Topic = ""
			}

			err := ExecuteTransactionalOutbox(context.Background(), executor, entry, func(ctx context.Context) error {
				state := txStateFromContext(ctx)
				if state == nil {
					return errors.New("missing tx state")
				}
				state.businessRows++
				if shouldBusinessFail {
					return errors.New("business failure")
				}
				return nil
			})

			if shouldBusinessFail || invalidEntry {
				return err != nil && executor.state.businessRows == 0 && executor.state.outboxRows == 0
			}

			return err == nil && executor.state.businessRows == 1 && executor.state.outboxRows == 1
		},
		gen.Bool(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}
