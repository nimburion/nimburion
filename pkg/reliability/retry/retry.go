// Package retry provides retry helpers for transient operations.
package retry

import (
	"context"
	"errors"
	"time"
)

// Classification describes the retry semantics of an error.
type Classification string

const (
	// ClassificationRetryable marks failures that may be retried safely.
	ClassificationRetryable Classification = "retryable"
	// ClassificationPoison marks failures that should be quarantined or dead-lettered.
	ClassificationPoison Classification = "poison"
	// ClassificationTerminal marks failures that should not be retried.
	ClassificationTerminal Classification = "terminal"
)

// Disposition describes the action taken after classifying a failure.
type Disposition string

const (
	// DispositionImmediateRetry retries the operation immediately.
	DispositionImmediateRetry Disposition = "immediate_retry"
	// DispositionDelayedRetry retries the operation after a delay.
	DispositionDelayedRetry Disposition = "delayed_retry"
	// DispositionDeadLetter routes the failure to a dead-letter sink.
	DispositionDeadLetter Disposition = "dead_letter"
	// DispositionQuarantine routes the failure to quarantine.
	DispositionQuarantine Disposition = "quarantine"
	// DispositionDrop discards the failure without further handling.
	DispositionDrop Disposition = "drop"
)

type taggedError struct {
	err            error
	classification Classification
}

func (e *taggedError) Error() string { return e.err.Error() }
func (e *taggedError) Unwrap() error { return e.err }

// Retryable tags err as retryable.
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &taggedError{err: err, classification: ClassificationRetryable}
}

// Poison tags err as poison and unsuitable for normal retries.
func Poison(err error) error {
	if err == nil {
		return nil
	}
	return &taggedError{err: err, classification: ClassificationPoison}
}

// Terminal tags err as terminal and non-retryable.
func Terminal(err error) error {
	if err == nil {
		return nil
	}
	return &taggedError{err: err, classification: ClassificationTerminal}
}

// Classify returns the tagged classification for err or retryable by default.
func Classify(err error) Classification {
	if err == nil {
		return ClassificationRetryable
	}
	var tagged *taggedError
	if errors.As(err, &tagged) {
		return tagged.classification
	}
	return ClassificationRetryable
}

// Budget configures retry attempt and backoff limits.
type Budget struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	AttemptTimeout time.Duration
}

// Normalize applies defaults to unset retry budget fields.
func (b *Budget) Normalize(defaultMaxAttempts int, defaultInitialBackoff, defaultMaxBackoff, defaultAttemptTimeout time.Duration) {
	if b.MaxAttempts <= 0 {
		b.MaxAttempts = defaultMaxAttempts
	}
	if b.InitialBackoff <= 0 {
		b.InitialBackoff = defaultInitialBackoff
	}
	if b.MaxBackoff <= 0 {
		b.MaxBackoff = defaultMaxBackoff
	}
	if b.AttemptTimeout <= 0 {
		b.AttemptTimeout = defaultAttemptTimeout
	}
}

// Policy configures dead-letter and quarantine behavior.
type Policy struct {
	DeadLetterEnabled bool
	QuarantineEnabled bool
}

// Decision describes the retry handling chosen for one failure.
type Decision struct {
	Classification Classification
	Disposition    Disposition
	Attempt        int
	MaxAttempts    int
	Delay          time.Duration
}

// QuarantineRecord captures one failure routed to quarantine.
type QuarantineRecord struct {
	Scope          string
	Key            string
	Classification Classification
	Reason         string
	Attempt        int
	MaxAttempts    int
	OccurredAt     time.Time
	Metadata       map[string]string
}

// Validate checks that the quarantine record contains the required fields.
func (r *QuarantineRecord) Validate() error {
	if r == nil {
		return errors.New("quarantine record is nil")
	}
	if r.Scope == "" {
		return errors.New("quarantine scope is required")
	}
	if r.Key == "" {
		return errors.New("quarantine key is required")
	}
	if r.Reason == "" {
		return errors.New("quarantine reason is required")
	}
	if r.OccurredAt.IsZero() {
		return errors.New("quarantine occurred_at is required")
	}
	return nil
}

// QuarantineSink stores quarantined failures.
type QuarantineSink interface {
	Quarantine(ctx context.Context, record *QuarantineRecord) error
}

// Decide selects the next retry disposition for class and attempt state.
func Decide(class Classification, attempt, maxAttempts int, budget Budget, policy Policy) Decision {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	switch class {
	case ClassificationPoison:
		if policy.QuarantineEnabled {
			return Decision{Classification: class, Disposition: DispositionQuarantine, Attempt: attempt, MaxAttempts: maxAttempts}
		}
		if policy.DeadLetterEnabled {
			return Decision{Classification: class, Disposition: DispositionDeadLetter, Attempt: attempt, MaxAttempts: maxAttempts}
		}
		return Decision{Classification: class, Disposition: DispositionDrop, Attempt: attempt, MaxAttempts: maxAttempts}
	case ClassificationTerminal:
		if policy.DeadLetterEnabled {
			return Decision{Classification: class, Disposition: DispositionDeadLetter, Attempt: attempt, MaxAttempts: maxAttempts}
		}
		return Decision{Classification: class, Disposition: DispositionDrop, Attempt: attempt, MaxAttempts: maxAttempts}
	default:
		if attempt < maxAttempts {
			return Decision{
				Classification: class,
				Disposition:    DispositionDelayedRetry,
				Attempt:        attempt,
				MaxAttempts:    maxAttempts,
				Delay:          ExponentialBackoff(attempt, budget.InitialBackoff, budget.MaxBackoff),
			}
		}
		if policy.QuarantineEnabled {
			return Decision{Classification: class, Disposition: DispositionQuarantine, Attempt: attempt, MaxAttempts: maxAttempts}
		}
		if policy.DeadLetterEnabled {
			return Decision{Classification: class, Disposition: DispositionDeadLetter, Attempt: attempt, MaxAttempts: maxAttempts}
		}
		return Decision{Classification: class, Disposition: DispositionDrop, Attempt: attempt, MaxAttempts: maxAttempts}
	}
}

// ExponentialBackoff returns a capped exponential backoff for attempt.
func ExponentialBackoff(attempt int, initial, maxBackoff time.Duration) time.Duration {
	if attempt <= 0 {
		return initial
	}
	backoff := initial
	for i := 1; i < attempt; i++ {
		if backoff >= maxBackoff/2 {
			return maxBackoff
		}
		backoff *= 2
	}
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}
