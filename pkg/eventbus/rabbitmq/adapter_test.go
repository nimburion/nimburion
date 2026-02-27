package rabbitmq

import (
	"context"
	"testing"
	"time"

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

func TestNewAdapter_Validation(t *testing.T) {
	_, err := NewAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected validation error for empty URL")
	}
}

func TestClosedAdapterOperations(t *testing.T) {
	a := &Adapter{closed: true, subs: map[string]*subscription{}}
	msg := &eventbus.Message{ID: "1", Value: []byte("v"), Timestamp: time.Now()}

	if err := a.Publish(context.Background(), "topic", msg); err == nil {
		t.Fatal("publish must fail when closed")
	}
	if err := a.PublishBatch(context.Background(), "topic", []*eventbus.Message{msg}); err == nil {
		t.Fatal("publish batch must fail when closed")
	}
	if err := a.Subscribe(context.Background(), "topic", func(context.Context, *eventbus.Message) error { return nil }); err == nil {
		t.Fatal("subscribe must fail when closed")
	}
	if err := a.HealthCheck(context.Background()); err == nil {
		t.Fatal("healthcheck must fail when closed")
	}
}

func TestUnsubscribe_NotSubscribed(t *testing.T) {
	a := &Adapter{subs: map[string]*subscription{}}
	if err := a.Unsubscribe("missing"); err == nil {
		t.Fatal("expected error for missing subscription")
	}
}

func TestHeadersConversion(t *testing.T) {
	in := map[string]string{"k1": "v1", "k2": "v2"}
	amqpHeaders := toAMQPHeaders(in)
	out := fromAMQPHeaders(amqpHeaders)
	if len(out) != 2 || out["k1"] != "v1" || out["k2"] != "v2" {
		t.Fatalf("unexpected conversion output: %#v", out)
	}
}

func TestHeadersConversionEmpty(t *testing.T) {
	amqpHeaders := toAMQPHeaders(nil)
	if len(amqpHeaders) != 0 {
		t.Fatalf("expected empty headers, got %v", amqpHeaders)
	}

	out := fromAMQPHeaders(nil)
	if len(out) != 0 {
		t.Fatalf("expected empty map, got %v", out)
	}
}

func TestHeadersConversionNonString(t *testing.T) {
	amqpHeaders := map[string]interface{}{"k1": "v1", "k2": 123, "k3": true}
	out := fromAMQPHeaders(amqpHeaders)
	if out["k1"] != "v1" {
		t.Fatalf("expected k1=v1, got %v", out["k1"])
	}
	// Non-string values are converted to strings via fmt.Sprintf
	if out["k2"] != "123" {
		t.Fatalf("expected k2=123, got %v", out["k2"])
	}
	if out["k3"] != "true" {
		t.Fatalf("expected k3=true, got %v", out["k3"])
	}
}
