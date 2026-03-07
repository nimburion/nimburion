package outbox

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	logpkg "github.com/nimburion/nimburion/pkg/observability/logger"
)

type testLogger struct{}

func (testLogger) Debug(string, ...any)                      {}
func (testLogger) Info(string, ...any)                       {}
func (testLogger) Warn(string, ...any)                       {}
func (testLogger) Error(string, ...any)                      {}
func (testLogger) With(...any) logpkg.Logger                 { return testLogger{} }
func (testLogger) WithContext(context.Context) logpkg.Logger { return testLogger{} }

type fakeStore struct {
	mu      sync.Mutex
	entries map[string]*Entry
}

func newFakeStore(entries ...*Entry) *fakeStore {
	m := make(map[string]*Entry, len(entries))
	for _, e := range entries {
		cp := *e
		m[e.ID] = &cp
	}
	return &fakeStore{entries: m}
}

func (s *fakeStore) Insert(_ context.Context, entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[entry.ID]; exists {
		return errors.New("duplicate id")
	}
	cp := *entry
	s.entries[entry.ID] = &cp
	return nil
}

func (s *fakeStore) FetchPending(_ context.Context, limit int, now time.Time) ([]*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := make([]*Entry, 0)
	for _, e := range s.entries {
		if !e.Published && !e.AvailableAt.After(now) {
			cp := *e
			pending = append(pending, &cp)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].CreatedAt.Before(pending[j].CreatedAt) })
	if len(pending) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}

func (s *fakeStore) MarkPublished(_ context.Context, id string, publishedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return errors.New("not found")
	}
	e.Published = true
	e.PublishedAt = &publishedAt
	return nil
}

func (s *fakeStore) MarkFailed(_ context.Context, id string, retryCount int, nextAttemptAt time.Time, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[id]
	if !ok {
		return errors.New("not found")
	}
	e.RetryCount = retryCount
	e.AvailableAt = nextAttemptAt
	e.LastError = reason
	return nil
}

func (s *fakeStore) CleanupPublishedBefore(_ context.Context, before time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for id, e := range s.entries {
		if deleted >= limit {
			break
		}
		if e.PublishedAt != nil && e.PublishedAt.Before(before) {
			delete(s.entries, id)
			deleted++
		}
	}
	return deleted, nil
}

func (s *fakeStore) PendingCount(_ context.Context, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, e := range s.entries {
		if !e.Published && !e.AvailableAt.After(now) {
			count++
		}
	}
	return count, nil
}

func (s *fakeStore) OldestPendingAgeSeconds(_ context.Context, now time.Time) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var oldest *time.Time
	for _, e := range s.entries {
		if e.Published || e.AvailableAt.After(now) {
			continue
		}
		if oldest == nil || e.CreatedAt.Before(*oldest) {
			ts := e.CreatedAt
			oldest = &ts
		}
	}
	if oldest == nil {
		return 0, nil
	}
	return now.Sub(*oldest).Seconds(), nil
}

type fakePublisher struct {
	mu        sync.Mutex
	topics    []string
	records   []*Record
	failByKey map[string]int
}

func (p *fakePublisher) Publish(_ context.Context, topic string, record *Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failByKey != nil {
		if remaining, ok := p.failByKey[record.Key]; ok && remaining > 0 {
			p.failByKey[record.Key] = remaining - 1
			return errors.New("simulated publish failure")
		}
	}
	p.topics = append(p.topics, topic)
	p.records = append(p.records, record)
	return nil
}

func TestRelay_PublishSuccess(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeStore(&Entry{
		ID:          "1",
		Topic:       "events.users",
		Record:      &Record{ID: "msg_1", Key: "k1", Payload: []byte("ok")},
		CreatedAt:   now.Add(-time.Minute),
		AvailableAt: now.Add(-time.Second),
	})
	publisher := &fakePublisher{}
	relay, err := NewRelay(store, publisher, testLogger{}, nil, PublisherConfig{PollInterval: time.Millisecond, BatchSize: 10})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	if err := relay.tick(context.Background(), now); err != nil {
		t.Fatalf("tick failed: %v", err)
	}
	if !store.entries["1"].Published {
		t.Fatal("expected entry to be published")
	}
}

func TestRelay_PublishFailureSchedulesRetry(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeStore(&Entry{
		ID:          "1",
		Topic:       "events.users",
		Record:      &Record{ID: "msg_1", Key: "k1", Payload: []byte("ok")},
		CreatedAt:   now.Add(-time.Minute),
		AvailableAt: now.Add(-time.Second),
	})
	publisher := &fakePublisher{failByKey: map[string]int{"k1": 1}}
	relay, err := NewRelay(store, publisher, testLogger{}, nil, PublisherConfig{
		PollInterval:   time.Millisecond,
		BatchSize:      10,
		InitialBackoff: time.Second,
		MaxBackoff:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	if err := relay.tick(context.Background(), now); err != nil {
		t.Fatalf("tick failed: %v", err)
	}
	e := store.entries["1"]
	if e.Published {
		t.Fatal("entry should not be marked as published")
	}
	if e.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", e.RetryCount)
	}
}

func TestExponentialBackoff_CapsAtMax(t *testing.T) {
	if got := exponentialBackoff(10, time.Second, 5*time.Second); got != 5*time.Second {
		t.Fatalf("expected capped backoff, got %v", got)
	}
}
