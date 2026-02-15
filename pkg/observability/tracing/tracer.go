// Package tracing provides OpenTelemetry distributed tracing support for the framework.
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerProvider wraps the OpenTelemetry tracer provider with lifecycle management.
// It provides methods for creating tracers and graceful shutdown.
//
// Requirements: 14.1, 14.2
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	config   TracerConfig
}

// TracerConfig holds configuration for the tracer provider.
type TracerConfig struct {
	// ServiceName identifies the service in traces
	ServiceName string
	
	// ServiceVersion is the version of the service
	ServiceVersion string
	
	// Environment identifies the deployment environment (dev, staging, prod)
	Environment string
	
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317")
	Endpoint string
	
	// SampleRate is the fraction of traces to sample (0.0 to 1.0)
	SampleRate float64
	
	// Enabled controls whether tracing is active
	Enabled bool
}

// NewTracerProvider creates and initializes a new tracer provider with OTLP exporter.
// It configures the provider with service information, sampling rate, and OTLP endpoint.
//
// Requirements: 14.1, 14.7
func NewTracerProvider(ctx context.Context, cfg TracerConfig) (*TracerProvider, error) {
	if !cfg.Enabled {
		// Return a no-op provider when tracing is disabled
		return &TracerProvider{
			provider: sdktrace.NewTracerProvider(),
			config:   cfg,
		}, nil
	}
	
	// Validate configuration
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("OTLP endpoint is required")
	}
	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		return nil, fmt.Errorf("sample rate must be between 0 and 1")
	}
	
	// Create OTLP exporter
	exporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithInsecure(), // Use WithTLSCredentials in production
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}
	
	// Create resource with service information
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Create tracer provider with sampling
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)
	
	// Set global tracer provider
	otel.SetTracerProvider(provider)
	
	// Set global propagator for context propagation
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	
	return &TracerProvider{
		provider: provider,
		config:   cfg,
	}, nil
}

// Tracer returns a tracer for the given instrumentation scope.
// The name should identify the instrumentation library (e.g., "http", "database").
//
// Requirements: 14.3, 14.4, 14.5
func (tp *TracerProvider) Tracer(name string) trace.Tracer {
	return tp.provider.Tracer(name)
}

// Shutdown gracefully shuts down the tracer provider, flushing any pending spans.
// It should be called during application shutdown to ensure all traces are exported.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}
	
	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	if err := tp.provider.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}
	
	return nil
}

// ForceFlush forces the tracer provider to flush any pending spans.
// This is useful for ensuring traces are exported before a timeout.
func (tp *TracerProvider) ForceFlush(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}
	
	if err := tp.provider.ForceFlush(ctx); err != nil {
		return fmt.Errorf("failed to flush tracer provider: %w", err)
	}
	
	return nil
}
