package audit

import (
	"context"
	"testing"
	"time"
)

type memorySink struct {
	records []*Record
}

func (s *memorySink) Write(_ context.Context, record *Record) error {
	cp := *record
	s.records = append(s.records, &cp)
	return nil
}

func TestWriterAndVerifyChain(t *testing.T) {
	sink := &memorySink{}
	writer, err := NewWriter(sink)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}

	first := &Record{
		StreamID:    "auth",
		Sequence:    1,
		Class:       ClassSecurity,
		Requirement: RequirementRequired,
		Action:      "login.failed",
		OccurredAt:  time.Now().UTC(),
	}
	if err := writer.Write(context.Background(), first); err != nil {
		t.Fatalf("write first: %v", err)
	}

	second := &Record{
		StreamID:    "auth",
		Sequence:    2,
		Class:       ClassSecurity,
		Requirement: RequirementRequired,
		Action:      "login.locked",
		OccurredAt:  time.Now().UTC(),
		PrevDigest:  sink.records[0].Digest,
	}
	if err := writer.Write(context.Background(), second); err != nil {
		t.Fatalf("write second: %v", err)
	}

	if err := VerifyChain(sink.records); err != nil {
		t.Fatalf("verify chain: %v", err)
	}
}
