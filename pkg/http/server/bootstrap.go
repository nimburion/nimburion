package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/config"
	coreapp "github.com/nimburion/nimburion/pkg/core/app"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/http/openapi"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
	"github.com/nimburion/nimburion/pkg/version"
)

// LifecycleHook defines a named startup/shutdown action.
type LifecycleHook struct {
	Name string
	Fn   func(context.Context) error
}

// RunHTTPServersOptions defines inputs for building/running framework HTTP servers.
type RunHTTPServersOptions struct {
	Config *config.Config

	PublicRouter     router.Router
	ManagementRouter router.Router
	RegisterRoutes   func(router.Router, *config.Config)
	DescribeRoutes   func(router.Router, *config.Config)

	Logger logger.Logger

	HealthRegistry  *health.Registry
	MetricsRegistry *metrics.Registry

	ManagementJWTValidator auth.JWTValidator

	StartupHooks        []LifecycleHook
	ShutdownHooks       []LifecycleHook
	ShutdownHookTimeout time.Duration
}

// NewDefaultRunHTTPServersOptions creates default options for running HTTP servers
func NewDefaultRunHTTPServersOptions() *RunHTTPServersOptions {
	return &RunHTTPServersOptions{
		Config:                 nil,
		PublicRouter:           nil,
		ManagementRouter:       nil,
		RegisterRoutes:         nil,
		DescribeRoutes:         nil,
		Logger:                 nil,
		HealthRegistry:         nil,
		MetricsRegistry:        nil,
		ManagementJWTValidator: nil,
		StartupHooks:           nil,
		ShutdownHooks:          nil,
		ShutdownHookTimeout:    0,
	}
}

// HTTPServers groups the runtime public/management servers.
type HTTPServers struct {
	Public     *PublicAPIServer
	Management *ManagementServer
}

// BuildHTTPServers constructs framework HTTP servers from config/options.
func BuildHTTPServers(opts *RunHTTPServersOptions) (*HTTPServers, error) {
	if opts == nil {
		return nil, errors.New("options are required")
	}
	if opts.Config == nil {
		opts.Config = config.DefaultConfig()
	}
	if opts.Logger == nil {
		httpLogger, err := logger.NewZapLogger(logger.DefaultConfig())
		if err != nil {
			return nil, err
		}
		opts.Logger = httpLogger
	}

	if opts.PublicRouter == nil {
		return nil, errors.New("public router is required")
	}
	healthRegistry := opts.HealthRegistry
	if healthRegistry == nil {
		healthRegistry = health.NewRegistry()
	}
	metricsRegistry := opts.MetricsRegistry
	if metricsRegistry == nil {
		metricsRegistry = metrics.NewRegistry()
	}

	publicServer := newPublicAPIServerWithDependencies(
		opts.Config.HTTP,
		opts.Config.CORS,
		opts.Config.SecurityHeaders,
		opts.Config.Security,
		opts.Config.I18n,
		opts.Config.Session,
		opts.Config.CSRF,
		opts.Config.SSE,
		opts.Config.EventBus,
		opts.Config.Validation,
		opts.Config.Observability,
		metricsRegistry,
		opts.PublicRouter,
		opts.Logger,
	)
	if publicServer.startErr != nil {
		return nil, fmt.Errorf("create public server: %w", publicServer.startErr)
	}
	if opts.RegisterRoutes != nil {
		opts.RegisterRoutes(opts.PublicRouter, opts.Config)
	}

	servers := &HTTPServers{Public: publicServer}
	if !opts.Config.Management.Enabled {
		return servers, nil
	}

	if opts.ManagementRouter == nil {
		return nil, errors.New("management router is required when management server is enabled")
	}
	versionInfo := version.Current(resolveServiceName(opts))
	registerVersionEndpoint(opts.ManagementRouter, versionInfo)
	provisionGeneratedOpenAPISpec(opts, versionInfo)

	managementServer, err := NewManagementServerWithSwagger(
		opts.Config.Management,
		opts.Config.Swagger,
		opts.ManagementRouter,
		opts.Logger,
		healthRegistry,
		metricsRegistry,
		opts.ManagementJWTValidator,
	)
	if err != nil {
		return nil, fmt.Errorf("create management server: %w", err)
	}
	servers.Management = managementServer
	return servers, nil
}

