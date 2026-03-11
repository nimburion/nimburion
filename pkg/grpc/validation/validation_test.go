package validation

import (
	"context"
	"errors"
	"testing"
)

func TestPipelineStopsAtFirstLayerFailure(t *testing.T) {
	pipeline := Pipeline{
		Transport: []Validator{
			ValidatorFunc(func(context.Context, any) error {
				return errors.New("bad metadata")
			}),
		},
		Contract: []Validator{
			ValidatorFunc(func(context.Context, any) error {
				t.Fatal("contract validator should not run")
				return nil
			}),
		},
	}

	err := pipeline.Validate(context.Background(), "req")
	if !IsLayer(err, LayerTransport) {
		t.Fatalf("expected transport validation error, got %v", err)
	}
}

func TestPipelinePreservesExplicitLayerErrors(t *testing.T) {
	want := NewError(LayerDomain, "domain failed", nil)
	pipeline := Pipeline{
		Domain: []Validator{
			ValidatorFunc(func(context.Context, any) error { return want }),
		},
	}

	err := pipeline.Validate(context.Background(), "req")
	if !errors.Is(err, want) {
		t.Fatalf("expected original error to be preserved")
	}
}
