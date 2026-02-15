package tracing

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTestTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(provider)
	
	return spanRecorder
}

func TestStartDatabaseSpan(t *testing.T) {
	recorder := setupTestTracer(t)
	ctx := context.Background()
	
	tests := []struct {
		name           string
		operation      SpanOperation
		opts           []DatabaseSpanOption
		expectedName   string
		expectedAttrs  map[string]interface{}
	}{
		{
			name:         "query without options",
			operation:    SpanOperationDBQuery,
			opts:         nil,
			expectedName: "DB db.query",
			expectedAttrs: map[string]interface{}{
				"db.operation": "db.query",
			},
		},
		{
			name:      "query with table",
			operation: SpanOperationDBQuery,
			opts: []DatabaseSpanOption{
				WithDBTable("users"),
			},
			expectedName: "DB db.query users",
			expectedAttrs: map[string]interface{}{
				"db.operation": "db.query",
				"db.table":     "users",
			},
		},
		{
			name:      "insert with all options",
			operation: SpanOperationDBInsert,
			opts: []DatabaseSpanOption{
				WithDBTable("orders"),
				WithDBSystem("postgresql"),
				WithDBStatement("INSERT INTO orders (id, total) VALUES ($1, $2)"),
				WithDBName("mydb"),
			},
			expectedName: "DB db.insert orders",
			expectedAttrs: map[string]interface{}{
				"db.operation": "db.insert",
				"db.table":     "orders",
				"db.system":    "postgresql",
				"db.statement": "INSERT INTO orders (id, total) VALUES ($1, $2)",
				"db.name":      "mydb",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.Reset()
			
			_, span := StartDatabaseSpan(ctx, tt.operation, tt.opts...)
			if span == nil {
				t.Fatal("expected span to be non-nil")
			}
			span.End()
			
			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("expected 1 span, got %d", len(spans))
			}
			
			recordedSpan := spans[0]
			if recordedSpan.Name() != tt.expectedName {
				t.Errorf("expected span name %q, got %q", tt.expectedName, recordedSpan.Name())
			}
			
			// Check attributes
			attrs := recordedSpan.Attributes()
			for key, expectedValue := range tt.expectedAttrs {
				found := false
				for _, attr := range attrs {
					if string(attr.Key) == key {
						found = true
						if attr.Value.AsInterface() != expectedValue {
							t.Errorf("expected attribute %s=%v, got %v", key, expectedValue, attr.Value.AsInterface())
						}
						break
					}
				}
				if !found {
					t.Errorf("expected attribute %s not found", key)
				}
			}
		})
	}
}

func TestStartMessagingSpan(t *testing.T) {
	recorder := setupTestTracer(t)
	ctx := context.Background()
	
	tests := []struct {
		name           string
		operation      SpanOperation
		opts           []MessagingSpanOption
		expectedName   string
		expectedAttrs  map[string]interface{}
	}{
		{
			name:         "publish without options",
			operation:    SpanOperationMsgPublish,
			opts:         nil,
			expectedName: "MSG messaging.publish",
			expectedAttrs: map[string]interface{}{
				"messaging.operation": "messaging.publish",
			},
		},
		{
			name:      "publish with destination",
			operation: SpanOperationMsgPublish,
			opts: []MessagingSpanOption{
				WithMessagingDestination("orders.created"),
			},
			expectedName: "MSG messaging.publish orders.created",
			expectedAttrs: map[string]interface{}{
				"messaging.operation":    "messaging.publish",
				"messaging.destination":  "orders.created",
			},
		},
		{
			name:      "consume with all options",
			operation: SpanOperationMsgConsume,
			opts: []MessagingSpanOption{
				WithMessagingSystem("kafka"),
				WithMessagingDestination("user.events"),
				WithMessagingMessageID("msg-123"),
				WithMessagingPayloadSize(1024),
			},
			expectedName: "MSG messaging.consume user.events",
			expectedAttrs: map[string]interface{}{
				"messaging.operation":          "messaging.consume",
				"messaging.system":             "kafka",
				"messaging.destination":        "user.events",
				"messaging.message_id":         "msg-123",
				"messaging.payload_size_bytes": int64(1024),
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.Reset()
			
			_, span := StartMessagingSpan(ctx, tt.operation, tt.opts...)
			if span == nil {
				t.Fatal("expected span to be non-nil")
			}
			span.End()
			
			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("expected 1 span, got %d", len(spans))
			}
			
			recordedSpan := spans[0]
			if recordedSpan.Name() != tt.expectedName {
				t.Errorf("expected span name %q, got %q", tt.expectedName, recordedSpan.Name())
			}
			
			// Check attributes
			attrs := recordedSpan.Attributes()
			for key, expectedValue := range tt.expectedAttrs {
				found := false
				for _, attr := range attrs {
					if string(attr.Key) == key {
						found = true
						if attr.Value.AsInterface() != expectedValue {
							t.Errorf("expected attribute %s=%v, got %v", key, expectedValue, attr.Value.AsInterface())
						}
						break
					}
				}
				if !found {
					t.Errorf("expected attribute %s not found", key)
				}
			}
		})
	}
}

