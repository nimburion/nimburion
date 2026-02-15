package factory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestWrapWithSchemaValidation_Enforce_PublishValid(t *testing.T) {
	log := testLogger(t)
	bus := &mockEventBus{}
	descriptorPath := writeDescriptorSet(t)

	wrapped, err := wrapWithSchemaValidation(bus, config.KafkaValidationConfig{
		Enabled:        true,
		Mode:           "enforce",
		DescriptorPath: descriptorPath,
	}, log)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	msg := &eventbus.Message{
		Value:   []byte(`{}`),
		Headers: map[string]string{},
	}
	if err := wrapped.Publish(context.Background(), "events.UserCreated", msg); err != nil {
		t.Fatalf("expected publish success, got %v", err)
	}
	if bus.publishCalls != 1 {
		t.Fatalf("expected base publish to be called once, got %d", bus.publishCalls)
	}
	if msg.Headers["schema_id"] == "" || msg.Headers["schema_hash"] == "" {
		t.Fatalf("expected schema headers to be enriched")
	}
}

func TestWrapWithSchemaValidation_Enforce_PublishInvalid(t *testing.T) {
	log := testLogger(t)
	bus := &mockEventBus{}
	descriptorPath := writeDescriptorSet(t)

	wrapped, err := wrapWithSchemaValidation(bus, config.KafkaValidationConfig{
		Enabled:        true,
		Mode:           "enforce",
		DescriptorPath: descriptorPath,
	}, log)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	msg := &eventbus.Message{Value: []byte(`{}`), Headers: map[string]string{}}
	err = wrapped.Publish(context.Background(), "unknown.Subject", msg)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
	if bus.publishCalls != 0 {
		t.Fatalf("expected base publish to be skipped, got %d calls", bus.publishCalls)
	}
}

func TestWrapWithSchemaValidation_Warn_DescriptorLoadFailure(t *testing.T) {
	log := testLogger(t)
	bus := &mockEventBus{}

	wrapped, err := wrapWithSchemaValidation(bus, config.KafkaValidationConfig{
		Enabled:        true,
		Mode:           "warn",
		DescriptorPath: "/path/does/not/exist.pb",
	}, log)
	if err != nil {
		t.Fatalf("expected no error in warn mode, got %v", err)
	}
	if wrapped != bus {
		t.Fatal("expected base bus to be returned unchanged in warn mode")
	}
}

func testLogger(t *testing.T) logger.Logger {
	t.Helper()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("create test logger: %v", err)
	}
	return log
}

func writeDescriptorSet(t *testing.T) string {
	t.Helper()

	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("events.proto"),
				Package: proto.String("events"),
				Syntax:  proto.String("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("UserCreated"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("id"),
								Number: proto.Int32(1),
								Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
				},
			},
		},
	}
	raw, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}

	path := filepath.Join(t.TempDir(), "schema.pb")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write descriptor set: %v", err)
	}
	return path
}

type mockEventBus struct {
	publishCalls int
}

func (m *mockEventBus) Publish(ctx context.Context, topic string, message *eventbus.Message) error {
	m.publishCalls++
	return nil
}

func (m *mockEventBus) PublishBatch(ctx context.Context, topic string, messages []*eventbus.Message) error {
	return nil
}

func (m *mockEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.MessageHandler) error {
	return nil
}

func (m *mockEventBus) Unsubscribe(topic string) error {
	return nil
}

func (m *mockEventBus) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockEventBus) Close() error {
	return nil
}
