package jobs

import (
	"context"
	"time"
)

// DLQEntry represents one dead-letter record.
type DLQEntry struct {
	ID            string
	Queue         string
	OriginalQueue string
	Job           *Job
	Reason        string
	FailedAt      time.Time
}

// DLQStore exposes listing and replay operations for dead-letter queues.
type DLQStore interface {
	ListDLQ(ctx context.Context, queue string, limit int) ([]*DLQEntry, error)
	ReplayDLQ(ctx context.Context, queue string, ids []string) (int, error)
}