func provisionGeneratedOpenAPISpec(opts *RunHTTPServersOptions, versionInfo version.Info) {
	if opts == nil || opts.Config == nil || opts.ManagementRouter == nil {
		return
	}
	if !opts.Config.Swagger.Enabled || !opts.Config.Swagger.ProvisionGeneratedSpec {
		return
	}
	if opts.DescribeRoutes == nil {
		if opts.Logger != nil {
			opts.Logger.Warn("swagger.provision_generated_spec enabled but describe routes callback is missing; skipping generated spec provisioning")
		}
		return
	}

	routes := openapi.CollectRoutes(func(r router.Router) {
		opts.DescribeRoutes(r, opts.Config)
	})
	if opts.Logger != nil {
		for _, route := range routes {
			opts.Logger.Debug("collected openapi route", "method", route.Method, "path", route.Path)
		}
		opts.Logger.Debug("collected openapi routes", "count", len(routes))
	}
	spec := openapi.BuildSpec(resolveServiceName(opts), versionInfo.Version, routes)

	tempFile, err := os.CreateTemp("", "nimburion-openapi-*.yaml")
	if err != nil {
		if opts.Logger != nil {
			opts.Logger.Error("failed to create temp openapi spec file", "error", err)
		}
		return
	}
	if closeErr := tempFile.Close(); closeErr != nil {
		if opts.Logger != nil {
			opts.Logger.Error("failed to close temp openapi spec file", "error", closeErr)
		}
		return
	}
	if err := openapi.WriteSpec(tempFile.Name(), spec); err != nil {
		if opts.Logger != nil {
			opts.Logger.Error("failed to write generated openapi spec", "error", err)
		}
		return
	}

	openapi.NewHandler(tempFile.Name()).RegisterRoutes(opts.ManagementRouter)
}

// RunHTTPServers starts public server and (optionally) management server.
func RunHTTPServers(ctx context.Context, servers *HTTPServers, opts *RunHTTPServersOptions) error {
	if servers == nil || servers.Public == nil {
		return errors.New("servers and public server are required")
	}
	if opts == nil {
		return errors.New("options are required")
	}
	if opts.Logger == nil {
		return errors.New("logger is required")
	}
	if opts.Config == nil {
		return errors.New("config is required")
	}

	versionInfo := version.Current(resolveServiceName(opts))
	opts.Logger.Info("application version metadata",
		"service", versionInfo.Service,
		"version", versionInfo.Version,
		"commit", versionInfo.Commit,
		"build_time", versionInfo.BuildTime,
	)

	tracerProvider, shouldShutdownTracer, err := initTracerProvider(ctx, opts, versionInfo)
	if err != nil {
		return fmt.Errorf("initialize tracing provider: %w", err)
	}

	lifecycleApp, err := coreapp.New(coreapp.Options{
		Name:            versionInfo.Service,
		Config:          opts.Config,
		Logger:          opts.Logger,
		HealthRegistry:  opts.HealthRegistry,
		MetricsRegistry: opts.MetricsRegistry,
		TracerProvider: func() *tracing.TracerProvider {
			if shouldShutdownTracer {
				return tracerProvider
			}
			return nil
		}(),
		ShutdownTimeout: opts.ShutdownHookTimeout,
		ObservabilityHooks: []coreapp.Hook{
			{
				Name: "version_metadata",
				Fn: func(_ context.Context, runtime *coreapp.Runtime) error {
					runtime.Logger.Info("application version metadata",
						"service", versionInfo.Service,
						"version", versionInfo.Version,
						"commit", versionInfo.Commit,
						"build_time", versionInfo.BuildTime,
					)
					return nil
				},
			},
		},
		FeatureRegistrations: toCoreHooks(opts.StartupHooks),
		Runners:              serverRunners(servers),
		ShutdownHooks:        toCoreHooks(opts.ShutdownHooks),
	})
	if err != nil {
		return fmt.Errorf("create lifecycle app: %w", err)
	}

	return lifecycleApp.Run(ctx)
}

