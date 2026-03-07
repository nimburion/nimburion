package retry

import (
	"context"
	"errors"
	"time"
)

type Classification string

const (
	ClassificationRetryable Classification = "retryable"
	ClassificationPoison    Classification = "poison"
	ClassificationTerminal  Classification = "terminal"
)

type Disposition string

const (
	DispositionImmediateRetry Disposition = "immediate_retry"
	DispositionDelayedRetry   Disposition = "delayed_retry"
	DispositionDeadLetter     Disposition = "dead_letter"
	DispositionQuarantine     Disposition = "quarantine"
	DispositionDrop           Disposition = "drop"
)

type taggedError struct {
	err            error
	classification Classification
}

func (e *taggedError) Error() string { return e.err.Error() }
func (e *taggedError) Unwrap() error { return e.err }

func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &taggedError{err: err, classification: ClassificationRetryable}
}

func Poison(err error) error {
	if err == nil {
		return nil
	}
	return &taggedError{err: err, classification: ClassificationPoison}
}

func Terminal(err error) error {
	if err == nil {
		return nil
	}
	return &taggedError{err: err, classification: ClassificationTerminal}
}

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

type Budget struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	AttemptTimeout time.Duration
}

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

type Policy struct {
	DeadLetterEnabled bool
	QuarantineEnabled bool
}

type Decision struct {
	Classification Classification
	Disposition    Disposition
	Attempt        int
	MaxAttempts    int
	Delay          time.Duration
}

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

type QuarantineSink interface {
	Quarantine(ctx context.Context, record *QuarantineRecord) error
}

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

func ExponentialBackoff(attempt int, initial, max time.Duration) time.Duration {
	if attempt <= 0 {
		return initial
	}
	backoff := initial
	for i := 1; i < attempt; i++ {
		if backoff >= max/2 {
			return max
		}
		backoff *= 2
	}
	if backoff > max {
		return max
	}
	return backoff
}
