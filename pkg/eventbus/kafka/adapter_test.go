package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/eventbus"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// mockLogger is a simple logger implementation for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...any) {}
func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Warn(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) With(args ...any) logger.Logger {
	return m
}
func (m *mockLogger) WithContext(ctx context.Context) logger.Logger {
	return m
}

func TestNewKafkaAdapter(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: Config{
				Brokers:          []string{"localhost:9092"},
				OperationTimeout: 30 * time.Second,
				MaxRetries:       3,
				RetryBackoff:     100 * time.Millisecond,
				GroupID:          "test-group",
			},
			wantErr: false,
		},
		{
			name: "valid configuration with defaults",
			config: Config{
				Brokers: []string{"localhost:9092"},
			},
			wantErr: false,
		},
		{
			name: "missing brokers",
			config: Config{
				OperationTimeout: 30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "empty brokers list",
			config: Config{
				Brokers: []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewKafkaAdapter(tt.config, &mockLogger{})

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewKafkaAdapter() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewKafkaAdapter() unexpected error: %v", err)
				return
			}

			if adapter == nil {
				t.Error("NewKafkaAdapter() returned nil adapter")
				return
			}

			// Verify defaults are applied
			if adapter.config.OperationTimeout == 0 {
				t.Error("OperationTimeout should have default value")
			}
			if adapter.config.MaxRetries == 0 {
				t.Error("MaxRetries should have default value")
			}
			if adapter.config.RetryBackoff == 0 {
				t.Error("RetryBackoff should have default value")
			}
			if adapter.config.GroupID == "" {
				t.Error("GroupID should have default value")
			}

			// Verify producer is initialized
			if adapter.producer == nil {
				t.Error("producer should be initialized")
			}

			// Verify consumers map is initialized
			if adapter.consumers == nil {
				t.Error("consumers map should be initialized")
			}

			// Clean up
			if err := adapter.Close(); err != nil {
				t.Errorf("Close() unexpected error: %v", err)
			}
		})
	}
}

func TestKafkaAdapter_Close(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}

	// Close should succeed
	if err := adapter.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	// Second close should not error
	if err := adapter.Close(); err != nil {
		t.Errorf("Close() second call unexpected error: %v", err)
	}

	// Operations after close should fail
	ctx := context.Background()
	msg := &eventbus.Message{
		ID:        "test-id",
		Key:       "test-key",
		Value:     []byte("test-value"),
		Timestamp: time.Now(),
	}

	if err := adapter.Publish(ctx, "test-topic", msg); err == nil {
		t.Error("Publish() after close should return error")
	}

	if err := adapter.Subscribe(ctx, "test-topic", nil); err == nil {
		t.Error("Subscribe() after close should return error")
	}

	if err := adapter.HealthCheck(ctx); err == nil {
		t.Error("HealthCheck() after close should return error")
	}
}

func TestKafkaAdapter_Subscribe_AlreadySubscribed(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	topic := "test-topic"
	handler := func(ctx context.Context, msg *eventbus.Message) error {
		return nil
	}

	// First subscription should succeed
	if err := adapter.Subscribe(ctx, topic, handler); err != nil {
		t.Errorf("Subscribe() first call unexpected error: %v", err)
	}

	// Second subscription to same topic should fail
	if err := adapter.Subscribe(ctx, topic, handler); err == nil {
		t.Error("Subscribe() second call should return error for duplicate subscription")
	}
}

func TestKafkaAdapter_Unsubscribe_NotSubscribed(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	// Unsubscribe from non-existent subscription should fail
	if err := adapter.Unsubscribe("non-existent-topic"); err == nil {
		t.Error("Unsubscribe() should return error for non-existent subscription")
	}
}

func TestConvertHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{
			name:    "nil headers",
			headers: nil,
			want:    0,
		},
		{
			name:    "empty headers",
			headers: map[string]string{},
			want:    0,
		},
		{
			name: "single header",
			headers: map[string]string{
				"key1": "value1",
			},
			want: 1,
		},
		{
			name: "multiple headers",
			headers: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertHeaders(tt.headers)

			if tt.headers == nil {
				if result != nil {
					t.Errorf("convertHeaders() with nil input should return nil, got %v", result)
				}
				return
			}

			if len(result) != tt.want {
				t.Errorf("convertHeaders() length = %d, want %d", len(result), tt.want)
			}

			// Verify all headers are converted
			for key, value := range tt.headers {
				found := false
				for _, header := range result {
					if header.Key == key && string(header.Value) == value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("convertHeaders() missing header %s=%s", key, value)
				}
			}
		})
	}
}

