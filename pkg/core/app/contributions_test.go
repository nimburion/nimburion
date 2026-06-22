package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nimburion/nimburion/pkg/core/feature"
)

// structOnlyFeature contributes exclusively via Contributions().
type structOnlyFeature struct {
	name string
	tag  string
}

func (f structOnlyFeature) Name() string { return f.name }

func (f structOnlyFeature) Contributions() feature.Contributions {
	return feature.Contributions{
		StartupHooks: []feature.Hook{{Name: f.tag}},
	}
}

// providerOnlyFeature returns an empty Contributions() and exposes its
// contributions through the optional provider interfaces only.
type providerOnlyFeature struct {
	name string
	tag  string
}

func (f providerOnlyFeature) Name() string { return f.name }

func (f providerOnlyFeature) Contributions() feature.Contributions {
	return feature.Contributions{}
}

func (f providerOnlyFeature) StartupHooks() []feature.Hook {
	return []feature.Hook{{Name: f.tag}}
}

func (f providerOnlyFeature) ShutdownHooks() []feature.Hook {
	return []feature.Hook{{Name: f.tag + "-shutdown"}}
}

// mixedFeature provides one category via Contributions() and a different,
// empty-in-struct category via an optional provider. It also re-declares the
// struct-provided category via a provider to prove the struct wins.
type mixedFeature struct {
	name string
}

func (f mixedFeature) Name() string { return f.name }

func (f mixedFeature) Contributions() feature.Contributions {
	return feature.Contributions{
		StartupHooks: []feature.Hook{{Name: "struct-startup"}},
	}
}

// StartupHooks would double-count if consulted; it must be ignored because the
// struct already populated StartupHooks.
func (f mixedFeature) StartupHooks() []feature.Hook {
	return []feature.Hook{{Name: "provider-startup-should-be-ignored"}}
}

func (f mixedFeature) ServiceConstructors() []feature.Hook {
	return []feature.Hook{{Name: "provider-service"}}
}

func hookNames(hooks []feature.Hook) []string {
	names := make([]string, 0, len(hooks))
	for _, h := range hooks {
		names = append(names, h.Name)
	}
	return names
}

func TestEffectiveContributions_StructOnly(t *testing.T) {
	t.Parallel()

	got := effectiveContributions(structOnlyFeature{name: "s", tag: "struct-hook"})
	if names := hookNames(got.StartupHooks); !reflect.DeepEqual(names, []string{"struct-hook"}) {
		t.Fatalf("startup hooks = %v, want [struct-hook]", names)
	}
}

func TestEffectiveContributions_ProviderOnly(t *testing.T) {
	t.Parallel()

	got := effectiveContributions(providerOnlyFeature{name: "p", tag: "prov"})
	if names := hookNames(got.StartupHooks); !reflect.DeepEqual(names, []string{"prov"}) {
		t.Fatalf("startup hooks = %v, want [prov]", names)
	}
	if names := hookNames(got.ShutdownHooks); !reflect.DeepEqual(names, []string{"prov-shutdown"}) {
		t.Fatalf("shutdown hooks = %v, want [prov-shutdown]", names)
	}
}

func TestEffectiveContributions_Mixed_StructWinsNoDoubleCount(t *testing.T) {
	t.Parallel()

	got := effectiveContributions(mixedFeature{name: "m"})

	// Struct-provided category wins; the provider variant is ignored.
	if names := hookNames(got.StartupHooks); !reflect.DeepEqual(names, []string{"struct-startup"}) {
		t.Fatalf("startup hooks = %v, want [struct-startup]", names)
	}
	// Category absent from struct is filled by the provider.
	if names := hookNames(got.ServiceConstructors); !reflect.DeepEqual(names, []string{"provider-service"}) {
		t.Fatalf("service constructors = %v, want [provider-service]", names)
	}
}

func TestCollectFeatureContributions_MergesProviderFeatures(t *testing.T) {
	t.Parallel()

	collected, err := collectFeatureContributions([]feature.Feature{
		structOnlyFeature{name: "s", tag: "struct-hook"},
		providerOnlyFeature{name: "p", tag: "prov"},
		nil,
	})
	if err != nil {
		t.Fatalf("collectFeatureContributions() error = %v", err)
	}

	if names := hookNames(collected.startupHooks); !reflect.DeepEqual(names, []string{"struct-hook", "prov"}) {
		t.Fatalf("startup hooks = %v, want [struct-hook prov]", names)
	}
	if names := hookNames(collected.shutdownHooks); !reflect.DeepEqual(names, []string{"prov-shutdown"}) {
		t.Fatalf("shutdown hooks = %v, want [prov-shutdown]", names)
	}
}

