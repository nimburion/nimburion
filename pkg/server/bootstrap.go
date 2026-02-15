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
	if shouldShutdownTracer {
		defer shutdownTracerProvider(tracerProvider, opts.Logger)
	}

	if err := runStartupHooks(ctx, opts); err != nil {
		return err
	}
	defer func() {
		if shutdownErr := runShutdownHooks(opts); shutdownErr != nil {
			opts.Logger.Error("shutdown hooks completed with errors", "error", shutdownErr)
		}
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	serverCount := 1
	if servers.Management != nil {
		serverCount = 2
	}

	errCh := make(chan error, serverCount)
	go func() { errCh <- servers.Public.Start(runCtx) }()
	if servers.Management != nil {
		go func() { errCh <- servers.Management.Start(runCtx) }()
	}

	var firstErr error
	for idx := 0; idx < serverCount; idx++ {
		currentErr := <-errCh
		if currentErr != nil && firstErr == nil {
			firstErr = currentErr
			cancel()
		}
	}
	return firstErr
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

func shutdownTracerProvider(provider *tracing.TracerProvider, log logger.Logger) {
	if provider == nil {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := provider.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown tracing provider", "error", err)
	}
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
	for _, hook := range opts.StartupHooks {
		if hook.Fn == nil {
			continue
		}
		name := strings.TrimSpace(hook.Name)
		if name == "" {
			name = "unnamed"
		}
		opts.Logger.Info("startup hook start", "hook", name)
		if err := hook.Fn(ctx); err != nil {
			opts.Logger.Error("startup hook failed", "hook", name, "error", err)
			return fmt.Errorf("startup hook %q failed: %w", name, err)
		}
		opts.Logger.Info("startup hook complete", "hook", name)
	}
	return nil
}

func runShutdownHooks(opts *RunHTTPServersOptions) error {
	if len(opts.ShutdownHooks) == 0 {
		return nil
	}

	timeout := opts.ShutdownHookTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	var errs []error
	for _, hook := range opts.ShutdownHooks {
		if hook.Fn == nil {
			continue
		}
		name := strings.TrimSpace(hook.Name)
		if name == "" {
			name = "unnamed"
		}
		opts.Logger.Info("shutdown hook start", "hook", name)

		hookCtx, cancel := context.WithTimeout(context.Background(), timeout)
		err := hook.Fn(hookCtx)
		cancel()

		if err != nil {
			opts.Logger.Error("shutdown hook failed", "hook", name, "error", err)
			errs = append(errs, fmt.Errorf("shutdown hook %q failed: %w", name, err))
			continue
		}
		opts.Logger.Info("shutdown hook complete", "hook", name)
	}
	return errors.Join(errs...)
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
