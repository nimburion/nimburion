// Package tracing provides OpenTelemetry distributed tracing support for the framework.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SpanOperation represents a traced operation type.
type SpanOperation string

// Span operation constants for different operation types
const (
	// SpanOperationDBQuery represents a database query operation
	SpanOperationDBQuery SpanOperation = "db.query"
	// SpanOperationDBInsert represents a database insert operation
	SpanOperationDBInsert SpanOperation = "db.insert"
	// SpanOperationDBUpdate represents a database update operation
	SpanOperationDBUpdate SpanOperation = "db.update"
	// SpanOperationDBDelete represents a database delete operation
	SpanOperationDBDelete SpanOperation = "db.delete"
	// SpanOperationDBTx represents a database transaction
	SpanOperationDBTx SpanOperation = "db.transaction"

	// SpanOperationMsgPublish represents publishing a message
	SpanOperationMsgPublish SpanOperation = "messaging.publish"
	// SpanOperationMsgConsume represents consuming a message
	SpanOperationMsgConsume SpanOperation = "messaging.consume"
	// SpanOperationMsgProcess represents processing a message
	SpanOperationMsgProcess SpanOperation = "messaging.process"

	// SpanOperationCacheGet represents a cache get operation
	SpanOperationCacheGet SpanOperation = "cache.get"
	// SpanOperationCacheSet represents a cache set operation
	SpanOperationCacheSet SpanOperation = "cache.set"
	// SpanOperationCacheDel represents a cache delete operation
	SpanOperationCacheDel SpanOperation = "cache.delete"
)

// StartDatabaseSpan creates a new span for a database operation.
// It includes database-specific attributes like operation type, table name, and query.
//
// Requirements: 14.4
func StartDatabaseSpan(ctx context.Context, operation SpanOperation, opts ...DatabaseSpanOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("database")
	
	spanOpts := &databaseSpanOptions{
		attributes: []attribute.KeyValue{
			attribute.String("db.operation", string(operation)),
		},
	}
	
	for _, opt := range opts {
		opt(spanOpts)
	}
	
	spanName := fmt.Sprintf("DB %s", operation)
	if spanOpts.table != "" {
		spanName = fmt.Sprintf("DB %s %s", operation, spanOpts.table)
	}
	
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	
	// Add all attributes to span
	span.SetAttributes(spanOpts.attributes...)
	
	return ctx, span
}

// DatabaseSpanOption configures a database span.
type DatabaseSpanOption func(*databaseSpanOptions)

type databaseSpanOptions struct {
	table      string
	attributes []attribute.KeyValue
}

// WithDBTable sets the database table name for the span.
func WithDBTable(table string) DatabaseSpanOption {
	return func(opts *databaseSpanOptions) {
		opts.table = table
		opts.attributes = append(opts.attributes, attribute.String("db.table", table))
	}
}

// WithDBSystem sets the database system (e.g., "postgresql", "mysql").
func WithDBSystem(system string) DatabaseSpanOption {
	return func(opts *databaseSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.String("db.system", system))
	}
}

// WithDBStatement sets the database query statement.
func WithDBStatement(statement string) DatabaseSpanOption {
	return func(opts *databaseSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.String("db.statement", statement))
	}
}

// WithDBName sets the database name.
func WithDBName(name string) DatabaseSpanOption {
	return func(opts *databaseSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.String("db.name", name))
	}
}

// StartMessagingSpan creates a new span for a message broker operation.
// It includes messaging-specific attributes like operation type, topic, and message ID.
//
// Requirements: 14.5
func StartMessagingSpan(ctx context.Context, operation SpanOperation, opts ...MessagingSpanOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("messaging")
	
	spanOpts := &messagingSpanOptions{
		attributes: []attribute.KeyValue{
			attribute.String("messaging.operation", string(operation)),
		},
	}
	
	for _, opt := range opts {
		opt(spanOpts)
	}
	
	spanName := fmt.Sprintf("MSG %s", operation)
	if spanOpts.destination != "" {
		spanName = fmt.Sprintf("MSG %s %s", operation, spanOpts.destination)
	}
	
	// Determine span kind based on operation
	spanKind := trace.SpanKindClient
	if operation == SpanOperationMsgConsume || operation == SpanOperationMsgProcess {
		spanKind = trace.SpanKindConsumer
	} else if operation == SpanOperationMsgPublish {
		spanKind = trace.SpanKindProducer
	}
	
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(spanKind))
	
	// Add all attributes to span
	span.SetAttributes(spanOpts.attributes...)
	
	return ctx, span
}

// MessagingSpanOption configures a messaging span.
type MessagingSpanOption func(*messagingSpanOptions)

type messagingSpanOptions struct {
	destination string
	attributes  []attribute.KeyValue
}

// WithMessagingSystem sets the messaging system (e.g., "kafka", "rabbitmq").
func WithMessagingSystem(system string) MessagingSpanOption {
	return func(opts *messagingSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.String("messaging.system", system))
	}
}

// WithMessagingDestination sets the destination (topic, queue) name.
func WithMessagingDestination(destination string) MessagingSpanOption {
	return func(opts *messagingSpanOptions) {
		opts.destination = destination
		opts.attributes = append(opts.attributes, attribute.String("messaging.destination", destination))
	}
}

// WithMessagingMessageID sets the message ID.
func WithMessagingMessageID(messageID string) MessagingSpanOption {
	return func(opts *messagingSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.String("messaging.message_id", messageID))
	}
}

// WithMessagingPayloadSize sets the message payload size in bytes.
func WithMessagingPayloadSize(size int) MessagingSpanOption {
	return func(opts *messagingSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.Int("messaging.payload_size_bytes", size))
	}
}

// StartCacheSpan creates a new span for a cache operation.
// It includes cache-specific attributes like operation type and key.
func StartCacheSpan(ctx context.Context, operation SpanOperation, opts ...CacheSpanOption) (context.Context, trace.Span) {
	tracer := otel.Tracer("cache")
	
	spanOpts := &cacheSpanOptions{
		attributes: []attribute.KeyValue{
			attribute.String("cache.operation", string(operation)),
		},
	}
	
	for _, opt := range opts {
		opt(spanOpts)
	}
	
	spanName := fmt.Sprintf("CACHE %s", operation)
	if spanOpts.key != "" {
		spanName = fmt.Sprintf("CACHE %s %s", operation, spanOpts.key)
	}
	
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	
	// Add all attributes to span
	span.SetAttributes(spanOpts.attributes...)
	
	return ctx, span
}

// CacheSpanOption configures a cache span.
type CacheSpanOption func(*cacheSpanOptions)

type cacheSpanOptions struct {
	key        string
	attributes []attribute.KeyValue
}

// WithCacheSystem sets the cache system (e.g., "redis", "memcached").
func WithCacheSystem(system string) CacheSpanOption {
	return func(opts *cacheSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.String("cache.system", system))
	}
}

// WithCacheKey sets the cache key.
func WithCacheKey(key string) CacheSpanOption {
	return func(opts *cacheSpanOptions) {
		opts.key = key
		opts.attributes = append(opts.attributes, attribute.String("cache.key", key))
	}
}

// WithCacheHit sets whether the cache operation was a hit or miss.
func WithCacheHit(hit bool) CacheSpanOption {
	return func(opts *cacheSpanOptions) {
		opts.attributes = append(opts.attributes, attribute.Bool("cache.hit", hit))
	}
}

// RecordError records an error in the current span and sets the span status to error.
// This is a convenience function for consistent error recording.
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// RecordSuccess sets the span status to OK.
// This is a convenience function for marking successful operations.
func RecordSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}
