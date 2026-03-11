// Package app provides the transport-agnostic application runtime lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/featureflag"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
)

const defaultShutdownTimeout = 30 * time.Second

// Phase identifies one step in the core application lifecycle.
type Phase string

const (
	// PhaseConfigResolution resolves runtime configuration before other startup work.
	PhaseConfigResolution Phase = "config_resolution"
	// PhaseObservabilityBaseline initializes shared logs, metrics, and tracing state.
	PhaseObservabilityBaseline Phase = "observability_baseline"
	// PhaseFeatureRegistration registers feature-owned startup contributions.
	PhaseFeatureRegistration Phase = "feature_registration"
	// PhaseHealthRegistration registers shared health checks and optional introspection entries.
	PhaseHealthRegistration Phase = "health_registration"
	// PhaseServiceConstruction constructs feature-owned runtime services.
	PhaseServiceConstruction Phase = "service_construction"
	// PhaseRuntimeExecution starts long-running workloads.
	PhaseRuntimeExecution Phase = "runtime_execution"
	// PhaseGracefulShutdown stops workloads and shutdown hooks in reverse order.
	PhaseGracefulShutdown Phase = "graceful_shutdown"
)

// Hook defines a named lifecycle action.
type Hook struct {
	Name string
	Fn   func(context.Context, *Runtime) error
}

// Runner defines one runtime workload owned by the application.
type Runner struct {
	Name string
	Fn   func(context.Context, *Runtime) error
}

// Runtime is the live shared state exposed to lifecycle phases and runners.
type Runtime struct {
	Name          string
	Config        any
	Debug         bool
	Flags         *featureflag.Registry
	Failures      *FailureInjector
	Deployment    *DeploymentPosture
	Signals       *SignalCatalog
	Logger        logger.Logger
	Health        *health.Registry
	Metrics       *metrics.Registry
	Tracer        *tracing.TracerProvider
	Introspection *IntrospectionRegistry
	Posture       *featureflag.RuntimePosture
	Services      map[string]any
}

// SetService stores a feature-owned runtime service by name.
func (r *Runtime) SetService(name string, service any) {
	if r.Services == nil {
		r.Services = make(map[string]any)
	}
	r.Services[name] = service
}

// Service returns a previously registered service by name.
func (r *Runtime) Service(name string) (any, bool) {
	if r.Services == nil {
		return nil, false
	}
	service, ok := r.Services[name]
	return service, ok
}

// IntrospectionRegistry stores framework-owned introspection data.
type IntrospectionRegistry struct {
	mu      sync.RWMutex
	entries map[string]any
}

// NewIntrospectionRegistry creates an empty introspection registry.
func NewIntrospectionRegistry() *IntrospectionRegistry {
	return &IntrospectionRegistry{
		entries: make(map[string]any),
	}
}

// Set stores an introspection entry.
func (r *IntrospectionRegistry) Set(name string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[name] = value
}

// Get returns one introspection entry by name.
func (r *IntrospectionRegistry) Get(name string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	value, ok := r.entries[name]
	return value, ok
}

// Snapshot returns a copy of the registered introspection data.
func (r *IntrospectionRegistry) Snapshot() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make(map[string]any, len(r.entries))
	for key, value := range r.entries {
		snapshot[key] = value
	}
	return snapshot
}

// Options configures the application lifecycle.
type Options struct {
	Name string

	Config   any
	Features []feature.Feature

	Logger                logger.Logger
	HealthRegistry        *health.Registry
	MetricsRegistry       *metrics.Registry
	TracerProvider        *tracing.TracerProvider
	IntrospectionRegistry *IntrospectionRegistry
	FeatureFlags          *featureflag.Registry
	RuntimePosture        *featureflag.RuntimePosture
	FailureInjector       *FailureInjector
	DeploymentPosture     *DeploymentPosture
	SignalCatalog         *SignalCatalog
	ShutdownTimeout       time.Duration
	Debug                 bool

	ConfigResolvers      []Hook
	ObservabilityHooks   []Hook
	FeatureRegistrations []Hook
	HealthRegistrations  []Hook
	InstrumentationHooks []Hook
	ServiceConstructors  []Hook
	Runners              []Runner
	ShutdownHooks        []Hook
}

