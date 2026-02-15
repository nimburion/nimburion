package tracing

import (
	"context"
	"testing"
	"time"
)

func TestNewTracerProvider_Disabled(t *testing.T) {
	ctx := context.Background()
	
	cfg := TracerConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}
	
	provider, err := NewTracerProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("expected no error for disabled tracing, got: %v", err)
	}
	
	if provider == nil {
		t.Fatal("expected provider to be non-nil")
	}
	
	// Should be able to get a tracer even when disabled
	tracer := provider.Tracer("test")
	if tracer == nil {
		t.Fatal("expected tracer to be non-nil")
	}
}

func TestNewTracerProvider_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name        string
		config      TracerConfig
		expectedErr string
	}{
		{
			name: "missing service name",
			config: TracerConfig{
				Enabled:  true,
				Endpoint: "localhost:4317",
			},
			expectedErr: "service name is required",
		},
		{
			name: "missing endpoint",
			config: TracerConfig{
				ServiceName: "test-service",
				Enabled:     true,
			},
			expectedErr: "OTLP endpoint is required",
		},
		{
			name: "invalid sample rate - negative",
			config: TracerConfig{
				ServiceName: "test-service",
				Endpoint:    "localhost:4317",
				SampleRate:  -0.1,
				Enabled:     true,
			},
			expectedErr: "sample rate must be between 0 and 1",
		},
		{
			name: "invalid sample rate - too high",
			config: TracerConfig{
				ServiceName: "test-service",
				Endpoint:    "localhost:4317",
				SampleRate:  1.5,
				Enabled:     true,
			},
			expectedErr: "sample rate must be between 0 and 1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTracerProvider(ctx, tt.config)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			
			if err.Error() != tt.expectedErr {
				t.Errorf("expected error %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestTracerProvider_Tracer(t *testing.T) {
	ctx := context.Background()
	
	cfg := TracerConfig{
		ServiceName: "test-service",
		Enabled:     false, // Disabled to avoid needing real OTLP endpoint
	}
	
	provider, err := NewTracerProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	
	tracer := provider.Tracer("test-tracer")
	if tracer == nil {
		t.Fatal("expected tracer to be non-nil")
	}
	
	// Should be able to create spans with the tracer
	_, span := tracer.Start(ctx, "test-span")
	if span == nil {
		t.Fatal("expected span to be non-nil")
	}
	span.End()
}

func TestTracerProvider_Shutdown(t *testing.T) {
	ctx := context.Background()
	
	cfg := TracerConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}
	
	provider, err := NewTracerProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	
	// Shutdown should not error
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	err = provider.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("expected no error on shutdown, got: %v", err)
	}
}

func TestTracerProvider_ForceFlush(t *testing.T) {
	ctx := context.Background()
	
	cfg := TracerConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}
	
	provider, err := NewTracerProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	// ForceFlush should not error
	flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	err = provider.ForceFlush(flushCtx)
	if err != nil {
		t.Errorf("expected no error on force flush, got: %v", err)
	}
}

func TestTracerConfig_ValidSampleRates(t *testing.T) {
	ctx := context.Background()
	
	validRates := []float64{0.0, 0.01, 0.1, 0.5, 1.0}
	
	for _, rate := range validRates {
		t.Run("sample_rate_"+string(rune(rate*100)), func(t *testing.T) {
			cfg := TracerConfig{
				ServiceName: "test-service",
				Enabled:     false, // Disabled to avoid needing real endpoint
				SampleRate:  rate,
			}
			
			_, err := NewTracerProvider(ctx, cfg)
			if err != nil {
				t.Errorf("expected no error for sample rate %f, got: %v", rate, err)
			}
		})
	}
}
