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
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/factory"
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

	// PublicRouter is optional. If nil, a router will be created based on Config.RouterType.
	PublicRouter router.Router
	// ManagementRouter is optional. If nil and management is enabled, a router will be created.
	ManagementRouter router.Router

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

	// Create public router if not provided
	if opts.PublicRouter == nil {
		r, err := factory.NewRouter(opts.Config.RouterType)
		if err != nil {
			return nil, fmt.Errorf("create public router: %w", err)
		}
		opts.PublicRouter = r
	}

	publicServer := NewPublicAPIServerWithConfig(
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
		opts.PublicRouter,
		opts.Logger,
	)

	servers := &HTTPServers{Public: publicServer}
	if !opts.Config.Management.Enabled {
		return servers, nil
	}

	// Create management router if not provided
	if opts.ManagementRouter == nil {
		r, err := factory.NewRouter(opts.Config.RouterType)
		if err != nil {
			return nil, fmt.Errorf("create management router: %w", err)
		}
		opts.ManagementRouter = r
	}
	versionInfo := version.Current(resolveServiceName(opts))
	registerVersionEndpoint(opts.ManagementRouter, versionInfo)

	healthRegistry := opts.HealthRegistry
	if healthRegistry == nil {
		healthRegistry = health.NewRegistry()
	}
	metricsRegistry := opts.MetricsRegistry
	if metricsRegistry == nil {
		metricsRegistry = metrics.NewRegistry()
	}

	managementServer, err := NewManagementServer(
		opts.Config.Management,
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
				Fn: func(ctx context.Context, runtime *coreapp.Runtime) error {
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
		if trimmed := strings.TrimSpace(opts.Config.Service.Name); trimmed != "" {
			return trimmed
		}
	}
	return version.Unknown
}

func resolveEnvironment(opts *RunHTTPServersOptions) string {
	if opts.Config != nil {
		return normalizeEnvironment(opts.Config.Service.Environment)
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

func runStartupHooks(ctx context.Context, opts *RunHTTPServersOptions) error {
	lifecycleApp, err := coreapp.New(coreapp.Options{
		Logger:               opts.Logger,
		FeatureRegistrations: toCoreHooks(opts.StartupHooks),
	})
	if err != nil {
		return err
	}
	return lifecycleApp.Run(ctx)
}

func runShutdownHooks(opts *RunHTTPServersOptions) error {
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
	return lifecycleApp.Run(context.Background())
}

func toCoreHooks(hooks []LifecycleHook) []coreapp.Hook {
	if len(hooks) == 0 {
		return nil
	}

	coreHooks := make([]coreapp.Hook, 0, len(hooks))
	for _, hook := range hooks {
		currentHook := hook
		coreHooks = append(coreHooks, coreapp.Hook{
			Name: currentHook.Name,
			Fn: func(ctx context.Context, runtime *coreapp.Runtime) error {
				name := strings.TrimSpace(currentHook.Name)
				if name == "" {
					name = "unnamed"
				}
				if currentHook.Fn == nil {
					return nil
				}
				return currentHook.Fn(ctx)
			},
		})
	}
	return coreHooks
}

func serverRunners(servers *HTTPServers) []coreapp.Runner {
	runners := []coreapp.Runner{
		{
			Name: "http_public",
			Fn: func(ctx context.Context, runtime *coreapp.Runtime) error {
				return servers.Public.Start(ctx)
			},
		},
	}
	if servers.Management != nil {
		runners = append(runners, coreapp.Runner{
			Name: "http_management",
			Fn: func(ctx context.Context, runtime *coreapp.Runtime) error {
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