// App owns application lifecycle orchestration and shared runtime state.
type App struct {
	runtime         *Runtime
	shutdownTimeout time.Duration

	configResolvers      []Hook
	observabilityHooks   []Hook
	featureRegistrations []Hook
	healthRegistrations  []Hook
	instrumentationHooks []Hook
	serviceConstructors  []Hook
	runners              []Runner
	shutdownHooks        []Hook
	introspectionHooks   []Hook
}

// New builds a transport-agnostic application runtime.
func New(opts Options) (*App, error) {
	log := opts.Logger
	if log == nil {
		defaultLogger, err := logger.NewZapLogger(logger.DefaultConfig())
		if err != nil {
			return nil, fmt.Errorf("create default logger: %w", err)
		}
		log = defaultLogger
	}

	healthRegistry := opts.HealthRegistry
	if healthRegistry == nil {
		healthRegistry = health.NewRegistry()
	}
	metricsRegistry := opts.MetricsRegistry
	if metricsRegistry == nil {
		metricsRegistry = metrics.NewRegistry()
	}
	flagRegistry := opts.FeatureFlags
	if flagRegistry == nil {
		flagRegistry = featureflag.NewRegistry()
	}
	introspectionRegistry := opts.IntrospectionRegistry
	if introspectionRegistry == nil {
		introspectionRegistry = NewIntrospectionRegistry()
	}
	posture := opts.RuntimePosture
	if posture == nil {
		posture = featureflag.NewRuntimePosture()
	}
	failures := opts.FailureInjector
	if failures == nil {
		failures = NewFailureInjector()
	}
	deploymentPosture := opts.DeploymentPosture
	if deploymentPosture == nil {
		deploymentPosture = NewDeploymentPosture()
	}
	signalCatalog := opts.SignalCatalog
	if signalCatalog == nil {
		signalCatalog = NewSignalCatalog()
	}
	signalCatalog.Register(SignalAttachment{
		Name:        "runtime-readiness",
		Family:      "runtime",
		Class:       SignalErrors,
		Metric:      featureflag.CheckNameRuntimeReadiness,
		Description: "Readiness posture exposed through the shared health registry.",
	})
	signalCatalog.Register(SignalAttachment{
		Name:        "runtime-liveness",
		Family:      "runtime",
		Class:       SignalErrors,
		Metric:      featureflag.CheckNameRuntimeLiveness,
		Description: "Liveness posture exposed through the shared health registry.",
	})
	signalCatalog.Register(SignalAttachment{
		Name:        "runtime-degraded-mode",
		Family:      "runtime",
		Class:       SignalSaturation,
		Metric:      featureflag.CheckNameRuntimePosture,
		Description: "Degraded-mode posture exposed through the shared health registry.",
	})
	featureflag.RegisterHealthChecks(healthRegistry, posture)

	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	featureContributions := collectFeatureContributions(opts.Features)

	return &App{
		runtime: &Runtime{
			Name:          opts.Name,
			Config:        opts.Config,
			Debug:         opts.Debug,
			Flags:         flagRegistry,
			Failures:      failures,
			Deployment:    deploymentPosture,
			Signals:       signalCatalog,
			Logger:        log,
			Health:        healthRegistry,
			Metrics:       metricsRegistry,
			Tracer:        opts.TracerProvider,
			Introspection: introspectionRegistry,
			Posture:       posture,
			Services:      make(map[string]any),
		},
		shutdownTimeout: shutdownTimeout,
		configResolvers: append([]Hook(nil), opts.ConfigResolvers...),
		observabilityHooks: append(
			append([]Hook(nil), opts.ObservabilityHooks...),
			adaptFeatureHooks(featureContributions.observabilityHooks)...,
		),
		featureRegistrations: append(
			append([]Hook(nil), opts.FeatureRegistrations...),
			adaptFeatureHooks(featureContributions.startupHooks)...,
		),
		healthRegistrations: append(
			append([]Hook(nil), opts.HealthRegistrations...),
			adaptFeatureHooks(featureContributions.healthHooks)...,
		),
		instrumentationHooks: append(
			append([]Hook(nil), opts.InstrumentationHooks...),
			adaptFeatureHooks(featureContributions.instrumentationHooks)...,
		),
		serviceConstructors: append(
			append([]Hook(nil), opts.ServiceConstructors...),
			adaptFeatureHooks(featureContributions.serviceHooks)...,
		),
		runners: append(
			append([]Runner(nil), opts.Runners...),
			adaptFeatureRunners(featureContributions.runners)...,
		),
		shutdownHooks: append(
			append([]Hook(nil), opts.ShutdownHooks...),
			adaptFeatureHooks(featureContributions.shutdownHooks)...,
		),
		introspectionHooks: adaptFeatureHooks(featureContributions.introspectionHooks),
	}, nil
}

