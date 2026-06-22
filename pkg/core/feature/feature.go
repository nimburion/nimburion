// Package feature defines feature contribution contracts for the core runtime.
package feature

import (
	"context"

	"github.com/nimburion/nimburion/pkg/featureflag"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
)

// Runtime exposes the runtime surface that features may contribute to.
type Runtime interface {
	AppName() string
	ConfigValue() any
	DebugEnabled() bool
	Log() logger.Logger
	FeatureFlags() *featureflag.Registry
	RuntimePosture() *featureflag.RuntimePosture
	FailureInjector() FailureInjector
	DeploymentPosture() DeploymentPosture
	SignalCatalog() SignalCatalog
	HealthRegistry() *health.Registry
	MetricsRegistry() *metrics.Registry
	TracerProvider() *tracing.TracerProvider
	IntrospectionRegistry() IntrospectionRegistry
	RegisterService(name string, service any)
	LookupService(name string) (any, bool)
}

// IntrospectionRegistry stores framework introspection entries.
type IntrospectionRegistry interface {
	Set(name string, value any)
	Get(name string) (any, bool)
	Snapshot() map[string]any
}

// FailureInjector exposes opt-in runtime failure injection.
type FailureInjector interface {
	Apply(context.Context, string) error
	Snapshot() any
}

// DeploymentPosture exposes deployment topology metadata.
type DeploymentPosture interface {
	Validate() error
	Snapshot() any
}

// SignalCatalog exposes runtime signal attachment metadata.
type SignalCatalog interface {
	Snapshot() any
}

// Hook defines a named feature lifecycle action.
type Hook struct {
	Name string
	Fn   func(context.Context, Runtime) error
}

// Runner defines one feature-owned runtime workload.
type Runner struct {
	Name string
	Fn   func(context.Context, Runtime) error
}

// CommandContribution describes a feature-owned CLI command contribution.
type CommandContribution struct {
	Name    string
	Command any
}

// ConfigExtension describes one feature-owned config contribution.
type ConfigExtension struct {
	Name      string
	Extension any
}

// Contributions groups all extension points exposed by one feature.
type Contributions struct {
	ConfigExtensions          []ConfigExtension
	CommandRegistrations      []CommandContribution
	ObservabilityHooks        []Hook
	StartupHooks              []Hook
	HealthContributors        []Hook
	InstrumentationHooks      []Hook
	ServiceConstructors       []Hook
	RuntimeRunners            []Runner
	ShutdownHooks             []Hook
	IntrospectionContributors []Hook
}

// Feature contributes runtime behavior without editing the base app.
type Feature interface {
	Name() string
	Contributions() Contributions
}

// The interfaces below are optional, focused alternatives to the broad
// Contributions struct. A Feature MAY implement any subset of them to expose
// only the categories it actually contributes, instead of constructing a full
// Contributions value.
//
// Merge semantics (see app.collectFeatureContributions): for each category the
// value returned by Contributions() wins when it is non-empty. The matching
// optional interface is consulted ONLY when Contributions() leaves that
// category empty (the zero value). This makes the two mechanisms a clean
// fallback rather than additive, so a feature can never double-count the same
// contribution by exposing it through both paths.

// ConfigExtensionProvider optionally contributes config extensions.
type ConfigExtensionProvider interface {
	ConfigExtensions() []ConfigExtension
}

// CommandProvider optionally contributes CLI command registrations.
type CommandProvider interface {
	CommandRegistrations() []CommandContribution
}

// ObservabilityHookProvider optionally contributes observability hooks.
type ObservabilityHookProvider interface {
	ObservabilityHooks() []Hook
}

// StartupHookProvider optionally contributes startup hooks.
type StartupHookProvider interface {
	StartupHooks() []Hook
}

// HealthContributorProvider optionally contributes health contributors.
type HealthContributorProvider interface {
	HealthContributors() []Hook
}

// InstrumentationHookProvider optionally contributes instrumentation hooks.
type InstrumentationHookProvider interface {
	InstrumentationHooks() []Hook
}

// ServiceConstructorProvider optionally contributes service constructors.
type ServiceConstructorProvider interface {
	ServiceConstructors() []Hook
}

// RunnerProvider optionally contributes runtime runners.
type RunnerProvider interface {
	RuntimeRunners() []Runner
}

// ShutdownHookProvider optionally contributes shutdown hooks.
type ShutdownHookProvider interface {
	ShutdownHooks() []Hook
}

// IntrospectionProvider optionally contributes introspection contributors.
type IntrospectionProvider interface {
	IntrospectionContributors() []Hook
}

// DependencyDeclaring optionally declares the names of other features that must
// be collected (and therefore initialized) before this one. Features that do
// not implement it keep their original registration order.
type DependencyDeclaring interface {
	DependsOn() []string
}
