package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/featureflag"
	"github.com/nimburion/nimburion/pkg/health"
)

type demoFeature struct {
	record func(string)
}

func (f demoFeature) Name() string { return "demo" }

func (f demoFeature) Contributions() feature.Contributions {
	return feature.Contributions{
		StartupHooks: []feature.Hook{{
			Name: "feature-startup",
			Fn: func(_ context.Context, _ feature.Runtime) error {
				f.record("feature-startup")
				return nil
			},
		}},
		HealthContributors: []feature.Hook{{
			Name: "feature-health",
			Fn: func(_ context.Context, runtime feature.Runtime) error {
				runtime.HealthRegistry().RegisterFunc("feature-health", func(_ context.Context) health.CheckResult {
					return health.CheckResult{Name: "feature-health", Status: health.StatusHealthy}
				})
				f.record("feature-health")
				return nil
			},
		}},
		InstrumentationHooks: []feature.Hook{{
			Name: "feature-instrumentation",
			Fn: func(_ context.Context, _ feature.Runtime) error {
				f.record("feature-instrumentation")
				return nil
			},
		}},
		ServiceConstructors: []feature.Hook{{
			Name: "feature-service",
			Fn: func(_ context.Context, runtime feature.Runtime) error {
				runtime.RegisterService("feature-service", "ready")
				f.record("feature-service")
				return nil
			},
		}},
		RuntimeRunners: []feature.Runner{{
			Name: "feature-runner",
			Fn: func(_ context.Context, _ feature.Runtime) error {
				f.record("feature-runner")
				return nil
			},
		}},
		ShutdownHooks: []feature.Hook{{
			Name: "feature-shutdown",
			Fn: func(_ context.Context, _ feature.Runtime) error {
				f.record("feature-shutdown")
				return nil
			},
		}},
		IntrospectionContributors: []feature.Hook{{
			Name: "feature-introspection",
			Fn: func(_ context.Context, runtime feature.Runtime) error {
				runtime.IntrospectionRegistry().Set("feature", "enabled")
				f.record("feature-introspection")
				return nil
			},
		}},
	}
}