func registerVersionEndpoint(r router.Router, info version.Info) {
	r.GET("/version", func(c router.Context) error {
		return c.JSON(200, info)
	})
}

func initTracerProvider(ctx context.Context, opts *RunHTTPServersOptions, info version.Info) (*tracing.TracerProvider, bool, error) {
	tracerCfg := tracing.TracerConfig{
		ServiceName:    resolveTracingServiceName(opts, info.Service),
		ServiceVersion: info.Version,
		Environment:    resolveEnvironment(opts),
		Endpoint:       opts.Config.Observability.TracingEndpoint,
		SampleRate:     opts.Config.Observability.TracingSampleRate,
		Enabled:        opts.Config.Observability.TracingEnabled,
	}

	provider, err := tracing.NewTracerProvider(ctx, tracerCfg)
	if err != nil {
		return nil, false, err
	}
	return provider, true, nil
}

func normalizeEnvironment(env string) string {
	trimmed := strings.TrimSpace(env)
	if trimmed == "" {
		return version.Unknown
	}
	return trimmed
}

func resolveServiceName(opts *RunHTTPServersOptions) string {
	if opts.Config != nil {
		if trimmed := strings.TrimSpace(opts.Config.App.Name); trimmed != "" {
			return trimmed
		}
	}
	return version.Unknown
}

func resolveEnvironment(opts *RunHTTPServersOptions) string {
	if opts.Config != nil {
		return normalizeEnvironment(opts.Config.App.Environment)
	}
	return version.Unknown
}

func resolveTracingServiceName(opts *RunHTTPServersOptions, fallback string) string {
	if opts != nil && opts.Config != nil {
		if trimmed := strings.TrimSpace(opts.Config.Observability.ServiceName); trimmed != "" {
			return trimmed
		}
	}
	if trimmed := strings.TrimSpace(fallback); trimmed != "" {
		return trimmed
	}
	return version.Unknown
}

func runShutdownHooks(ctx context.Context, opts *RunHTTPServersOptions) error {
	timeout := opts.ShutdownHookTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	lifecycleApp, err := coreapp.New(coreapp.Options{
		Logger:          opts.Logger,
		ShutdownTimeout: timeout,
		ShutdownHooks:   toCoreHooks(opts.ShutdownHooks),
	})
	if err != nil {
		return err
	}
	shutdownCtx, cancel := shutdownContextFromParent(ctx)
	defer cancel()
	return lifecycleApp.Run(shutdownCtx)
}

func toCoreHooks(hooks []LifecycleHook) []coreapp.Hook {
	if len(hooks) == 0 {
		return nil
	}

	coreHooks := make([]coreapp.Hook, 0, len(hooks))
	for _, hook := range hooks {
		coreHooks = append(coreHooks, coreapp.Hook{
			Name: hook.Name,
			Fn: func(ctx context.Context, _ *coreapp.Runtime) error {
				if hook.Fn == nil {
					return nil
				}
				return hook.Fn(ctx)
			},
		})
	}
	return coreHooks
}

func serverRunners(servers *HTTPServers) []coreapp.Runner {
	runners := []coreapp.Runner{
		{
			Name: "http_public",
			Fn: func(ctx context.Context, _ *coreapp.Runtime) error {
				return servers.Public.Start(ctx)
			},
		},
	}
	if servers.Management != nil {
		runners = append(runners, coreapp.Runner{
			Name: "http_management",
			Fn: func(ctx context.Context, _ *coreapp.Runtime) error {
				return servers.Management.Start(ctx)
			},
		})
	}
	return runners
}

// RunHTTPServersWithSignals runs servers with centralized signal handling.
func RunHTTPServersWithSignals(servers *HTTPServers, opts *RunHTTPServersOptions, signals ...os.Signal) error {
	if len(signals) == 0 {
		signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}
	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()
	return RunHTTPServers(ctx, servers, opts)
}
