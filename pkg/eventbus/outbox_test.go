package eventbus

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

type fakeOutboxStore struct {
	mu      sync.Mutex
	entries map[string]*OutboxEntry
}

func newFakeOutboxStore(entries ...*OutboxEntry) *fakeOutboxStore {
	m := make(map[string]*OutboxEntry, len(entries))
	for _, e := range entries {
		cp := *e
		m[e.ID] = &cp
	}
	return &fakeOutboxStore{entries: m}
}

func (s *fakeOutboxStore) Insert(_ context.Context, entry *OutboxEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[entry.ID]; exists {
		return errors.New("duplicate id")
	}
	cp := *entry
	s.entries[entry.ID] = &cp
	return nil
}

func (s *fakeOutboxStore) FetchPending(_ context.Context, limit int, now time.Time) ([]*OutboxEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := make([]*OutboxEntry, 0)
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

func (s *fakeOutboxStore) MarkPublished(_ context.Context, id string, publishedAt time.Time) error {
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

func (s *fakeOutboxStore) MarkFailed(_ context.Context, id string, retryCount int, nextAttemptAt time.Time, reason string) error {
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

func (s *fakeOutboxStore) CleanupPublishedBefore(_ context.Context, before time.Time, limit int) (int, error) {
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

func (s *fakeOutboxStore) PendingCount(_ context.Context, now time.Time) (int, error) {
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

func (s *fakeOutboxStore) OldestPendingAgeSeconds(_ context.Context, now time.Time) (float64, error) {
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

type fakeProducer struct {
	mu        sync.Mutex
	topics    []string
	messages  []*Message
	failByKey map[string]int
}

func (p *fakeProducer) Publish(_ context.Context, topic string, message *Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failByKey != nil {
		if remaining, ok := p.failByKey[message.Key]; ok && remaining > 0 {
			p.failByKey[message.Key] = remaining - 1
			return errors.New("simulated publish failure")
		}
	}
	p.topics = append(p.topics, topic)
	p.messages = append(p.messages, message)
	return nil
}

func (p *fakeProducer) PublishBatch(ctx context.Context, topic string, messages []*Message) error {
	for _, msg := range messages {
		if err := p.Publish(ctx, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

func (p *fakeProducer) Close() error { return nil }

func TestOutboxPublisher_PublishSuccess(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeOutboxStore(&OutboxEntry{
		ID:          "1",
		Topic:       "events.users",
		Message:     &Message{ID: "msg_1", Key: "k1", Value: []byte("ok")},
		CreatedAt:   now.Add(-time.Minute),
		AvailableAt: now.Add(-time.Second),
	})
	producer := &fakeProducer{}
	publisher, err := NewOutboxPublisher(store, producer, testLogger{}, nil, OutboxPublisherConfig{PollInterval: time.Millisecond, BatchSize: 10})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := publisher.tick(context.Background(), now); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	e := store.entries["1"]
	if !e.Published {
		t.Fatalf("expected entry to be published")
	}
	if e.PublishedAt == nil {
		t.Fatalf("expected published_at to be set")
	}
}

func TestOutboxPublisher_PublishFailureSchedulesRetry(t *testing.T) {
	now := time.Now().UTC()
	store := newFakeOutboxStore(&OutboxEntry{
		ID:          "1",
		Topic:       "events.users",
		Message:     &Message{ID: "msg_1", Key: "k1", Value: []byte("ok")},
		CreatedAt:   now.Add(-time.Minute),
		AvailableAt: now.Add(-time.Second),
	})
	producer := &fakeProducer{failByKey: map[string]int{"k1": 1}}
	publisher, err := NewOutboxPublisher(store, producer, testLogger{}, nil, OutboxPublisherConfig{
		PollInterval:   time.Millisecond,
		BatchSize:      10,
		InitialBackoff: time.Second,
		MaxBackoff:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	if err := publisher.tick(context.Background(), now); err != nil {
		t.Fatalf("tick failed: %v", err)
	}

	e := store.entries["1"]
	if e.Published {
		t.Fatalf("entry should not be marked as published")
	}
	if e.RetryCount != 1 {
		t.Fatalf("expected retry_count=1, got %d", e.RetryCount)
	}
	if !e.AvailableAt.After(now) {
		t.Fatalf("expected next available_at in the future")
	}
	if e.LastError == "" {
		t.Fatalf("expected last_error to be set")
	}
}

func TestExponentialBackoff_CapsAtMax(t *testing.T) {
	initial := time.Second
	max := 5 * time.Second
	if got := exponentialBackoff(10, initial, max); got != max {
		t.Fatalf("expected backoff to cap at %v, got %v", max, got)
	}
}