func TestRunExecutesLifecyclePhasesInOrder(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		steps []string
	)

	record := func(name string) Hook {
		return Hook{
			Name: name,
			Fn: func(_ context.Context, _ *Runtime) error {
				mu.Lock()
				defer mu.Unlock()
				steps = append(steps, name)
				return nil
			},
		}
	}

	a, err := New(Options{
		ConfigResolvers:      []Hook{record("config")},
		ObservabilityHooks:   []Hook{record("observability")},
		FeatureRegistrations: []Hook{record("features")},
		HealthRegistrations:  []Hook{record("health")},
		ServiceConstructors:  []Hook{record("services")},
		Runners: []Runner{{
			Name: "runtime",
			Fn: func(_ context.Context, _ *Runtime) error {
				mu.Lock()
				steps = append(steps, "runtime")
				mu.Unlock()
				return nil
			},
		}},
		ShutdownHooks: []Hook{record("shutdown")},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{"config", "observability", "features", "health", "services", "runtime", "shutdown"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("lifecycle order = %v, want %v", steps, want)
	}
}

func TestRunExecutesShutdownHooksInReverseOrder(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		steps []string
	)

	shutdownHook := func(name string) Hook {
		return Hook{
			Name: name,
			Fn: func(_ context.Context, _ *Runtime) error {
				mu.Lock()
				defer mu.Unlock()
				steps = append(steps, name)
				return nil
			},
		}
	}

	a, err := New(Options{
		Runners: []Runner{{
			Name: "runtime",
			Fn: func(_ context.Context, _ *Runtime) error {
				return nil
			},
		}},
		ShutdownHooks: []Hook{
			shutdownHook("shutdown-1"),
			shutdownHook("shutdown-2"),
			shutdownHook("shutdown-3"),
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{"shutdown-3", "shutdown-2", "shutdown-1"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("shutdown order = %v, want %v", steps, want)
	}
}

func TestRunNormalizesNilContext(t *testing.T) {
	t.Parallel()

	a, err := New(Options{
		Runners: []Runner{{
			Name: "runtime",
			Fn: func(ctx context.Context, _ *Runtime) error {
				if ctx == nil {
					t.Fatal("expected Run to normalize nil context")
				}
				return nil
			},
		}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Run(nil); err != nil {
		t.Fatalf("Run(nil) error = %v", err)
	}
}

func TestRunCancelsPeerRunnersWhenOneFails(t *testing.T) {
	t.Parallel()

	released := make(chan struct{})
	observedCancel := make(chan struct{})

	a, err := New(Options{
		Runners: []Runner{
			{
				Name: "failing",
				Fn: func(_ context.Context, _ *Runtime) error {
					close(released)
					return errors.New("boom")
				},
			},
			{
				Name: "peer",
				Fn: func(ctx context.Context, _ *Runtime) error {
					<-released
					<-ctx.Done()
					close(observedCancel)
					return nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = a.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Run() error = %v, want boom", err)
	}

	select {
	case <-observedCancel:
	case <-time.After(2 * time.Second):
		t.Fatal("peer runner did not observe cancellation")
	}
}

func TestRunStopsAtFirstStartupPhaseError(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		steps []string
	)

	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		steps = append(steps, name)
	}

	a, err := New(Options{
		ConfigResolvers: []Hook{{
			Name: "config",
			Fn: func(_ context.Context, _ *Runtime) error {
				record("config")
				return nil
			},
		}},
		ObservabilityHooks: []Hook{{
			Name: "observability",
			Fn: func(_ context.Context, _ *Runtime) error {
				record("observability")
				return errors.New("bad config")
			},
		}},
		FeatureRegistrations: []Hook{{
			Name: "features",
			Fn: func(_ context.Context, _ *Runtime) error {
				record("features")
				return nil
			},
		}},
		ShutdownHooks: []Hook{{
			Name: "shutdown",
			Fn: func(_ context.Context, _ *Runtime) error {
				record("shutdown")
				return nil
			},
		}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = a.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bad config") {
		t.Fatalf("Run() error = %v, want bad config", err)
	}

	want := []string{"config", "observability"}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
}

func TestRunExecutesFeatureContributionsWithoutEditingBaseRuntime(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		steps []string
	)

	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		steps = append(steps, name)
	}

	a, err := New(Options{
		Features: []feature.Feature{demoFeature{record: record}},
		Debug:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if service, ok := a.Runtime().Service("feature-service"); !ok || service != "ready" {
		t.Fatalf("service lookup = %v, %v; want ready, true", service, ok)
	}

	if _, err := a.Runtime().Health.CheckOne(context.Background(), "feature-health"); err != nil {
		t.Fatalf("health contribution not registered: %v", err)
	}

	value, ok := a.Runtime().Introspection.Get("feature")
	if !ok || value != "enabled" {
		t.Fatalf("introspection entry = %v, %v; want enabled, true", value, ok)
	}

	want := []string{
		"feature-startup",
		"feature-health",
		"feature-introspection",
		"feature-instrumentation",
		"feature-service",
		"feature-runner",
		"feature-shutdown",
	}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("feature lifecycle order = %v, want %v", steps, want)
	}
}

func TestPrepareSkipsIntrospectionContributionsWhenDebugDisabled(t *testing.T) {
	t.Parallel()

	a, err := New(Options{
		Features: []feature.Feature{demoFeature{record: func(string) {}}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if _, ok := a.Runtime().Introspection.Get("feature"); ok {
		t.Fatal("expected introspection entry to stay disabled without debug")
	}
}

func TestPrepareRegistersIntrospectionContributionsWhenDebugEnabled(t *testing.T) {
	t.Parallel()

	a, err := New(Options{
		Features: []feature.Feature{demoFeature{record: func(string) {}}},
		Debug:    true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	value, ok := a.Runtime().Introspection.Get("feature")
	if !ok || value != "enabled" {
		t.Fatalf("introspection entry = %v, %v; want enabled, true", value, ok)
	}
}

func TestPrepareUpdatesRuntimePosture(t *testing.T) {
	t.Parallel()

	a, err := New(Options{Name: "testapp"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	snapshot := a.Runtime().RuntimePosture().Snapshot()
	if snapshot.Startup != featureflag.StartupReady {
		t.Fatalf("startup = %s, want ready", snapshot.Startup)
	}
	if snapshot.Readiness != featureflag.ReadinessReady {
		t.Fatalf("readiness = %s, want ready", snapshot.Readiness)
	}
	if snapshot.Liveness != featureflag.LivenessAlive {
		t.Fatalf("liveness = %s, want alive", snapshot.Liveness)
	}
}

func TestPrepareFailureMarksRuntimePostureFailed(t *testing.T) {
	t.Parallel()

	a, err := New(Options{
		ObservabilityHooks: []Hook{{
			Name: "boom",
			Fn: func(_ context.Context, _ *Runtime) error {
				return errors.New("bad observability")
			},
		}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = a.Prepare(context.Background())
	if err == nil {
		t.Fatal("expected prepare error")
	}

	snapshot := a.Runtime().RuntimePosture().Snapshot()
	if snapshot.Startup != featureflag.StartupFailed {
		t.Fatalf("startup = %s, want failed", snapshot.Startup)
	}
	if snapshot.Readiness != featureflag.ReadinessBlocked {
		t.Fatalf("readiness = %s, want blocked", snapshot.Readiness)
	}
}
