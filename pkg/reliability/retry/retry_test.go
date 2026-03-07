package retry

import (
	"errors"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	if got := Classify(Poison(errors.New("bad payload"))); got != ClassificationPoison {
		t.Fatalf("expected poison, got %s", got)
	}
	if got := Classify(Terminal(errors.New("terminal"))); got != ClassificationTerminal {
		t.Fatalf("expected terminal, got %s", got)
	}
	if got := Classify(errors.New("transient")); got != ClassificationRetryable {
		t.Fatalf("expected retryable default, got %s", got)
	}
}

func TestDecide(t *testing.T) {
	budget := Budget{InitialBackoff: time.Second, MaxBackoff: 10 * time.Second}

	retryDecision := Decide(ClassificationRetryable, 1, 3, budget, Policy{DeadLetterEnabled: true})
	if retryDecision.Disposition != DispositionDelayedRetry {
		t.Fatalf("expected delayed retry, got %s", retryDecision.Disposition)
	}

	poisonDecision := Decide(ClassificationPoison, 1, 3, budget, Policy{QuarantineEnabled: true, DeadLetterEnabled: true})
	if poisonDecision.Disposition != DispositionQuarantine {
		t.Fatalf("expected quarantine, got %s", poisonDecision.Disposition)
	}

	terminalDecision := Decide(ClassificationTerminal, 3, 3, budget, Policy{DeadLetterEnabled: true})
	if terminalDecision.Disposition != DispositionDeadLetter {
		t.Fatalf("expected dead letter, got %s", terminalDecision.Disposition)
	}
}

func TestExponentialBackoffCapsAtMax(t *testing.T) {
	if got := ExponentialBackoff(10, time.Second, 5*time.Second); got != 5*time.Second {
		t.Fatalf("expected capped backoff, got %v", got)
	}
}

func TestQuarantineRecordValidate(t *testing.T) {
	record := &QuarantineRecord{
		Scope:      "eventbus",
		Key:        "events.orders:m1",
		Reason:     "bad payload",
		OccurredAt: time.Now().UTC(),
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("expected valid record, got %v", err)
	}
}
