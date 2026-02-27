package eventbus

import (
	"context"
	"errors"
	"fmt"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	// AggregateVersionColumnSQL is the SQL column definition to add optimistic aggregate versioning.
	AggregateVersionColumnSQL = "aggregate_version BIGINT NOT NULL DEFAULT 0"
)

// VersionStatus describes ordering quality for a received event version.
type VersionStatus string

// Version status constants
const (
	// VersionStatusOK indicates the event version is in correct order
	VersionStatusOK VersionStatus = "ok"
	// VersionStatusDuplicate indicates the event version was already processed
	VersionStatusDuplicate VersionStatus = "duplicate"
	// VersionStatusOutOfOrder indicates the event arrived out of sequence
	VersionStatusOutOfOrder VersionStatus = "out_of_order"
	// VersionStatusGap indicates there's a gap in the version sequence
	VersionStatusGap VersionStatus = "gap"
)

// VersionCheck holds the result of validating a received event version.
type VersionCheck struct {
	Status   VersionStatus
	Expected int64
	Received int64
	GapSize  int64
}

// NextAggregateVersion increments an aggregate version for producer-side event emission.
func NextAggregateVersion(current int64) (int64, error) {
	if current < 0 {
		return 0, errors.New("current version cannot be negative")
	}
	return current + 1, nil
}

// ValidateAggregateVersion compares received version against the expected next version.
func ValidateAggregateVersion(lastSeenVersion, receivedVersion int64) VersionCheck {
	expected := lastSeenVersion + 1

	switch {
	case receivedVersion == expected:
		return VersionCheck{Status: VersionStatusOK, Expected: expected, Received: receivedVersion}
	case receivedVersion == lastSeenVersion:
		return VersionCheck{Status: VersionStatusDuplicate, Expected: expected, Received: receivedVersion}
	case receivedVersion < lastSeenVersion:
		return VersionCheck{Status: VersionStatusOutOfOrder, Expected: expected, Received: receivedVersion}
	default:
		return VersionCheck{Status: VersionStatusGap, Expected: expected, Received: receivedVersion, GapSize: receivedVersion - expected}
	}
}

// AggregateVersionStore persists consumer-side last-seen versions.
type AggregateVersionStore interface {
	GetLastSeenVersion(ctx context.Context, tenantID, aggregateID string) (int64, error)
	SetLastSeenVersion(ctx context.Context, tenantID, aggregateID string, version int64) error
}

// AggregateVersionTracker validates ordering and advances last-seen version on successful sequence.
type AggregateVersionTracker struct {
	store  AggregateVersionStore
	logger logger.Logger
}

// NewAggregateVersionTracker constructs a tracker using an external store.
func NewAggregateVersionTracker(store AggregateVersionStore, log logger.Logger) (*AggregateVersionTracker, error) {
	if store == nil {
		return nil, errors.New("aggregate version store is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	return &AggregateVersionTracker{store: store, logger: log}, nil
}

// CheckAndAdvance validates a version and persists it when sequence is valid.
func (t *AggregateVersionTracker) CheckAndAdvance(ctx context.Context, tenantID, aggregateID string, receivedVersion int64) (VersionCheck, error) {
	if t == nil || t.store == nil {
		return VersionCheck{}, errors.New("aggregate version tracker is not initialized")
	}
	if tenantID == "" {
		return VersionCheck{}, errors.New("tenant id is required")
	}
	if aggregateID == "" {
		return VersionCheck{}, errors.New("aggregate id is required")
	}
	if receivedVersion < 1 {
		return VersionCheck{}, errors.New("received version must be >= 1")
	}

	lastSeen, err := t.store.GetLastSeenVersion(ctx, tenantID, aggregateID)
	if err != nil {
		return VersionCheck{}, fmt.Errorf("load last seen version failed: %w", err)
	}

	check := ValidateAggregateVersion(lastSeen, receivedVersion)
	if check.Status != VersionStatusOK {
		t.logger.Warn("aggregate version anomaly detected",
			"tenant_id", tenantID,
			"aggregate_id", aggregateID,
			"status", check.Status,
			"expected", check.Expected,
			"received", check.Received,
			"gap_size", check.GapSize,
		)
		return check, nil
	}

	if err := t.store.SetLastSeenVersion(ctx, tenantID, aggregateID, receivedVersion); err != nil {
		return VersionCheck{}, fmt.Errorf("persist last seen version failed: %w", err)
	}

	return check, nil
}
