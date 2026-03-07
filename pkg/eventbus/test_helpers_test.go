package eventbus

import (
	"context"
	"errors"
	"sync"

	logpkg "github.com/nimburion/nimburion/pkg/observability/logger"
)

type testLogger struct{}

func (testLogger) Debug(string, ...any)                      {}
func (testLogger) Info(string, ...any)                       {}
func (testLogger) Warn(string, ...any)                       {}
func (testLogger) Error(string, ...any)                      {}
func (testLogger) With(...any) logpkg.Logger                 { return testLogger{} }
func (testLogger) WithContext(context.Context) logpkg.Logger { return testLogger{} }

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
