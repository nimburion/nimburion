package featureflag

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryEvalBoolUsesSafeDefaultOnProviderError(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterBool(BoolDefinition{Key: "new-ui", Default: false})
	registry.SetProvider(ProviderFunc(func(ctx context.Context, key string, target TargetContext) (any, bool, error) {
		return nil, false, errors.New("provider unavailable")
	}))

	evaluation := registry.EvalBool(context.Background(), "new-ui", TargetContext{AppName: "nimburion"})
	if evaluation.Value {
		t.Fatal("expected safe default value")
	}
	if evaluation.Source != SourceSafeDefault {
		t.Fatalf("source = %s, want %s", evaluation.Source, SourceSafeDefault)
	}
}

func TestRegistryEvalBoolUsesProviderValue(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterBool(BoolDefinition{Key: "new-ui", Default: false})
	registry.SetProvider(ProviderFunc(func(ctx context.Context, key string, target TargetContext) (any, bool, error) {
		if target.Attributes["region"] == "eu" {
			return true, true, nil
		}
		return false, false, nil
	}))

	evaluation := registry.EvalBool(context.Background(), "new-ui", TargetContext{
		AppName:    "nimburion",
		Attributes: map[string]string{"region": "eu"},
	})
	if !evaluation.Value {
		t.Fatal("expected provider value")
	}
	if evaluation.Source != SourceProvider {
		t.Fatalf("source = %s, want %s", evaluation.Source, SourceProvider)
	}
}

func TestRegistryEvalStringDefaultsWithoutProvider(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterString(StringDefinition{Key: "banner", Default: "stable"})

	evaluation := registry.EvalString(context.Background(), "banner", TargetContext{})
	if evaluation.Value != "stable" {
		t.Fatalf("value = %q, want stable", evaluation.Value)
	}
	if evaluation.Source != SourceDefault {
		t.Fatalf("source = %s, want %s", evaluation.Source, SourceDefault)
	}
}

func TestRegistrySnapshotIncludesRegisteredFlags(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterBool(BoolDefinition{Key: "new-ui", Default: true})
	registry.RegisterInt(IntDefinition{Key: "batch-size", Default: 10})

	snapshot := registry.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot size = %d, want 2", len(snapshot))
	}
}
