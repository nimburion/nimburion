package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// FailureMode describes the active failure-injection behavior for one runtime surface.
type FailureMode string

const (
	// FailureModeOff disables failure injection for the target surface.
	FailureModeOff FailureMode = "off"
	// FailureModeError makes the target surface fail immediately.
	FailureModeError FailureMode = "error"
	// FailureModeDelay delays execution for the configured duration.
	FailureModeDelay FailureMode = "delay"
)

// FailureSpec configures one opt-in failure-injection rule.
type FailureSpec struct {
	Target   string            `json:"target"`
	Mode     FailureMode       `json:"mode"`
	Delay    time.Duration     `json:"delay,omitempty"`
	Message  string            `json:"message,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// FailureInjector stores opt-in failure-injection rules for runtime-owned target surfaces.
type FailureInjector struct {
	mu    sync.RWMutex
	specs map[string]FailureSpec
}

// NewFailureInjector creates an empty failure-injection registry.
func NewFailureInjector() *FailureInjector {
	return &FailureInjector{specs: make(map[string]FailureSpec)}
}

// Set activates or replaces one failure-injection rule.
func (i *FailureInjector) Set(spec FailureSpec) {
	if i == nil {
		return
	}
	target := strings.TrimSpace(spec.Target)
	if target == "" {
		return
	}
	if spec.Mode == "" {
		spec.Mode = FailureModeOff
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	i.specs[target] = spec
}

// Clear removes one failure-injection rule.
func (i *FailureInjector) Clear(target string) {
	if i == nil {
		return
	}

	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.specs, strings.TrimSpace(target))
}

// Snapshot returns the configured failure-injection rules.
func (i *FailureInjector) Snapshot() any {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	snapshot := make(map[string]FailureSpec, len(i.specs))
	for key, spec := range i.specs {
		snapshot[key] = spec
	}
	return snapshot
}

// Apply executes the configured failure mode for one target surface.
func (i *FailureInjector) Apply(ctx context.Context, target string) error {
	if i == nil {
		return nil
	}

	i.mu.RLock()
	spec, ok := i.specs[strings.TrimSpace(target)]
	i.mu.RUnlock()
	if !ok || spec.Mode == FailureModeOff {
		return nil
	}

	switch spec.Mode {
	case FailureModeDelay:
		if spec.Delay <= 0 {
			return nil
		}
		timer := time.NewTimer(spec.Delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	case FailureModeError:
		msg := strings.TrimSpace(spec.Message)
		if msg == "" {
			msg = "failure injection triggered"
		}
		return errors.New(msg)
	default:
		return nil
	}
}

// HookFailureTarget returns the canonical failure-injection target for one lifecycle hook.
func HookFailureTarget(phase Phase, hookName string) string {
	return fmt.Sprintf("phase:%s/hook:%s", phase, strings.TrimSpace(hookName))
}

// RunnerFailureTarget returns the canonical failure-injection target for one runtime runner.
func RunnerFailureTarget(name string) string {
	return "runner:" + strings.TrimSpace(name)
}

// ShutdownFailureTarget returns the canonical failure-injection target for one shutdown hook.
func ShutdownFailureTarget(name string) string {
	return "shutdown:" + strings.TrimSpace(name)
}

// PostureLevel describes deployment topology guarantees claimed by the runtime.
type PostureLevel string

const (
	// PostureSingleRegionOnly marks a runtime intended for single-region operation only.
	PostureSingleRegionOnly PostureLevel = "single_region_only"
	// PostureSingleWriterMultiRegionFailover marks a single-writer multi-region failover topology.
	PostureSingleWriterMultiRegionFailover PostureLevel = "single_writer_multi_region_failover"
	// PostureMultiRegionActivePassive marks an active-passive multi-region topology.
	PostureMultiRegionActivePassive PostureLevel = "multi_region_active_passive"
	// PostureMultiRegionActiveActiveSubset marks an active-active subset multi-region topology.
	PostureMultiRegionActiveActiveSubset PostureLevel = "multi_region_active_active_subset"
)

// RecoveryState describes the current failover or replay state of the runtime.
type RecoveryState string

const (
	// RecoveryNormal marks a runtime not in failover or replay recovery.
	RecoveryNormal RecoveryState = "normal"
	// RecoveryFailoverPending marks a runtime waiting for failover completion.
	RecoveryFailoverPending RecoveryState = "failover_pending"
	// RecoveryReplayRequired marks a runtime that requires replay before recovery completes.
	RecoveryReplayRequired RecoveryState = "replay_required"
	// RecoveryReconciling marks a runtime reconciling state after failover or replay.
	RecoveryReconciling RecoveryState = "reconciling"
	// RecoveryDegradedRecovery marks a runtime recovering in degraded mode.
	RecoveryDegradedRecovery RecoveryState = "degraded_recovery"
)

// DeploymentPosture describes multi-region and recovery metadata exposed by the runtime.
type DeploymentPosture struct {
	mu                                sync.RWMutex
	Level                             PostureLevel  `json:"level"`
	RecoveryState                     RecoveryState `json:"recovery_state"`
	RegionScope                       string        `json:"region_scope,omitempty"`
	WriterTopology                    string        `json:"writer_topology,omitempty"`
	ReplayRequired                    bool          `json:"replay_required"`
	ExternalDurableDependencyRequired bool          `json:"external_durable_dependency_required"`
	LeaderLockLocality                string        `json:"leader_lock_locality,omitempty"`
	OrderingScope                     string        `json:"ordering_scope,omitempty"`
	IdempotencyRequired               bool          `json:"idempotency_required"`
	DeduplicationBoundary             string        `json:"deduplication_boundary,omitempty"`
}

// NewDeploymentPosture creates the default deployment posture for a single-region runtime.
func NewDeploymentPosture() *DeploymentPosture {
	return &DeploymentPosture{
		Level:         PostureSingleRegionOnly,
		RecoveryState: RecoveryNormal,
	}
}

// Snapshot returns the current deployment posture metadata.
func (p *DeploymentPosture) Snapshot() any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return DeploymentPosture{
		Level:                             p.Level,
		RecoveryState:                     p.RecoveryState,
		RegionScope:                       p.RegionScope,
		WriterTopology:                    p.WriterTopology,
		ReplayRequired:                    p.ReplayRequired,
		ExternalDurableDependencyRequired: p.ExternalDurableDependencyRequired,
		LeaderLockLocality:                p.LeaderLockLocality,
		OrderingScope:                     p.OrderingScope,
		IdempotencyRequired:               p.IdempotencyRequired,
		DeduplicationBoundary:             p.DeduplicationBoundary,
	}
}

// Update mutates the deployment posture metadata.
func (p *DeploymentPosture) Update(fn func(*DeploymentPosture)) {
	if p == nil || fn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	fn(p)
}

// Validate rejects unsupported posture combinations.
func (p *DeploymentPosture) Validate() error {
	if p == nil {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	multiRegion := p.Level != PostureSingleRegionOnly
	if multiRegion {
		if strings.TrimSpace(p.RegionScope) == "" {
			return errors.New("deployment posture region_scope is required for multi-region levels")
		}
		if strings.TrimSpace(p.WriterTopology) == "" {
			return errors.New("deployment posture writer_topology is required for multi-region levels")
		}
	}
	if !multiRegion && p.RecoveryState != RecoveryNormal {
		return errors.New("single-region posture cannot declare failover recovery states")
	}
	if p.ReplayRequired && !p.IdempotencyRequired {
		return errors.New("replay_required posture must also require idempotency")
	}
	if multiRegion && strings.EqualFold(strings.TrimSpace(p.LeaderLockLocality), "single_instance_only") {
		return errors.New("multi-region posture cannot use single_instance_only leader lock locality")
	}
	if multiRegion && p.ExternalDurableDependencyRequired && strings.TrimSpace(p.DeduplicationBoundary) == "" {
		return errors.New("multi-region posture with durable dependencies must define deduplication_boundary")
	}
	return nil
}

// SignalClass describes the type of operational signal.
type SignalClass string

const (
	// SignalLatency marks latency-oriented operational signals.
	SignalLatency SignalClass = "latency"
	// SignalTraffic marks traffic-oriented operational signals.
	SignalTraffic SignalClass = "traffic"
	// SignalErrors marks error-oriented operational signals.
	SignalErrors SignalClass = "errors"
	// SignalSaturation marks saturation-oriented operational signals.
	SignalSaturation SignalClass = "saturation"
)

// SignalAttachment describes one golden-signal or capacity attachment point.
type SignalAttachment struct {
	Name        string      `json:"name"`
	Family      string      `json:"family"`
	Class       SignalClass `json:"class"`
	Metric      string      `json:"metric"`
	Description string      `json:"description,omitempty"`
}

// SignalCatalog stores runtime-visible signal attachment metadata.
type SignalCatalog struct {
	mu          sync.RWMutex
	attachments map[string]SignalAttachment
}

// NewSignalCatalog creates an empty signal catalog.
func NewSignalCatalog() *SignalCatalog {
	return &SignalCatalog{attachments: make(map[string]SignalAttachment)}
}

// Register adds or replaces one signal attachment.
func (c *SignalCatalog) Register(attachment SignalAttachment) {
	if c == nil {
		return
	}
	name := strings.TrimSpace(attachment.Name)
	if name == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.attachments[name] = attachment
}

// Snapshot returns the registered signal attachments.
func (c *SignalCatalog) Snapshot() any {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]SignalAttachment, 0, len(c.attachments))
	for _, attachment := range c.attachments {
		out = append(out, attachment)
	}
	return out
}
