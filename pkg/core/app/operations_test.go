package app

import (
	"context"
	"testing"
	"time"

	nfharness "github.com/nimburion/nimburion/internal/testharness/nonfunctional"
	"github.com/nimburion/nimburion/pkg/featureflag"
)

func TestFailureInjectorApplyError(t *testing.T) {
	t.Parallel()

	injector := NewFailureInjector()
	injector.Set(FailureSpec{
		Target:  HookFailureTarget(PhaseConfigResolution, "config"),
		Mode:    FailureModeError,
		Message: "forced failure",
	})

	err := injector.Apply(context.Background(), HookFailureTarget(PhaseConfigResolution, "config"))
	if err == nil || err.Error() != "forced failure" {
		t.Fatalf("Apply() error = %v, want forced failure", err)
	}
}

func TestPrepareFailureInjectionStopsPhase(t *testing.T) {
	t.Parallel()

	injector := NewFailureInjector()
	injector.Set(FailureSpec{
		Target:  HookFailureTarget(PhaseConfigResolution, "config"),
		Mode:    FailureModeError,
		Message: "forced startup fault",
	})

	a, err := New(Options{
		FailureInjector: injector,
		ConfigResolvers: []Hook{{
			Name: "config",
			Fn: func(ctx context.Context, runtime *Runtime) error {
				return nil
			},
		}},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = a.Prepare(context.Background())
	if err == nil || err.Error() == "" {
		t.Fatal("expected prepare error")
	}
	if snapshot := a.Runtime().Posture.Snapshot(); snapshot.Startup != featureflag.StartupFailed {
		t.Fatalf("startup = %s, want failed", snapshot.Startup)
	}
}

func TestDeploymentPostureValidateRejectsUnsupportedCombination(t *testing.T) {
	t.Parallel()

	posture := NewDeploymentPosture()
	posture.Update(func(current *DeploymentPosture) {
		current.Level = PostureMultiRegionActivePassive
		current.RegionScope = "eu,us"
		current.WriterTopology = "single-writer"
		current.ReplayRequired = true
		current.IdempotencyRequired = false
	})

	err := posture.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSignalCatalogSnapshot(t *testing.T) {
	t.Parallel()

	catalog := NewSignalCatalog()
	catalog.Register(SignalAttachment{
		Name:        "jobs-queue-depth",
		Family:      "jobs",
		Class:       SignalSaturation,
		Metric:      "jobs_queue_depth",
		Description: "Current queue backlog.",
	})

	snapshot := catalog.Snapshot().([]SignalAttachment)
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
}

func TestFailureInjectorDelayHonorsContext(t *testing.T) {
	t.Parallel()

	injector := NewFailureInjector()
	injector.Set(FailureSpec{
		Target: HookFailureTarget(PhaseConfigResolution, "config"),
		Mode:   FailureModeDelay,
		Delay:  50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := injector.Apply(ctx, HookFailureTarget(PhaseConfigResolution, "config"))
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestResilienceFailureInjection_HookTarget(t *testing.T) {
	nfharness.Run(t, nfharness.CategoryResilience, func(t *testing.T) {
		injector := NewFailureInjector()
		injector.Set(FailureSpec{
			Target:  HookFailureTarget(PhaseConfigResolution, "config"),
			Mode:    FailureModeError,
			Message: "forced startup fault",
		})

		a, err := New(Options{
			FailureInjector: injector,
			ConfigResolvers: []Hook{{
				Name: "config",
				Fn: func(ctx context.Context, runtime *Runtime) error {
					return nil
				},
			}},
		})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}

		if err := a.Prepare(context.Background()); err == nil {
			t.Fatal("expected prepare error")
		}
	})
}

func TestCompatibilityDeploymentPosture_Validation(t *testing.T) {
	nfharness.Run(t, nfharness.CategoryCompatibility, func(t *testing.T) {
		posture := NewDeploymentPosture()
		posture.Update(func(current *DeploymentPosture) {
			current.Level = PostureMultiRegionActivePassive
			current.RegionScope = "eu,us"
			current.WriterTopology = "single-writer"
			current.ReplayRequired = true
			current.IdempotencyRequired = false
		})

		if err := posture.Validate(); err == nil {
			t.Fatal("expected validation error")
		}
	})
}