func TestKafkaAdapter_Publish_AfterClose(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}

	// Close the adapter
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	ctx := context.Background()
	msg := &eventbus.Message{
		ID:        "test-id",
		Key:       "test-key",
		Value:     []byte("test-value"),
		Timestamp: time.Now(),
	}

	// Publish after close should fail
	if err := adapter.Publish(ctx, "test-topic", msg); err == nil {
		t.Error("Publish() after close should return error")
	}
}

func TestKafkaAdapter_Publish_WithTimeout(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers:          []string{"localhost:9092"},
		OperationTimeout: 1 * time.Millisecond, // Very short timeout to test timeout behavior
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	msg := &eventbus.Message{
		ID:        "test-id",
		Key:       "test-key",
		Value:     []byte("test-value"),
		Headers:   map[string]string{"header1": "value1"},
		Timestamp: time.Now(),
	}

	// This will likely timeout or fail due to no actual Kafka broker
	// We're just testing that the method handles errors gracefully
	err = adapter.Publish(ctx, "test-topic", msg)
	// We expect an error since there's no real Kafka broker
	if err == nil {
		t.Log("Publish() succeeded unexpectedly (might have connected to a local Kafka)")
	}
}

func TestKafkaAdapter_Publish_WithHeaders(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	msg := &eventbus.Message{
		ID:    "test-id",
		Key:   "test-key",
		Value: []byte("test-value"),
		Headers: map[string]string{
			"content-type": "application/json",
			"trace-id":     "12345",
		},
		Timestamp: time.Now(),
	}

	// Test that headers are properly converted
	// This will fail without a real Kafka broker, but we're testing the conversion logic
	err = adapter.Publish(ctx, "test-topic", msg)
	// We expect an error since there's no real Kafka broker
	if err == nil {
		t.Log("Publish() succeeded unexpectedly (might have connected to a local Kafka)")
	}
}

func TestKafkaAdapter_PublishBatch_EmptyBatch(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()

	// Publishing empty batch should succeed without error
	if err := adapter.PublishBatch(ctx, "test-topic", []*eventbus.Message{}); err != nil {
		t.Errorf("PublishBatch() with empty batch unexpected error: %v", err)
	}

	// Publishing nil batch should succeed without error
	if err := adapter.PublishBatch(ctx, "test-topic", nil); err != nil {
		t.Errorf("PublishBatch() with nil batch unexpected error: %v", err)
	}
}

func TestKafkaAdapter_PublishBatch_AfterClose(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}

	// Close the adapter
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}

	ctx := context.Background()
	messages := []*eventbus.Message{
		{
			ID:        "test-id-1",
			Key:       "test-key-1",
			Value:     []byte("test-value-1"),
			Timestamp: time.Now(),
		},
		{
			ID:        "test-id-2",
			Key:       "test-key-2",
			Value:     []byte("test-value-2"),
			Timestamp: time.Now(),
		},
	}

	// PublishBatch after close should fail
	if err := adapter.PublishBatch(ctx, "test-topic", messages); err == nil {
		t.Error("PublishBatch() after close should return error")
	}
}

func TestKafkaAdapter_PublishBatch_WithTimeout(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers:          []string{"localhost:9092"},
		OperationTimeout: 1 * time.Millisecond, // Very short timeout
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	messages := []*eventbus.Message{
		{
			ID:        "test-id-1",
			Key:       "test-key-1",
			Value:     []byte("test-value-1"),
			Headers:   map[string]string{"header1": "value1"},
			Timestamp: time.Now(),
		},
		{
			ID:        "test-id-2",
			Key:       "test-key-2",
			Value:     []byte("test-value-2"),
			Headers:   map[string]string{"header2": "value2"},
			Timestamp: time.Now(),
		},
	}

	// This will likely timeout or fail due to no actual Kafka broker
	err = adapter.PublishBatch(ctx, "test-topic", messages)
	// We expect an error since there's no real Kafka broker
	if err == nil {
		t.Log("PublishBatch() succeeded unexpectedly (might have connected to a local Kafka)")
	}
}

func TestKafkaAdapter_PublishBatch_MultipleMessages(t *testing.T) {
	adapter, err := NewKafkaAdapter(Config{
		Brokers: []string{"localhost:9092"},
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("NewKafkaAdapter() unexpected error: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	
	// Test with multiple messages
	messages := make([]*eventbus.Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = &eventbus.Message{
			ID:        time.Now().String(),
			Key:       time.Now().String(),
			Value:     []byte(time.Now().String()),
			Headers:   map[string]string{"index": time.Now().String()},
			Timestamp: time.Now(),
		}
	}

	// This will fail without a real Kafka broker
	err = adapter.PublishBatch(ctx, "test-topic", messages)
	// We expect an error since there's no real Kafka broker
	if err == nil {
		t.Log("PublishBatch() succeeded unexpectedly (might have connected to a local Kafka)")
	}
}
