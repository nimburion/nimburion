// Package app provides the transport-agnostic application runtime lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
)

const defaultShutdownTimeout = 30 * time.Second

// Phase identifies one step in the core application lifecycle.
type Phase string

const (
	PhaseConfigResolution      Phase = "config_resolution"
	PhaseObservabilityBaseline Phase = "observability_baseline"
	PhaseFeatureRegistration   Phase = "feature_registration"
	PhaseHealthRegistration    Phase = "health_registration"
	PhaseServiceConstruction   Phase = "service_construction"
	PhaseRuntimeExecution      Phase = "runtime_execution"
	PhaseGracefulShutdown      Phase = "graceful_shutdown"
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
	Logger        logger.Logger
	Health        *health.Registry
	Metrics       *metrics.Registry
	Tracer        *tracing.TracerProvider
	Introspection *IntrospectionRegistry
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
	ShutdownTimeout       time.Duration

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
	introspectionRegistry := opts.IntrospectionRegistry
	if introspectionRegistry == nil {
		introspectionRegistry = NewIntrospectionRegistry()
	}

	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	featureContributions := collectFeatureContributions(opts.Features)

	return &App{
		runtime: &Runtime{
			Name:          opts.Name,
			Config:        opts.Config,
			Logger:        log,
			Health:        healthRegistry,
			Metrics:       metricsRegistry,
			Tracer:        opts.TracerProvider,
			Introspection: introspectionRegistry,
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
			append(
				append([]Hook(nil), opts.HealthRegistrations...),
				adaptFeatureHooks(featureContributions.healthHooks)...,
			),
			adaptFeatureHooks(featureContributions.introspectionHooks)...,
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
	}, nil
}

// Runtime returns the shared runtime state owned by the application.
func (a *App) Runtime() *Runtime {
	return a.runtime
}

// Run executes the configured lifecycle phases and runtime workloads.
func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := a.runPhase(ctx, PhaseConfigResolution, a.configResolvers); err != nil {
		return err
	}
	if err := a.runPhase(ctx, PhaseObservabilityBaseline, a.observabilityHooks); err != nil {
		return err
	}
	if err := a.runPhase(ctx, PhaseFeatureRegistration, a.featureRegistrations); err != nil {
		return err
	}
	if err := a.runPhase(ctx, PhaseHealthRegistration, a.healthRegistrations); err != nil {
		return err
	}
	if err := a.runPhase(ctx, PhaseObservabilityBaseline, a.instrumentationHooks); err != nil {
		return err
	}
	if err := a.runPhase(ctx, PhaseServiceConstruction, a.serviceConstructors); err != nil {
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

// Log returns the runtime logger.
func (r *Runtime) Log() logger.Logger {
	return r.Logger
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
		currentRunner := runner
		go func() {
			if currentRunner.Fn == nil {
				errCh <- nil
				return
			}
			errCh <- currentRunner.Fn(ctx, a.runtime)
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
		if err := hook.Fn(shutdownCtx, a.runtime); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("%s hook %q failed: %w", PhaseGracefulShutdown, hook.Name, err))
		}
	}

	if tracer := a.runtime.Tracer; tracer != nil {
		if err := tracer.Shutdown(shutdownCtx); err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("shutdown tracer provider: %w", err))
		}
	}

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
		currentHook := hook
		adapted = append(adapted, Hook{
			Name: currentHook.Name,
			Fn: func(ctx context.Context, runtime *Runtime) error {
				if currentHook.Fn == nil {
					return nil
				}
				return currentHook.Fn(ctx, runtime)
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
		currentRunner := runner
		adapted = append(adapted, Runner{
			Name: currentRunner.Name,
			Fn: func(ctx context.Context, runtime *Runtime) error {
				if currentRunner.Fn == nil {
					return nil
				}
				return currentRunner.Fn(ctx, runtime)
			},
		})
	}
	return adapted
}