// Runtime returns the shared runtime state owned by the application.
func (a *App) Runtime() *Runtime {
	return a.runtime
}

// Prepare executes startup phases up to service construction without starting runners.
func (a *App) Prepare(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.runtime.Posture.SetLiveness(featureflag.LivenessAlive, "")
	a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup in progress")
	a.runtime.Posture.SetStartup(featureflag.StartupStarting, "startup in progress")

	if err := a.runPhase(ctx, PhaseConfigResolution, a.configResolvers); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return err
	}
	if err := a.runPhase(ctx, PhaseObservabilityBaseline, a.observabilityHooks); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return err
	}
	if err := a.runPhase(ctx, PhaseFeatureRegistration, a.featureRegistrations); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return err
	}
	if err := a.runPhase(ctx, PhaseHealthRegistration, a.healthRegistrations); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return err
	}
	if a.runtime.Debug {
		if err := a.runPhase(ctx, PhaseHealthRegistration, a.introspectionHooks); err != nil {
			a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
			a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
			return err
		}
	}
	if err := a.runPhase(ctx, PhaseObservabilityBaseline, a.instrumentationHooks); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return err
	}
	if err := a.runPhase(ctx, PhaseServiceConstruction, a.serviceConstructors); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return err
	}
	if err := a.runtime.Deployment.Validate(); err != nil {
		a.runtime.Posture.SetStartup(featureflag.StartupFailed, err.Error())
		a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "startup failed")
		return fmt.Errorf("deployment posture validation failed: %w", err)
	}
	a.runtime.Posture.SetStartup(featureflag.StartupReady, "startup completed")
	if a.runtime.Posture.Snapshot().Mode == featureflag.ModeDegraded {
		a.runtime.Posture.SetReadiness(featureflag.ReadinessDegraded, "runtime is ready in degraded mode")
	} else {
		a.runtime.Posture.SetReadiness(featureflag.ReadinessReady, "runtime is ready")
	}
	if a.runtime.Debug {
		a.runtime.Introspection.Set("feature_flags", a.runtime.Flags.Snapshot())
		a.runtime.Introspection.Set("runtime_posture", a.runtime.Posture.Snapshot())
		a.runtime.Introspection.Set("failure_injection", a.runtime.Failures.Snapshot())
		a.runtime.Introspection.Set("deployment_posture", a.runtime.Deployment.Snapshot())
		a.runtime.Introspection.Set("signal_catalog", a.runtime.Signals.Snapshot())
	}
	return nil
}

