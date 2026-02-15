package schema

import (
	"context"
	"errors"
	"testing"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

type fakeRegistry struct {
	resolveFn         func(subject, version string, headers map[string]string) (*Descriptor, error)
	validatePayloadFn func(desc *Descriptor, payload []byte) error
	validateHeadersFn func(desc *Descriptor, headers map[string]string) error
}

func (f *fakeRegistry) Resolve(subject, version string, headers map[string]string) (*Descriptor, error) {
	return f.resolveFn(subject, version, headers)
}
func (f *fakeRegistry) ValidatePayload(desc *Descriptor, payload []byte) error {
	return f.validatePayloadFn(desc, payload)
}
func (f *fakeRegistry) ValidateHeaders(desc *Descriptor, headers map[string]string) error {
	return f.validateHeadersFn(desc, headers)
}

func TestNormalizeMode(t *testing.T) {
	if got := normalizeMode("warn"); got != ValidationModeWarn {
		t.Fatalf("expected warn, got %s", got)
	}
	if got := normalizeMode("unexpected"); got != ValidationModeEnforce {
		t.Fatalf("expected enforce fallback, got %s", got)
	}
}

func TestProducerValidator_EnrichesHeadersOnSuccess(t *testing.T) {
	reg := &fakeRegistry{
		resolveFn: func(subject, version string, headers map[string]string) (*Descriptor, error) {
			if subject != "orders.created" || version != "v2" {
				t.Fatalf("unexpected resolve args: %s %s", subject, version)
			}
			return &Descriptor{
				Subject:    "orders.created",
				Version:    "v2",
				SchemaID:   "id1",
				SchemaHash: "hash1",
				Message:    "orders.Created",
			}, nil
		},
		validateHeadersFn: func(*Descriptor, map[string]string) error { return nil },
		validatePayloadFn: func(*Descriptor, []byte) error { return nil },
	}
	v := NewMessageProducerValidator(reg, ValidationModeEnforce)
	msg := &eventbus.Message{
		Value: []byte(`{}`),
		Headers: map[string]string{
			"schema_subject": "orders.created",
			"schema_version": "v2",
		},
	}

	if err := v.ValidateBeforePublish(context.Background(), "ignored", msg); err != nil {
		t.Fatalf("validate before publish: %v", err)
	}
	if msg.Headers["schema_id"] != "id1" || msg.Headers["schema_hash"] != "hash1" || msg.Headers["schema_message"] != "orders.Created" {
		t.Fatalf("expected enriched headers, got %+v", msg.Headers)
	}
}

func TestProducerValidator_WarnModeSuppressesError(t *testing.T) {
	reg := &fakeRegistry{
		resolveFn: func(string, string, map[string]string) (*Descriptor, error) {
			return nil, errors.New("boom")
		},
		validateHeadersFn: func(*Descriptor, map[string]string) error { return nil },
		validatePayloadFn: func(*Descriptor, []byte) error { return nil },
	}
	v := NewMessageProducerValidator(reg, ValidationModeWarn)
	msg := &eventbus.Message{Value: []byte("x")}

	if err := v.ValidateBeforePublish(context.Background(), "orders.created", msg); err != nil {
		t.Fatalf("expected warn mode to suppress error, got %v", err)
	}
	if msg.Headers["schema_validation_warn"] == "" {
		t.Fatalf("expected warning header, got %+v", msg.Headers)
	}
}

func TestConsumerValidator_EnforceAndWarnModes(t *testing.T) {
	failReg := &fakeRegistry{
		resolveFn: func(string, string, map[string]string) (*Descriptor, error) {
			return nil, errors.New("resolve failed")
		},
		validateHeadersFn: func(*Descriptor, map[string]string) error { return nil },
		validatePayloadFn: func(*Descriptor, []byte) error { return nil },
	}

	enforce := NewMessageConsumerValidator(failReg, ValidationModeEnforce)
	msg := &eventbus.Message{Value: []byte("x")}
	if err := enforce.ValidateAfterConsume(context.Background(), "orders.created", msg); err == nil {
		t.Fatal("expected error in enforce mode")
	}

	warn := NewMessageConsumerValidator(failReg, ValidationModeWarn)
	msg2 := &eventbus.Message{Value: []byte("x")}
	if err := warn.ValidateAfterConsume(context.Background(), "orders.created", msg2); err != nil {
		t.Fatalf("expected warn mode to suppress error, got %v", err)
	}
	if msg2.Headers["schema_validation_warn"] == "" {
		t.Fatalf("expected warning header, got %+v", msg2.Headers)
	}
}