func TestStartCacheSpan(t *testing.T) {
	recorder := setupTestTracer(t)
	ctx := context.Background()
	
	tests := []struct {
		name           string
		operation      SpanOperation
		opts           []CacheSpanOption
		expectedName   string
		expectedAttrs  map[string]interface{}
	}{
		{
			name:         "get without options",
			operation:    SpanOperationCacheGet,
			opts:         nil,
			expectedName: "CACHE cache.get",
			expectedAttrs: map[string]interface{}{
				"cache.operation": "cache.get",
			},
		},
		{
			name:      "get with key",
			operation: SpanOperationCacheGet,
			opts: []CacheSpanOption{
				WithCacheKey("user:123"),
			},
			expectedName: "CACHE cache.get user:123",
			expectedAttrs: map[string]interface{}{
				"cache.operation": "cache.get",
				"cache.key":       "user:123",
			},
		},
		{
			name:      "set with all options",
			operation: SpanOperationCacheSet,
			opts: []CacheSpanOption{
				WithCacheSystem("redis"),
				WithCacheKey("session:abc"),
				WithCacheHit(true),
			},
			expectedName: "CACHE cache.set session:abc",
			expectedAttrs: map[string]interface{}{
				"cache.operation": "cache.set",
				"cache.system":    "redis",
				"cache.key":       "session:abc",
				"cache.hit":       true,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.Reset()
			
			_, span := StartCacheSpan(ctx, tt.operation, tt.opts...)
			if span == nil {
				t.Fatal("expected span to be non-nil")
			}
			span.End()
			
			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("expected 1 span, got %d", len(spans))
			}
			
			recordedSpan := spans[0]
			if recordedSpan.Name() != tt.expectedName {
				t.Errorf("expected span name %q, got %q", tt.expectedName, recordedSpan.Name())
			}
			
			// Check attributes
			attrs := recordedSpan.Attributes()
			for key, expectedValue := range tt.expectedAttrs {
				found := false
				for _, attr := range attrs {
					if string(attr.Key) == key {
						found = true
						if attr.Value.AsInterface() != expectedValue {
							t.Errorf("expected attribute %s=%v, got %v", key, expectedValue, attr.Value.AsInterface())
						}
						break
					}
				}
				if !found {
					t.Errorf("expected attribute %s not found", key)
				}
			}
		})
	}
}

func TestRecordError(t *testing.T) {
	recorder := setupTestTracer(t)
	ctx := context.Background()
	
	tracer := otel.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	
	testErr := errors.New("test error")
	RecordError(span, testErr)
	span.End()
	
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	
	recordedSpan := spans[0]
	
	// Check that error was recorded
	events := recordedSpan.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event (error), got %d", len(events))
	}
	
	if events[0].Name != "exception" {
		t.Errorf("expected event name 'exception', got %q", events[0].Name)
	}
	
	// Check span status
	if recordedSpan.Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", recordedSpan.Status().Code)
	}
	
	if recordedSpan.Status().Description != testErr.Error() {
		t.Errorf("expected span status description %q, got %q", testErr.Error(), recordedSpan.Status().Description)
	}
}

func TestRecordSuccess(t *testing.T) {
	recorder := setupTestTracer(t)
	ctx := context.Background()
	
	tracer := otel.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	
	RecordSuccess(span)
	span.End()
	
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	
	recordedSpan := spans[0]
	
	// Check span status
	if recordedSpan.Status().Code != codes.Ok {
		t.Errorf("expected span status Ok, got %v", recordedSpan.Status().Code)
	}
}