// Run executes the configured lifecycle phases and runtime workloads.
func (a *App) Run(ctx context.Context) error {
	if err := a.Prepare(ctx); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runErr := a.runRunners(runCtx, cancel)
	shutdownErr := a.shutdown(ctx)
	return errors.Join(runErr, shutdownErr)
}

// AppName returns the application name visible to feature contributions.
func (r *Runtime) AppName() string {
	return r.Name
}

// ConfigValue returns the effective runtime config object.
func (r *Runtime) ConfigValue() any {
	return r.Config
}

// DebugEnabled reports whether framework debug surfaces are enabled.
func (r *Runtime) DebugEnabled() bool {
	return r.Debug
}

// Log returns the runtime logger.
func (r *Runtime) Log() logger.Logger {
	return r.Logger
}

// FeatureFlags returns the shared feature flag registry.
func (r *Runtime) FeatureFlags() *featureflag.Registry {
	return r.Flags
}

// RuntimePosture returns the shared runtime posture contract.
func (r *Runtime) RuntimePosture() *featureflag.RuntimePosture {
	return r.Posture
}

// FailureInjector returns the shared failure-injection registry.
func (r *Runtime) FailureInjector() feature.FailureInjector {
	return r.Failures
}

// DeploymentPosture returns the shared deployment posture metadata.
func (r *Runtime) DeploymentPosture() feature.DeploymentPosture {
	return r.Deployment
}

// SignalCatalog returns the shared operational signal catalog.
func (r *Runtime) SignalCatalog() feature.SignalCatalog {
	return r.Signals
}

// HealthRegistry returns the shared health registry.
func (r *Runtime) HealthRegistry() *health.Registry {
	return r.Health
}

// MetricsRegistry returns the shared metrics registry.
func (r *Runtime) MetricsRegistry() *metrics.Registry {
	return r.Metrics
}

// TracerProvider returns the shared tracer provider when present.
func (r *Runtime) TracerProvider() *tracing.TracerProvider {
	return r.Tracer
}

// IntrospectionRegistry returns the shared introspection registry.
func (r *Runtime) IntrospectionRegistry() feature.IntrospectionRegistry {
	return r.Introspection
}

// RegisterService stores a feature-owned runtime service by name.
func (r *Runtime) RegisterService(name string, service any) {
	r.SetService(name, service)
}

// LookupService returns a previously registered runtime service.
func (r *Runtime) LookupService(name string) (any, bool) {
	return r.Service(name)
}

func (a *App) runPhase(ctx context.Context, phase Phase, hooks []Hook) error {
	a.runtime.Logger.Info("application phase started", "phase", phase)
	defer a.runtime.Logger.Info("application phase completed", "phase", phase)

	for _, hook := range hooks {
		if hook.Fn == nil {
			continue
		}
		if err := a.runtime.Failures.Apply(ctx, HookFailureTarget(phase, hook.Name)); err != nil {
			return fmt.Errorf("%s failure injection for hook %q failed: %w", phase, hook.Name, err)
		}
		if err := hook.Fn(ctx, a.runtime); err != nil {
			return fmt.Errorf("%s hook %q failed: %w", phase, hook.Name, err)
		}
	}

	return nil
}

func (a *App) runRunners(ctx context.Context, cancel context.CancelFunc) error {
	a.runtime.Logger.Info("application phase started", "phase", PhaseRuntimeExecution)
	defer a.runtime.Logger.Info("application phase completed", "phase", PhaseRuntimeExecution)

	if len(a.runners) == 0 {
		return nil
	}

	errCh := make(chan error, len(a.runners))
	for _, runner := range a.runners {
		go func() {
			if runner.Fn == nil {
				errCh <- nil
				return
			}
			if err := a.runtime.Failures.Apply(ctx, RunnerFailureTarget(runner.Name)); err != nil {
				errCh <- fmt.Errorf("runner %q failure injection failed: %w", runner.Name, err)
				return
			}
			errCh <- runner.Fn(ctx, a.runtime)
		}()
	}

	var firstErr error
	for range a.runners {
		err := <-errCh
		if err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}

	return firstErr
}

