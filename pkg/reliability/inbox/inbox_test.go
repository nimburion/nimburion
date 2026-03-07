package inbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type txState struct {
	received int
	handled  int
	entries  map[string]bool
}

type txStateKey struct{}

type fakeExecutor struct {
	state txState
}

func (e *fakeExecutor) WithTransaction(ctx context.Context, fn func(context.Context, Writer) error) error {
	working := e.state
	if working.entries == nil {
		working.entries = map[string]bool{}
	}
	writer := &fakeWriter{state: &working}
	txCtx := context.WithValue(ctx, txStateKey{}, &working)
	if err := fn(txCtx, writer); err != nil {
		return err
	}
	e.state = working
	return nil
}

type fakeWriter struct {
	state *txState
}

func (w *fakeWriter) InsertReceived(_ context.Context, entry *Entry) error {
	w.state.received++
	w.state.entries[entry.ConsumerName+":"+entry.MessageID] = false
	return nil
}

func (w *fakeWriter) IsHandled(_ context.Context, consumerName, messageID string) (bool, error) {
	handled, ok := w.state.entries[consumerName+":"+messageID]
	return ok && handled, nil
}

func (w *fakeWriter) MarkHandled(_ context.Context, consumerName, messageID string, _ time.Time) error {
	w.state.handled++
	w.state.entries[consumerName+":"+messageID] = true
	return nil
}

func TestExecuteTransactional(t *testing.T) {
	executor := &fakeExecutor{}
	entry := &Entry{ConsumerName: "consumer-a", MessageID: "msg-1"}

	handled, err := ExecuteTransactional(context.Background(), executor, entry, func(context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("execute transactional: %v", err)
	}
	if !handled {
		t.Fatal("expected handler to run")
	}
	if executor.state.received != 1 || executor.state.handled != 1 {
		t.Fatalf("unexpected tx state: %+v", executor.state)
	}
}

func TestExecuteTransactional_DoesNotCommitOnFailure(t *testing.T) {
	executor := &fakeExecutor{}
	entry := &Entry{ConsumerName: "consumer-a", MessageID: "msg-1"}

	handled, err := ExecuteTransactional(context.Background(), executor, entry, func(context.Context) error {
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected handler failure")
	}
	if handled {
		t.Fatal("should not report handled on failure")
	}
	if executor.state.received != 0 || executor.state.handled != 0 {
		t.Fatalf("unexpected committed tx state: %+v", executor.state)
	}
}
