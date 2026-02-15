package sse

import (
	"fmt"
	"sync/atomic"
	"time"
)

var eventCounter uint64

// Event is the canonical SSE message used by manager, stores, and buses.
type Event struct {
	ID        string    `json:"id"`
	Channel   string    `json:"channel"`
	TenantID  string    `json:"tenant_id,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Type      string    `json:"type,omitempty"`
	Data      []byte    `json:"data"`
	RetryMS   int       `json:"retry_ms,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *Event) normalize(now time.Time) {
	if e.Timestamp.IsZero() {
		e.Timestamp = now.UTC()
	}
	if e.ID == "" {
		e.ID = nextEventID(e.Timestamp)
	}
}

func nextEventID(now time.Time) string {
	seq := atomic.AddUint64(&eventCounter, 1)
	return fmt.Sprintf("%013d-%010d", now.UTC().UnixMilli(), seq)
}
