package sqs

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

func TestNewSQSAdapter_Validation(t *testing.T) {
	_, err := NewSQSAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty region and queue URL")
	}
	_, err = NewSQSAdapter(Config{Region: "eu-west-1"}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty queue URL")
	}
}

func TestClosedAdapterOperations(t *testing.T) {
	a := &SQSAdapter{closed: true, subs: map[string]context.CancelFunc{}, logger: &mockLogger{}}
	msg := &eventbus.Message{ID: "1", Value: []byte("v"), Timestamp: time.Now()}

	if err := a.Publish(context.Background(), "", msg); err == nil {
		t.Fatal("publish must fail when closed")
	}
	if err := a.PublishBatch(context.Background(), "", []*eventbus.Message{msg}); err == nil {
		t.Fatal("publish batch must fail when closed")
	}
	if err := a.Subscribe(context.Background(), "x", func(context.Context, *eventbus.Message) error { return nil }); err == nil {
		t.Fatal("subscribe must fail when closed")
	}
	if err := a.HealthCheck(context.Background()); err == nil {
		t.Fatal("health check must fail when closed")
	}
}

func TestResolveQueueURL(t *testing.T) {
	a := &SQSAdapter{config: Config{QueueURL: "default"}}
	if got := a.resolveQueueURL("override"); got != "override" {
		t.Fatalf("expected override queue, got %s", got)
	}
	if got := a.resolveQueueURL(""); got != "default" {
		t.Fatalf("expected default queue, got %s", got)
	}
}

func TestAttributesConversion(t *testing.T) {
	in := map[string]string{"a": "1", "b": "2"}
	attrs := toSQSAttributes(in)
	out := fromSQSAttributes(attrs)
	if len(out) != 2 || out["a"] != "1" || out["b"] != "2" {
		t.Fatalf("unexpected attrs roundtrip: %#v", out)
	}

	decoded := fromSQSAttributes(map[string]types.MessageAttributeValue{
		"x": {DataType: aws.String("String"), StringValue: aws.String("y")},
	})
	if decoded["x"] != "y" {
		t.Fatalf("expected y, got %q", decoded["x"])
	}
}