// --- TASK C: dependency ordering ---

type orderedFeature struct {
	name      string
	dependsOn []string
	record    func(string)
}

func (f orderedFeature) Name() string { return f.name }

func (f orderedFeature) Contributions() feature.Contributions {
	return feature.Contributions{
		StartupHooks: []feature.Hook{{
			Name: f.name,
			Fn: func(_ context.Context, _ feature.Runtime) error {
				if f.record != nil {
					f.record(f.name)
				}
				return nil
			},
		}},
	}
}

func (f orderedFeature) DependsOn() []string { return f.dependsOn }

func TestOrderFeatures_LinearDependencyReorders(t *testing.T) {
	t.Parallel()

	// Registration order is reversed from dependency order on purpose.
	features := []feature.Feature{
		orderedFeature{name: "c", dependsOn: []string{"b"}},
		orderedFeature{name: "b", dependsOn: []string{"a"}},
		orderedFeature{name: "a"},
	}

	ordered, err := orderFeatures(features)
	if err != nil {
		t.Fatalf("orderFeatures() error = %v", err)
	}

	got := make([]string, 0, len(ordered))
	for _, f := range ordered {
		got = append(got, f.Name())
	}
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("order = %v, want [a b c]", got)
	}
}

func TestOrderFeatures_PreservesRegistrationOrderWithoutDeps(t *testing.T) {
	t.Parallel()

	features := []feature.Feature{
		orderedFeature{name: "x"},
		orderedFeature{name: "y"},
		orderedFeature{name: "z"},
	}

	ordered, err := orderFeatures(features)
	if err != nil {
		t.Fatalf("orderFeatures() error = %v", err)
	}

	got := make([]string, 0, len(ordered))
	for _, f := range ordered {
		got = append(got, f.Name())
	}
	if !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Fatalf("order = %v, want [x y z]", got)
	}
}

func TestOrderFeatures_MissingDependencyReported(t *testing.T) {
	t.Parallel()

	features := []feature.Feature{
		orderedFeature{name: "a", dependsOn: []string{"ghost"}},
	}

	_, err := orderFeatures(features)
	if err == nil {
		t.Fatal("expected missing dependency error")
	}
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("error = %v, want *DependencyError", err)
	}
	if depErr.Kind != ErrMissingDependency {
		t.Fatalf("kind = %v, want ErrMissingDependency", depErr.Kind)
	}
	if depErr.Feature != "a" || depErr.Missing != "ghost" {
		t.Fatalf("error = %+v, want feature=a missing=ghost", depErr)
	}
}

func TestOrderFeatures_CycleDetected(t *testing.T) {
	t.Parallel()

	features := []feature.Feature{
		orderedFeature{name: "a", dependsOn: []string{"b"}},
		orderedFeature{name: "b", dependsOn: []string{"a"}},
	}

	_, err := orderFeatures(features)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("error = %v, want *DependencyError", err)
	}
	if depErr.Kind != ErrDependencyCycle {
		t.Fatalf("kind = %v, want ErrDependencyCycle", depErr.Kind)
	}
	if len(depErr.Cycle) == 0 {
		t.Fatal("expected cycle path to be populated")
	}
}

func TestNew_ReturnsDependencyError(t *testing.T) {
	t.Parallel()

	_, err := New(Options{
		Features: []feature.Feature{
			orderedFeature{name: "a", dependsOn: []string{"missing"}},
		},
	})
	if err == nil {
		t.Fatal("expected New to surface dependency error")
	}
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("error = %v, want *DependencyError", err)
	}
}

func TestRun_ExecutesStartupHooksInDependencyOrder(t *testing.T) {
	t.Parallel()

	var order []string
	record := func(name string) { order = append(order, name) }

	a, err := New(Options{
		Features: []feature.Feature{
			orderedFeature{name: "c", dependsOn: []string{"b"}, record: record},
			orderedFeature{name: "b", dependsOn: []string{"a"}, record: record},
			orderedFeature{name: "a", record: record},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if !reflect.DeepEqual(order, []string{"a", "b", "c"}) {
		t.Fatalf("startup order = %v, want [a b c]", order)
	}
}
