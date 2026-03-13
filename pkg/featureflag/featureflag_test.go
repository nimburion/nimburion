package featureflag

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryEvalBoolUsesSafeDefaultOnProviderError(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBool(BoolDefinition{Key: "new-ui", Default: false}); err != nil {
		t.Fatalf("register bool: %v", err)
	}
	registry.SetProvider(ProviderFunc(func(_ context.Context, _ string, _ TargetContext) (any, bool, error) {
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
	if err := registry.RegisterBool(BoolDefinition{Key: "new-ui", Default: false}); err != nil {
		t.Fatalf("register bool: %v", err)
	}
	registry.SetProvider(ProviderFunc(func(_ context.Context, _ string, target TargetContext) (any, bool, error) {
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
	if err := registry.RegisterString(StringDefinition{Key: "banner", Default: "stable"}); err != nil {
		t.Fatalf("register string: %v", err)
	}

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
	if err := registry.RegisterBool(BoolDefinition{Key: "new-ui", Default: true}); err != nil {
		t.Fatalf("register bool: %v", err)
	}
	if err := registry.RegisterInt(IntDefinition{Key: "batch-size", Default: 10}); err != nil {
		t.Fatalf("register int: %v", err)
	}

	snapshot := registry.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot size = %d, want 2", len(snapshot))
	}
}

func TestRegistryRejectsCrossTypeDuplicateKey(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterBool(BoolDefinition{Key: "same", Default: false}); err != nil {
		t.Fatalf("register bool: %v", err)
	}
	if err := registry.RegisterString(StringDefinition{Key: "same", Default: "x"}); err == nil {
		t.Fatal("expected cross-type duplicate registration error")
	}
}

func TestRegistryEvalIntAcceptsInt64AndFloat64(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterInt(IntDefinition{Key: "batch-size", Default: 10}); err != nil {
		t.Fatalf("register int: %v", err)
	}

	registry.SetProvider(ProviderFunc(func(_ context.Context, key string, _ TargetContext) (any, bool, error) {
		switch key {
		case "batch-size":
			return int64(42), true, nil
		default:
			return float64(0), false, nil
		}
	}))

	evaluation := registry.EvalInt(context.Background(), "batch-size", TargetContext{})
	if evaluation.Value != 42 {
		t.Fatalf("value = %d, want 42", evaluation.Value)
	}
	if evaluation.Source != SourceProvider {
		t.Fatalf("source = %s, want %s", evaluation.Source, SourceProvider)
	}
}

func TestRegistryEmitsEvaluationEvents(t *testing.T) {
	registry := NewRegistry()
	if err := registry.RegisterInt(IntDefinition{Key: "batch-size", Default: 10}); err != nil {
		t.Fatalf("register int: %v", err)
	}

	events := make([]Event, 0, 2)
	registry.AddObserver(ObserverFunc(func(_ context.Context, event Event) {
		events = append(events, event)
	}))

	registry.SetProvider(ProviderFunc(func(_ context.Context, _ string, _ TargetContext) (any, bool, error) {
		return "wrong", true, nil
	}))

	_ = registry.EvalInt(context.Background(), "batch-size", TargetContext{Environment: "test"})

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	last := events[len(events)-1]
	if last.Type != EventTypeEvaluated {
		t.Fatalf("event type = %s, want %s", last.Type, EventTypeEvaluated)
	}
	if last.Outcome != "type_mismatch" {
		t.Fatalf("event outcome = %s, want type_mismatch", last.Outcome)
	}
}
