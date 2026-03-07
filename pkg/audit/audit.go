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

type Class string

const (
	ClassOperational Class = "operational"
	ClassBusiness    Class = "business"
	ClassSecurity    Class = "security"
)

type Requirement string

const (
	RequirementBestEffort Requirement = "best_effort"
	RequirementRequired   Requirement = "required"
)

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

func (r *Record) Seal() error {
	digest, err := r.CanonicalDigest()
	if err != nil {
		return err
	}
	r.Digest = digest
	return nil
}

type Sink interface {
	Write(context.Context, *Record) error
}

type Writer struct {
	sink Sink
}

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
