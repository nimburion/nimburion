package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Class identifies the audit domain for a record.
type Class string

const (
	// ClassOperational marks records emitted for operational events.
	ClassOperational Class = "operational"
	// ClassBusiness marks records emitted for business events.
	ClassBusiness Class = "business"
	// ClassSecurity marks records emitted for security events.
	ClassSecurity Class = "security"
)

// Requirement defines the delivery guarantee expected for a record.
type Requirement string

const (
	// RequirementBestEffort marks records that may be dropped under failure.
	RequirementBestEffort Requirement = "best_effort"
	// RequirementRequired marks records that must be delivered.
	RequirementRequired Requirement = "required"
)

// Record is a single chained audit event within a stream.
type Record struct {
	StreamID    string                 `json:"stream_id"`
	Sequence    uint64                 `json:"sequence"`
	Class       Class                  `json:"class"`
	Requirement Requirement            `json:"requirement"`
	Action      string                 `json:"action"`
	Actor       string                 `json:"actor,omitempty"`
	TenantID    string                 `json:"tenant_id,omitempty"`
	OccurredAt  time.Time              `json:"occurred_at"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	PrevDigest  string                 `json:"prev_digest,omitempty"`
	Digest      string                 `json:"digest,omitempty"`
}

// Validate checks that the record contains the minimum required fields.
func (r *Record) Validate() error {
	if r == nil {
		return errors.New("audit record is nil")
	}
	if r.StreamID == "" {
		return errors.New("audit stream_id is required")
	}
	if r.Sequence == 0 {
		return errors.New("audit sequence must be > 0")
	}
	if r.Action == "" {
		return errors.New("audit action is required")
	}
	if r.OccurredAt.IsZero() {
		return errors.New("audit occurred_at is required")
	}
	if r.Class == "" {
		return errors.New("audit class is required")
	}
	if r.Requirement == "" {
		return errors.New("audit requirement is required")
	}
	return nil
}

// CanonicalDigest computes the canonical digest for the record without Digest set.
func (r *Record) CanonicalDigest() (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	clone := *r
	clone.Digest = ""
	payload, err := json.Marshal(clone)
	if err != nil {
		return "", fmt.Errorf("marshal audit record: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// Seal computes and stores the record digest.
func (r *Record) Seal() error {
	digest, err := r.CanonicalDigest()
	if err != nil {
		return err
	}
	r.Digest = digest
	return nil
}

// Sink persists audit records.
type Sink interface {
	Write(context.Context, *Record) error
}

// Writer validates, seals, and forwards audit records to a sink.
type Writer struct {
	sink Sink
}

// NewWriter constructs a Writer backed by sink.
func NewWriter(sink Sink) (*Writer, error) {
	if sink == nil {
		return nil, errors.New("audit sink is required")
	}
	return &Writer{sink: sink}, nil
}

func (w *Writer) Write(ctx context.Context, record *Record) error {
	if w == nil || w.sink == nil {
		return errors.New("audit writer is not initialized")
	}
	if record == nil {
		return errors.New("audit record is required")
	}
	if err := record.Seal(); err != nil {
		return err
	}
	return w.sink.Write(ctx, record)
}

// VerifyChain validates sequence and digest continuity for records grouped by stream.
func VerifyChain(records []*Record) error {
	perStream := map[string]uint64{}
	prevDigest := map[string]string{}
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return err
		}
		expectedSeq := perStream[record.StreamID] + 1
		if record.Sequence != expectedSeq {
			return fmt.Errorf("audit sequence mismatch for stream %s: got %d want %d", record.StreamID, record.Sequence, expectedSeq)
		}
		if record.PrevDigest != prevDigest[record.StreamID] {
			return fmt.Errorf("audit prev_digest mismatch for stream %s", record.StreamID)
		}
		digest, err := record.CanonicalDigest()
		if err != nil {
			return err
		}
		if record.Digest != digest {
			return fmt.Errorf("audit digest mismatch for stream %s sequence %d", record.StreamID, record.Sequence)
		}
		perStream[record.StreamID] = record.Sequence
		prevDigest[record.StreamID] = record.Digest
	}
	return nil
}
