package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestConsumeWithRetry_SucceedsAfterRetries(t *testing.T) {
	producer := &fakeProducer{}
	calls := 0

	err := ConsumeWithRetry(
		context.Background(),
		"events.orders",
		&Message{ID: "m1", Key: "k1", Value: []byte("payload")},
		func(context.Context, *Message) error {
			calls++
			if calls < 3 {
				return errors.New("temporary failure")
			}
			return nil
		},
		producer,
		RetryDLQConfig{MaxRetries: 5, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, AttemptTimeout: time.Second},
		testLogger{},
		nil,
	)
	if err != nil {
		t.Fatalf("expected eventual success, got error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
	if len(producer.messages) != 0 {
		t.Fatalf("did not expect dlq publish on success")
	}
}

func TestConsumeWithRetry_SendsToDLQOnPermanentFailure(t *testing.T) {
	producer := &fakeProducer{}

	err := ConsumeWithRetry(
		context.Background(),
		"events.orders",
		&Message{ID: "m1", Key: "k1", Value: []byte("payload")},
		func(context.Context, *Message) error {
			return errors.New("permanent failure")
		},
		producer,
		RetryDLQConfig{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, AttemptTimeout: time.Second},
		testLogger{},
		nil,
	)
	if err == nil {
		t.Fatalf("expected failure after retries")
	}

	if len(producer.messages) != 1 {
		t.Fatalf("expected one dlq message, got %d", len(producer.messages))
	}
	if len(producer.topics) != 1 || producer.topics[0] != "events.orders.dlq" {
		t.Fatalf("unexpected dlq topic: %+v", producer.topics)
	}

	var payload DLQPayload
	if jsonErr := json.Unmarshal(producer.messages[0].Value, &payload); jsonErr != nil {
		t.Fatalf("invalid dlq payload json: %v", jsonErr)
	}
	if payload.OriginalTopic != "events.orders" {
		t.Fatalf("unexpected original topic: %s", payload.OriginalTopic)
	}
	if payload.AttemptCount != 3 {
		t.Fatalf("expected attempt_count=3, got %d", payload.AttemptCount)
	}
	if payload.FailureReason == "" || payload.StackTrace == "" {
		t.Fatalf("expected failure reason and stack trace in payload")
	}
}

func TestConsumeWithRetry_RespectsAttemptTimeout(t *testing.T) {
	producer := &fakeProducer{}

	err := ConsumeWithRetry(
		context.Background(),
		"events.orders",
		&Message{ID: "m1", Key: "k1", Value: []byte("payload")},
		func(ctx context.Context, _ *Message) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return nil
			}
		},
		producer,
		RetryDLQConfig{MaxRetries: 1, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, AttemptTimeout: 5 * time.Millisecond},
		testLogger{},
		nil,
	)
	if err == nil {
		t.Fatalf("expected timeout-related failure")
	}
	if len(producer.messages) != 1 {
		t.Fatalf("expected one dlq message after timeout retries")
	}
}