func (a *App) shutdown(ctx context.Context) error {
	a.runtime.Logger.Info("application phase started", "phase", PhaseGracefulShutdown)
	defer a.runtime.Logger.Info("application phase completed", "phase", PhaseGracefulShutdown)
	a.runtime.Posture.SetReadiness(featureflag.ReadinessBlocked, "graceful shutdown in progress")
	a.runtime.Posture.SetLiveness(featureflag.LivenessStopping, "graceful shutdown in progress")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				cancel()
			case <-shutdownCtx.Done():
			}
		}()
	}

	var shutdownErr error
	for idx := len(a.shutdownHooks) - 1; idx >= 0; idx-- {
		hook := a.shutdownHooks[idx]
		if hook.Fn == nil {
			continue
		}
		if err := a.runtime.Failures.Apply(shutdownCtx, ShutdownFailureTarget(hook.Name)); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("%s failure injection for hook %q failed: %w", PhaseGracefulShutdown, hook.Name, err))
			continue
		}
		if err := hook.Fn(shutdownCtx, a.runtime); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("%s hook %q failed: %w", PhaseGracefulShutdown, hook.Name, err))
		}
	}

	if tracer := a.runtime.Tracer; tracer != nil {
		if err := tracer.Shutdown(shutdownCtx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("shutdown tracer provider: %w", err))
		}
	}
	a.runtime.Posture.SetLiveness(featureflag.LivenessStopped, "runtime stopped")

	return shutdownErr
}

type collectedFeatureContributions struct {
	observabilityHooks   []feature.Hook
	startupHooks         []feature.Hook
	healthHooks          []feature.Hook
	instrumentationHooks []feature.Hook
	serviceHooks         []feature.Hook
	runners              []feature.Runner
	shutdownHooks        []feature.Hook
	introspectionHooks   []feature.Hook
}

func collectFeatureContributions(features []feature.Feature) collectedFeatureContributions {
	var collected collectedFeatureContributions

	for _, currentFeature := range features {
		if currentFeature == nil {
			continue
		}
		contributions := currentFeature.Contributions()
		collected.observabilityHooks = append(collected.observabilityHooks, contributions.ObservabilityHooks...)
		collected.startupHooks = append(collected.startupHooks, contributions.StartupHooks...)
		collected.healthHooks = append(collected.healthHooks, contributions.HealthContributors...)
		collected.instrumentationHooks = append(collected.instrumentationHooks, contributions.InstrumentationHooks...)
		collected.serviceHooks = append(collected.serviceHooks, contributions.ServiceConstructors...)
		collected.runners = append(collected.runners, contributions.RuntimeRunners...)
		collected.shutdownHooks = append(collected.shutdownHooks, contributions.ShutdownHooks...)
		collected.introspectionHooks = append(collected.introspectionHooks, contributions.IntrospectionContributors...)
	}

	return collected
}

func adaptFeatureHooks(hooks []feature.Hook) []Hook {
	if len(hooks) == 0 {
		return nil
	}

	adapted := make([]Hook, 0, len(hooks))
	for _, hook := range hooks {
		adapted = append(adapted, Hook{
			Name: hook.Name,
			Fn: func(ctx context.Context, runtime *Runtime) error {
				if hook.Fn == nil {
					return nil
				}
				return hook.Fn(ctx, runtime)
			},
		})
	}
	return adapted
}

func adaptFeatureRunners(runners []feature.Runner) []Runner {
	if len(runners) == 0 {
		return nil
	}

	adapted := make([]Runner, 0, len(runners))
	for _, runner := range runners {
		adapted = append(adapted, Runner{
			Name: runner.Name,
			Fn: func(ctx context.Context, runtime *Runtime) error {
				if runner.Fn == nil {
					return nil
				}
				return runner.Fn(ctx, runtime)
			},
		})
	}
	return adapted
}
