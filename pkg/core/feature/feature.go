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
