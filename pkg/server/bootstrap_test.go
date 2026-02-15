package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
	"github.com/nimburion/nimburion/pkg/version"
)

func TestBuildHTTPServers_ManagementEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Management.Enabled = true

	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	servers, err := BuildHTTPServers(&RunHTTPServersOptions{
		Config:           cfg,
		PublicRouter:     nethttp.NewRouter(),
		ManagementRouter: nethttp.NewRouter(),
		Logger:           log,
		HealthRegistry:   health.NewRegistry(),
		MetricsRegistry:  metrics.NewRegistry(),
	})
	if err != nil {
		t.Fatalf("build servers: %v", err)
	}
	if servers.Public == nil {
		t.Fatalf("expected public server")
	}
	if servers.Management == nil {
		t.Fatalf("expected management server")
	}
}

func TestBuildHTTPServers_RegistersVersionEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Management.Enabled = true
	cfg.Service.Name = "orders-from-config"

	oldVersion := version.AppVersion
	oldCommit := version.GitCommit
	oldBuildTime := version.BuildTime
	t.Cleanup(func() {
		version.AppVersion = oldVersion
		version.GitCommit = oldCommit
		version.BuildTime = oldBuildTime
	})
	version.AppVersion = "v1.2.3"
	version.GitCommit = "abc1234"
	version.BuildTime = "2026-02-20T10:00:00Z"

	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	publicRouter := nethttp.NewRouter()
	managementRouter := nethttp.NewRouter()
	_, err = BuildHTTPServers(&RunHTTPServersOptions{
		Config:           cfg,
		PublicRouter:     publicRouter,
		ManagementRouter: managementRouter,
		Logger:           log,
	})
	if err != nil {
		t.Fatalf("build servers: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	managementRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var got version.Info
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode version response: %v", err)
	}

	if got.Service != "orders-from-config" {
		t.Fatalf("expected service orders-from-config, got %s", got.Service)
	}
	if got.Version != "v1.2.3" {
		t.Fatalf("expected version v1.2.3, got %s", got.Version)
	}
	if got.Commit != "abc1234" {
		t.Fatalf("expected commit abc1234, got %s", got.Commit)
	}
	if got.BuildTime != "2026-02-20T10:00:00Z" {
		t.Fatalf("expected build_time 2026-02-20T10:00:00Z, got %s", got.BuildTime)
	}
}

func TestBuildHTTPServers_ManagementDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Management.Enabled = false

	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	servers, err := BuildHTTPServers(&RunHTTPServersOptions{
		Config:       cfg,
		PublicRouter: nethttp.NewRouter(),
		Logger:       log,
	})
	if err != nil {
		t.Fatalf("build servers: %v", err)
	}
	if servers.Public == nil {
		t.Fatalf("expected public server")
	}
	if servers.Management != nil {
		t.Fatalf("expected management server to be nil when disabled")
	}
}

func TestBuildHTTPServers_ManagementEnabledAutoCreatesRouter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Management.Enabled = true

	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	servers, err := BuildHTTPServers(&RunHTTPServersOptions{
		Config:       cfg,
		PublicRouter: nethttp.NewRouter(),
		Logger:       log,
	})
	if err != nil {
		t.Fatalf("expected no error when management router is omitted, got %v", err)
	}
	if servers == nil || servers.Management == nil {
		t.Fatalf("expected management server to be created")
	}
}

func TestInitTracerProvider_UsesVersionMetadata(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Observability.TracingEnabled = false
	cfg.Observability.TracingSampleRate = 0.25
	cfg.Observability.TracingEndpoint = "localhost:4317"
	cfg.Service.Environment = "staging"

	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	info := version.Info{
		Service: "billing",
		Version: "1.9.0",
	}

	provider, shouldShutdown, err := initTracerProvider(context.Background(), &RunHTTPServersOptions{
		Config:       cfg,
		PublicRouter: nethttp.NewRouter(),
		Logger:       log,
	}, info)
	if err != nil {
		t.Fatalf("init tracer provider: %v", err)
	}
	if provider == nil {
		t.Fatalf("expected tracer provider")
	}
	if !shouldShutdown {
		t.Fatalf("expected tracer provider shutdown ownership")
	}
}

func TestResolveEnvironment_UsesConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Service.Environment = "staging"

	fromConfig := resolveEnvironment(&RunHTTPServersOptions{Config: cfg})
	if fromConfig != "staging" {
		t.Fatalf("expected staging, got %s", fromConfig)
	}
}

func TestRunHTTPServers_ValidatesRequiredOptions(t *testing.T) {
	err := RunHTTPServers(context.Background(), nil, &RunHTTPServersOptions{})
	if err == nil || err.Error() != "servers and public server are required" {
		t.Fatalf("expected config validation error, got %v", err)
	}

	servers := &HTTPServers{Public: &PublicAPIServer{}}
	err = RunHTTPServers(context.Background(), servers, &RunHTTPServersOptions{
		Config: config.DefaultConfig(),
	})
	if err == nil || err.Error() != "logger is required" {
		t.Fatalf("expected logger validation error, got %v", err)
	}
}

func TestRunHTTPServers_StartupHookFailureStopsBoot(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Management.Enabled = false

	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	started := atomic.Bool{}
	opts := &RunHTTPServersOptions{
		Config:       cfg,
		PublicRouter: nethttp.NewRouter(),
		Logger:       log,
		StartupHooks: []LifecycleHook{
			{
				Name: "init",
				Fn: func(context.Context) error {
					started.Store(true)
					return errors.New("boom")
				},
			},
		},
	}
	servers, err := BuildHTTPServers(opts)
	if err != nil {
		t.Fatalf("build servers: %v", err)
	}
	err = RunHTTPServers(context.Background(), servers, opts)
	if err == nil {
		t.Fatal("expected startup hook error")
	}
	if !strings.Contains(err.Error(), `startup hook "init" failed`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !started.Load() {
		t.Fatal("expected startup hook to run")
	}
}

func TestRunShutdownHooks_BestEffortAndAggregatesErrors(t *testing.T) {
	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	runs := atomic.Int32{}
	joinedErr := runShutdownHooks(&RunHTTPServersOptions{
		Logger: log,
		ShutdownHooks: []LifecycleHook{
			{
				Name: "first",
				Fn: func(context.Context) error {
					runs.Add(1)
					return errors.New("first failed")
				},
			},
			{
				Name: "second",
				Fn: func(context.Context) error {
					runs.Add(1)
					return nil
				},
			},
		},
	})
	if joinedErr == nil {
		t.Fatal("expected joined shutdown hook error")
	}
	if runs.Load() != 2 {
		t.Fatalf("expected both shutdown hooks to run, got %d", runs.Load())
	}
	if !strings.Contains(joinedErr.Error(), `shutdown hook "first" failed`) {
		t.Fatalf("unexpected shutdown error: %v", joinedErr)
	}
}

func TestRunShutdownHooks_UsesDefaultTimeout(t *testing.T) {
	log, err := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	joinedErr := runShutdownHooks(&RunHTTPServersOptions{
		Logger:              log,
		ShutdownHookTimeout: 0,
		ShutdownHooks: []LifecycleHook{
			{
				Name: "timeout",
				Fn: func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				},
			},
		},
	})
	if joinedErr == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(joinedErr.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded, got %v", joinedErr)
	}
}
