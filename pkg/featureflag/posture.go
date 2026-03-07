package featureflag

import (
	"context"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/health"
)

const (
	// CheckNameRuntimeStartup is the health check name for startup posture.
	CheckNameRuntimeStartup = "runtime-startup"
	// CheckNameRuntimeReadiness is the health check name for readiness posture.
	CheckNameRuntimeReadiness = "runtime-readiness"
	// CheckNameRuntimeLiveness is the health check name for liveness posture.
	CheckNameRuntimeLiveness = "runtime-liveness"
	// CheckNameRuntimePosture is the health check name for the aggregated runtime posture.
	CheckNameRuntimePosture = "runtime-posture"
)

// StartupState describes startup progression.
type StartupState string

const (
	StartupPending  StartupState = "pending"
	StartupStarting StartupState = "starting"
	StartupReady    StartupState = "ready"
	StartupFailed   StartupState = "failed"
)

// ReadinessState describes whether the runtime should accept traffic.
type ReadinessState string

const (
	ReadinessBlocked  ReadinessState = "blocked"
	ReadinessReady    ReadinessState = "ready"
	ReadinessDegraded ReadinessState = "degraded"
)

// LivenessState describes whether the runtime is alive enough to stay running.
type LivenessState string

const (
	LivenessAlive    LivenessState = "alive"
	LivenessStopping LivenessState = "stopping"
	LivenessStopped  LivenessState = "stopped"
)

// Mode describes the current runtime posture mode.
type Mode string

const (
	ModeNormal   Mode = "normal"
	ModeDegraded Mode = "degraded"
)

// Snapshot contains the current runtime posture.
type Snapshot struct {
	Startup   StartupState   `json:"startup"`
	Readiness ReadinessState `json:"readiness"`
	Liveness  LivenessState  `json:"liveness"`
	Mode      Mode           `json:"mode"`
	Message   string         `json:"message,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// RuntimePosture stores the explicit shared runtime posture contract.
type RuntimePosture struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

// NewRuntimePosture creates the default posture for a not-yet-started runtime.
func NewRuntimePosture() *RuntimePosture {
	return &RuntimePosture{
		snapshot: Snapshot{
			Startup:   StartupPending,
			Readiness: ReadinessBlocked,
			Liveness:  LivenessAlive,
			Mode:      ModeNormal,
			UpdatedAt: time.Now().UTC(),
		},
	}
}

// Snapshot returns the current posture snapshot.
func (p *RuntimePosture) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.snapshot
}

// SetStartup updates the startup posture.
func (p *RuntimePosture) SetStartup(state StartupState, message string) {
	p.update(func(snapshot *Snapshot) {
		snapshot.Startup = state
		snapshot.Message = message
	})
}

// SetReadiness updates the readiness posture.
func (p *RuntimePosture) SetReadiness(state ReadinessState, message string) {
	p.update(func(snapshot *Snapshot) {
		snapshot.Readiness = state
		snapshot.Message = message
	})
}

// SetLiveness updates the liveness posture.
func (p *RuntimePosture) SetLiveness(state LivenessState, message string) {
	p.update(func(snapshot *Snapshot) {
		snapshot.Liveness = state
		snapshot.Message = message
	})
}

// SetMode updates the runtime mode and keeps readiness aligned with degraded operation.
func (p *RuntimePosture) SetMode(mode Mode, message string) {
	p.update(func(snapshot *Snapshot) {
		snapshot.Mode = mode
		snapshot.Message = message
		switch mode {
		case ModeDegraded:
			if snapshot.Readiness == ReadinessReady {
				snapshot.Readiness = ReadinessDegraded
			}
		default:
			if snapshot.Readiness == ReadinessDegraded {
				snapshot.Readiness = ReadinessReady
			}
		}
	})
}

func (p *RuntimePosture) update(fn func(*Snapshot)) {
	if p == nil || fn == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	fn(&p.snapshot)
	p.snapshot.UpdatedAt = time.Now().UTC()
}

// RegisterHealthChecks exposes the posture contract through the shared health registry.
func RegisterHealthChecks(registry *health.Registry, posture *RuntimePosture) {
	if registry == nil || posture == nil {
		return
	}

	registry.Register(NewStartupChecker(CheckNameRuntimeStartup, posture))
	registry.Register(NewReadinessChecker(CheckNameRuntimeReadiness, posture))
	registry.Register(NewLivenessChecker(CheckNameRuntimeLiveness, posture))
	registry.Register(NewPostureChecker(CheckNameRuntimePosture, posture))
}

// NewStartupChecker exposes startup posture through the health registry.
func NewStartupChecker(name string, posture *RuntimePosture) health.Checker {
	return health.NewCustomChecker(name, func(ctx context.Context) (health.Status, string, error) {
		snapshot := posture.Snapshot()
		switch snapshot.Startup {
		case StartupReady:
			return health.StatusHealthy, "startup completed", nil
		case StartupFailed:
			return health.StatusUnhealthy, snapshotMessage(snapshot, "startup failed"), nil
		default:
			return health.StatusDegraded, snapshotMessage(snapshot, "startup in progress"), nil
		}
	})
}

// NewReadinessChecker exposes readiness posture through the health registry.
func NewReadinessChecker(name string, posture *RuntimePosture) health.Checker {
	return health.NewCustomChecker(name, func(ctx context.Context) (health.Status, string, error) {
		snapshot := posture.Snapshot()
		switch snapshot.Readiness {
		case ReadinessReady:
			return health.StatusHealthy, "runtime is ready", nil
		case ReadinessDegraded:
			return health.StatusDegraded, snapshotMessage(snapshot, "runtime is ready in degraded mode"), nil
		default:
			return health.StatusUnhealthy, snapshotMessage(snapshot, "runtime is not ready"), nil
		}
	})
}

// NewLivenessChecker exposes liveness posture through the health registry.
func NewLivenessChecker(name string, posture *RuntimePosture) health.Checker {
	return health.NewCustomChecker(name, func(ctx context.Context) (health.Status, string, error) {
		snapshot := posture.Snapshot()
		switch snapshot.Liveness {
		case LivenessAlive:
			return health.StatusHealthy, "runtime is alive", nil
		case LivenessStopping:
			return health.StatusDegraded, snapshotMessage(snapshot, "runtime is stopping"), nil
		default:
			return health.StatusUnhealthy, snapshotMessage(snapshot, "runtime is stopped"), nil
		}
	})
}

// NewPostureChecker exposes the aggregated runtime posture through the health registry.
func NewPostureChecker(name string, posture *RuntimePosture) health.Checker {
	return health.NewCustomChecker(name, func(ctx context.Context) (health.Status, string, error) {
		snapshot := posture.Snapshot()
		switch {
		case snapshot.Liveness == LivenessStopped || snapshot.Startup == StartupFailed || snapshot.Readiness == ReadinessBlocked:
			return health.StatusUnhealthy, snapshotMessage(snapshot, "runtime posture is blocking"), nil
		case snapshot.Mode == ModeDegraded || snapshot.Readiness == ReadinessDegraded || snapshot.Liveness == LivenessStopping || snapshot.Startup != StartupReady:
			return health.StatusDegraded, snapshotMessage(snapshot, "runtime posture is degraded"), nil
		default:
			return health.StatusHealthy, "runtime posture is healthy", nil
		}
	})
}

func snapshotMessage(snapshot Snapshot, fallback string) string {
	if snapshot.Message != "" {
		return snapshot.Message
	}
	return fallback
}
