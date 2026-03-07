package featureflag

import (
	"context"
	"testing"

	"github.com/nimburion/nimburion/pkg/health"
)

func TestRegisterHealthChecksReflectsPostureTransitions(t *testing.T) {
	registry := health.NewRegistry()
	posture := NewRuntimePosture()
	RegisterHealthChecks(registry, posture)

	posture.SetStartup(StartupStarting, "booting")
	posture.SetReadiness(ReadinessBlocked, "waiting for dependencies")
	posture.SetLiveness(LivenessAlive, "")

	startup, err := registry.CheckOne(context.Background(), CheckNameRuntimeStartup)
	if err != nil {
		t.Fatalf("startup check error = %v", err)
	}
	if startup.Status != health.StatusDegraded {
		t.Fatalf("startup status = %s, want degraded", startup.Status)
	}

	posture.SetStartup(StartupReady, "")
	posture.SetReadiness(ReadinessDegraded, "degraded but serving")

	readiness, err := registry.CheckOne(context.Background(), CheckNameRuntimeReadiness)
	if err != nil {
		t.Fatalf("readiness check error = %v", err)
	}
	if readiness.Status != health.StatusDegraded {
		t.Fatalf("readiness status = %s, want degraded", readiness.Status)
	}

	posture.SetReadiness(ReadinessReady, "")
	posture.SetLiveness(LivenessStopping, "shutdown in progress")

	liveness, err := registry.CheckOne(context.Background(), CheckNameRuntimeLiveness)
	if err != nil {
		t.Fatalf("liveness check error = %v", err)
	}
	if liveness.Status != health.StatusDegraded {
		t.Fatalf("liveness status = %s, want degraded", liveness.Status)
	}
}

func TestSetModeKeepsReadinessAligned(t *testing.T) {
	posture := NewRuntimePosture()
	posture.SetReadiness(ReadinessReady, "")
	posture.SetMode(ModeDegraded, "serving with reduced guarantees")

	snapshot := posture.Snapshot()
	if snapshot.Readiness != ReadinessDegraded {
		t.Fatalf("readiness = %s, want degraded", snapshot.Readiness)
	}

	posture.SetMode(ModeNormal, "recovered")
	snapshot = posture.Snapshot()
	if snapshot.Readiness != ReadinessReady {
		t.Fatalf("readiness = %s, want ready", snapshot.Readiness)
	}
}
