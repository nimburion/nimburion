package tracing_test

import (
	"context"
	"fmt"
	"log"

	"github.com/nimburion/nimburion/pkg/observability/tracing"
)

// ExampleNewTracerProvider demonstrates how to create and configure a tracer provider.
func ExampleNewTracerProvider() {
	ctx := context.Background()

	// Create tracer provider with configuration
	provider, err := tracing.NewTracerProvider(ctx, tracing.TracerConfig{
		ServiceName:    "example-service",
		ServiceVersion: "1.0.0",
		Environment:    "production",
		Endpoint:       "localhost:4317",
		SampleRate:     0.1, // Sample 10% of traces
		Enabled:        true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Shutdown(ctx)

	// Get a tracer for your component
	tracer := provider.Tracer("example")

	// Create a span
	_, span := tracer.Start(ctx, "example-operation")
	defer span.End()

	fmt.Println("Tracer provider created successfully")
	// Output: Tracer provider created successfully
}

// ExampleStartDatabaseSpan demonstrates how to trace database operations.
func ExampleStartDatabaseSpan() {
	ctx := context.Background()

	// Start a database span
	ctx, span := tracing.StartDatabaseSpan(ctx, tracing.SpanOperationDBQuery,
		tracing.WithDBSystem("postgresql"),
		tracing.WithDBTable("users"),
		tracing.WithDBStatement("SELECT * FROM users WHERE id = $1"),
	)
	defer span.End()

	// Perform database operation here
	// ...

	// Record success
	tracing.RecordSuccess(span)

	fmt.Println("Database operation traced")
	// Output: Database operation traced
}

// ExampleStartMessagingSpan demonstrates how to trace message broker operations.
func ExampleStartMessagingSpan() {
	ctx := context.Background()

	// Start a messaging span for publishing
	ctx, span := tracing.StartMessagingSpan(ctx, tracing.SpanOperationMsgPublish,
		tracing.WithMessagingSystem("kafka"),
		tracing.WithMessagingDestination("user.events"),
		tracing.WithMessagingMessageID("msg-123"),
		tracing.WithMessagingPayloadSize(1024),
	)
	defer span.End()

	// Publish message here
	// ...

	// Record success
	tracing.RecordSuccess(span)

	fmt.Println("Message publish traced")
	// Output: Message publish traced
}

// ExampleStartCacheSpan demonstrates how to trace cache operations.
func ExampleStartCacheSpan() {
	ctx := context.Background()

	// Start a cache span
	ctx, span := tracing.StartCacheSpan(ctx, tracing.SpanOperationCacheGet,
		tracing.WithCacheSystem("redis"),
		tracing.WithCacheKey("user:123"),
	)
	defer span.End()

	// Perform cache operation here
	// ...

	// Record cache hit
	tracing.WithCacheHit(true)
	tracing.RecordSuccess(span)

	fmt.Println("Cache operation traced")
	// Output: Cache operation traced
}

// ExampleRecordError demonstrates how to record errors in spans.
func ExampleRecordError() {
	ctx := context.Background()

	// Create a span
	ctx, span := tracing.StartDatabaseSpan(ctx, tracing.SpanOperationDBQuery,
		tracing.WithDBTable("users"),
	)
	defer span.End()

	// Simulate an error
	err := fmt.Errorf("connection timeout")

	// Record the error
	tracing.RecordError(span, err)

	fmt.Println("Error recorded in span")
	// Output: Error recorded in span
}
