package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/cache"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/coordination"
	coreapp "github.com/nimburion/nimburion/pkg/core/app"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/descriptor"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	"github.com/nimburion/nimburion/pkg/health"
	httpopenapi "github.com/nimburion/nimburion/pkg/http/openapi"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsfeature "github.com/nimburion/nimburion/pkg/jobs/feature"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	relationalmigrate "github.com/nimburion/nimburion/pkg/persistence/relational/migrate"
	"github.com/nimburion/nimburion/pkg/scheduler"
	"github.com/spf13/cobra"
)

type testCLIFeature struct {
	command           *cobra.Command
	healthHook        feature.Hook
	introspectionHook feature.Hook
}

func (f testCLIFeature) Name() string { return "cli-test" }

func (f testCLIFeature) Contributions() feature.Contributions {
	contributions := feature.Contributions{}
	if f.command != nil {
		contributions.CommandRegistrations = []feature.CommandContribution{
			{Name: "inspect", Command: f.command},
		}
	}
	if f.healthHook.Name != "" || f.healthHook.Fn != nil {
		contributions.HealthContributors = []feature.Hook{f.healthHook}
	}
	if f.introspectionHook.Name != "" || f.introspectionHook.Fn != nil {
		contributions.IntrospectionContributors = []feature.Hook{f.introspectionHook}
	}
	return contributions
}

func TestResolveServiceNameValue(t *testing.T) {
	tests := []struct {
		name              string
		currentConfigName string
		defaultService    string
		override          string
		want              string
	}{
		{
			name:              "override wins",
			currentConfigName: "from-config",
			defaultService:    "from-cli",
			override:          "from-flag",
			want:              "from-flag",
		},
		{
			name:              "configured value wins over default",
			currentConfigName: "from-config",
			defaultService:    "from-cli",
			override:          "",
			want:              "from-config",
		},
		{
			name:              "default used when config missing",
			currentConfigName: "",
			defaultService:    "from-cli",
			override:          "",
			want:              "from-cli",
		},
		{
			name:              "app fallback",
			currentConfigName: "",
			defaultService:    "",
			override:          "",
			want:              "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAppNameValue(tt.currentConfigName, tt.defaultService, tt.override)
			if got != tt.want {
				t.Fatalf("resolveAppNameValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveValidationRequirements(t *testing.T) {
	tests := []struct {
		path                 []string
		wantJobsEventBusRule bool
	}{
		{path: []string{"jobs", "worker"}, wantJobsEventBusRule: true},
		{path: []string{"jobs", "enqueue"}, wantJobsEventBusRule: true},
		{path: []string{"jobs", "dlq", "list"}, wantJobsEventBusRule: true},
		{path: []string{"jobs", "dlq", "replay"}, wantJobsEventBusRule: true},
		{path: []string{"scheduler", "run"}, wantJobsEventBusRule: true},
		{path: []string{"scheduler", "trigger"}, wantJobsEventBusRule: true},
		{path: []string{"scheduler", "validate"}, wantJobsEventBusRule: false},
		{path: []string{"config", "validate"}, wantJobsEventBusRule: false},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.path, "_"), func(t *testing.T) {
			root := &cobra.Command{Use: "app"}
			current := root
			for _, segment := range tt.path {
				next := &cobra.Command{Use: segment}
				current.AddCommand(next)
				current = next
			}

			got := deriveValidationRequirements(current)
			if got.RequireJobsEventBus != tt.wantJobsEventBusRule {
				t.Fatalf("deriveValidationRequirements(%q) RequireJobsEventBus = %v, want %v", current.CommandPath(), got.RequireJobsEventBus, tt.wantJobsEventBusRule)
			}
		})
	}
}

func TestNewAppCommand_AddsCompletionByDefault(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
	})

	completionCmd, _, err := cmd.Find([]string{"completion"})
	if err != nil {
		t.Fatalf("expected completion command, got error: %v", err)
	}
	if completionCmd == nil || completionCmd.Name() != "completion" {
		t.Fatalf("expected completion command, got %#v", completionCmd)
	}

	policies := GetCommandPolicies(completionCmd)
	if got := policies[defaultPolicyContext]; got != string(PolicyAlways) {
		t.Fatalf("expected completion policy %q, got %q", PolicyAlways, got)
	}
}

func TestNewAppCommand_AddsJobsWorkerCommand(t *testing.T) {
	jobsFeature := jobsfeature.NewCommandFeature(jobsfeature.CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		ConfigureWorker: func(cfg *config.Config, log logger.Logger, worker jobs.Worker) error { return nil },
	})
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
		Features:    []feature.Feature{jobsFeature},
	})

	workerCmd, _, err := cmd.Find([]string{"jobs", "worker"})
	if err != nil {
		t.Fatalf("expected jobs worker command, got error: %v", err)
	}
	if workerCmd == nil || workerCmd.Name() != "worker" {
		t.Fatalf("expected worker command, got %#v", workerCmd)
	}
	policies := GetCommandPolicies(workerCmd)
	if got := policies[defaultPolicyContext]; got != string(PolicyScheduled) {
		t.Fatalf("expected worker policy %q, got %q", PolicyScheduled, got)
	}
}

func TestNewAppCommand_AddsJobsOpsCommands(t *testing.T) {
	jobsFeature := jobsfeature.NewCommandFeature(jobsfeature.CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
	})
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
		Features:    []feature.Feature{jobsFeature},
	})

	enqueueCmd, _, err := cmd.Find([]string{"jobs", "enqueue"})
	if err != nil {
		t.Fatalf("expected jobs enqueue command, got error: %v", err)
	}
	if enqueueCmd == nil || enqueueCmd.Name() != "enqueue" {
		t.Fatalf("expected enqueue command, got %#v", enqueueCmd)
	}

	dlqListCmd, _, err := cmd.Find([]string{"jobs", "dlq", "list"})
	if err != nil {
		t.Fatalf("expected jobs dlq list command, got error: %v", err)
	}
	if dlqListCmd == nil || dlqListCmd.Name() != "list" {
		t.Fatalf("expected dlq list command, got %#v", dlqListCmd)
	}

	dlqReplayCmd, _, err := cmd.Find([]string{"jobs", "dlq", "replay"})
	if err != nil {
		t.Fatalf("expected jobs dlq replay command, got error: %v", err)
	}
	if dlqReplayCmd == nil || dlqReplayCmd.Name() != "replay" {
		t.Fatalf("expected dlq replay command, got %#v", dlqReplayCmd)
	}
}

func TestNewAppCommand_AddsSchedulerRunCommand(t *testing.T) {
	schedulerFeature := scheduler.NewCommandFeature(scheduler.CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		ConfigureRuntime: func(cfg *config.Config, log logger.Logger, runtime *scheduler.Runtime) error { return nil },
	})
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
		Features:    []feature.Feature{schedulerFeature},
	})

	runCmd, _, err := cmd.Find([]string{"scheduler", "run"})
	if err != nil {
		t.Fatalf("expected scheduler run command, got error: %v", err)
	}
	if runCmd == nil || runCmd.Name() != "run" {
		t.Fatalf("expected run command, got %#v", runCmd)
	}
	policies := GetCommandPolicies(runCmd)
	if got := policies[defaultPolicyContext]; got != string(PolicyScheduled) {
		t.Fatalf("expected scheduler run policy %q, got %q", PolicyScheduled, got)
	}
}

func TestNewAppCommand_AddsSchedulerOpsCommands(t *testing.T) {
	schedulerFeature := scheduler.NewCommandFeature(scheduler.CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
	})
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
		Features:    []feature.Feature{schedulerFeature},
	})

	validateCmd, _, err := cmd.Find([]string{"scheduler", "validate"})
	if err != nil {
		t.Fatalf("expected scheduler validate command, got error: %v", err)
	}
	if validateCmd == nil || validateCmd.Name() != "validate" {
		t.Fatalf("expected validate command, got %#v", validateCmd)
	}

	triggerCmd, _, err := cmd.Find([]string{"scheduler", "trigger"})
	if err != nil {
		t.Fatalf("expected scheduler trigger command, got error: %v", err)
	}
	if triggerCmd == nil || triggerCmd.Name() != "trigger" {
		t.Fatalf("expected trigger command, got %#v", triggerCmd)
	}
}

func TestNewAppCommand_AddsHealthcheckByDefault(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
	})

	healthCmd, _, err := cmd.Find([]string{"healthcheck"})
	if err != nil {
		t.Fatalf("expected healthcheck command, got error: %v", err)
	}
	if healthCmd == nil || healthCmd.Name() != "healthcheck" {
		t.Fatalf("expected healthcheck command, got %#v", healthCmd)
	}
}

func TestNewAppCommand_UsesRunAsPrimaryEntrypointAndKeepsServeAlias(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
		Run: func(ctx context.Context, cfg *config.Config, log logger.Logger) error {
			return nil
		},
	})

	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("expected run command, got error: %v", err)
	}
	if runCmd == nil || runCmd.Name() != "run" {
		t.Fatalf("expected run command, got %#v", runCmd)
	}
	if len(runCmd.Aliases) != 0 {
		t.Fatalf("expected no aliases on run command, aliases = %v", runCmd.Aliases)
	}
}

func TestNewAppCommand_AddsFeatureContributedCommand(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
		Features: []feature.Feature{
			testCLIFeature{
				command: &cobra.Command{
					Use:   "inspect",
					Short: "Inspect feature state",
				},
			},
		},
	})

	inspectCmd, _, err := cmd.Find([]string{"inspect"})
	if err != nil {
		t.Fatalf("expected inspect command, got error: %v", err)
	}
	if inspectCmd == nil || inspectCmd.Name() != "inspect" {
		t.Fatalf("expected inspect command, got %#v", inspectCmd)
	}
}

func TestNewAppCommand_AddsCacheFromFeatureContribution(t *testing.T) {
	cacheFeature := cache.NewCommandFeature(cache.CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		Clean: func(ctx context.Context, cfg *config.Config, log logger.Logger, pattern string) error {
			return nil
		},
	})

	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
		Features:    []feature.Feature{cacheFeature},
	})

	cacheCmd, _, err := cmd.Find([]string{"cache", "clean"})
	if err != nil {
		t.Fatalf("expected cache clean command, got error: %v", err)
	}
	if cacheCmd == nil || cacheCmd.Name() != "clean" {
		t.Fatalf("expected clean command, got %#v", cacheCmd)
	}
}

func TestNewAppCommand_DoesNotAddMigrateWithoutFeatureContribution(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
	})

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	if err == nil && migrateCmd != nil && migrateCmd.Name() == "migrate" {
		t.Fatalf("expected migrate command to be absent without feature contribution")
	}
}

func TestNewAppCommand_AddsMigrateFromFeatureContribution(t *testing.T) {
	migrateFeature := relationalmigrate.NewCommandFeature(relationalmigrate.CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		Run: func(ctx context.Context, cfg *config.Config, log logger.Logger, direction string, args []string) error {
			return nil
		},
	})

	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
		Features:    []feature.Feature{migrateFeature},
	})

	migrateCmd, _, err := cmd.Find([]string{"migrate"})
	if err != nil {
		t.Fatalf("expected migrate command, got error: %v", err)
	}
	if migrateCmd == nil || migrateCmd.Name() != "migrate" {
		t.Fatalf("expected migrate command, got %#v", migrateCmd)
	}
}

func TestNewAppCommand_AddsIntrospectCommand(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
	})

	introspectCmd, _, err := cmd.Find([]string{"introspect"})
	if err != nil {
		t.Fatalf("expected introspect command, got error: %v", err)
	}
	if introspectCmd == nil || introspectCmd.Name() != "introspect" {
		t.Fatalf("expected introspect command, got %#v", introspectCmd)
	}
}

func TestNewAppCommand_AddsDescribeCommand(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
	})

	describeCmd, _, err := cmd.Find([]string{"describe"})
	if err != nil {
		t.Fatalf("expected describe command, got error: %v", err)
	}
	if describeCmd == nil || describeCmd.Name() != "describe" {
		t.Fatalf("expected describe command, got %#v", describeCmd)
	}
}

func TestDescribeCommand_EmitsDescriptorJSON(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
		Run:         func(ctx context.Context, cfg *config.Config, log logger.Logger) error { return nil },
		Descriptor: descriptor.Options{
			Application: descriptor.Application{
				Kind:        descriptor.ApplicationKindService,
				TenancyMode: descriptor.TenancyModeSingleTenant,
			},
			Transports: []descriptor.Transport{
				{Family: descriptor.TransportFamilyHTTP},
			},
		},
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"describe", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute describe: %v", err)
	}

	var payload descriptor.Descriptor
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal descriptor output: %v\noutput=%s", err, out.String())
	}
	if payload.DescriptorVersion != descriptor.VersionV1 {
		t.Fatalf("expected descriptor version %q, got %q", descriptor.VersionV1, payload.DescriptorVersion)
	}
	if payload.Application.Name != "testapp" {
		t.Fatalf("expected app name testapp, got %q", payload.Application.Name)
	}
	if payload.Runtime.DefaultCommand != "run" {
		t.Fatalf("expected run as default command, got %q", payload.Runtime.DefaultCommand)
	}
}

func TestNewAppCommand_OmitsFrameworkOptionalCommandsByDefault(t *testing.T) {
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
	})

	if jobsCmd, _, err := cmd.Find([]string{"jobs"}); err == nil && jobsCmd != nil && jobsCmd.Name() == "jobs" {
		t.Fatalf("expected jobs command to be omitted by default for app command")
	}
	if schedulerCmd, _, err := cmd.Find([]string{"scheduler"}); err == nil && schedulerCmd != nil && schedulerCmd.Name() == "scheduler" {
		t.Fatalf("expected scheduler command to be omitted by default for app command")
	}
	if cacheCmd, _, err := cmd.Find([]string{"cache"}); err == nil && cacheCmd != nil && cacheCmd.Name() == "cache" {
		t.Fatalf("expected cache command to be omitted by default for app command")
	}
	if openAPICmd, _, err := cmd.Find([]string{"openapi"}); err == nil && openAPICmd != nil && openAPICmd.Name() == "openapi" {
		t.Fatalf("expected openapi command to be omitted by default for app command")
	}
}

func TestNewAppCommand_AddsOpenAPIFromFeatureContribution(t *testing.T) {
	openAPIFeature := httpopenapi.NewCommandFeature(httpopenapi.CommandFeatureOptions{
		RegisterRoutes: func(r router.Router, _ *config.Config) {
			r.GET("/ping", func(c router.Context) error { return nil })
		},
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		ServiceName:    "testapp",
		ServiceVersion: "dev",
	})

	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testapp",
		Description: "test app",
		Features:    []feature.Feature{openAPIFeature},
	})

	openAPICmd, _, err := cmd.Find([]string{"openapi"})
	if err != nil {
		t.Fatalf("expected openapi command, got error: %v", err)
	}
	if openAPICmd == nil || openAPICmd.Name() != "openapi" {
		t.Fatalf("expected openapi command, got %#v", openAPICmd)
	}
}

func TestShouldCheckJobsRuntimeHealth(t *testing.T) {
	cfg := config.DefaultConfig()
	if shouldCheckJobsRuntimeHealth(cfg) {
		t.Fatal("expected default config to skip jobs runtime health checks")
	}

	cfg.EventBus.Type = eventbusconfig.EventBusTypeKafka
	if !shouldCheckJobsRuntimeHealth(cfg) {
		t.Fatal("expected jobs runtime health checks when eventbus is configured")
	}
}

func TestHealthCheckResultError(t *testing.T) {
	if err := healthCheckResultError(health.CheckResult{
		Name:   "ok",
		Status: health.StatusHealthy,
	}); err != nil {
		t.Fatalf("expected nil error for healthy status, got %v", err)
	}

	if err := healthCheckResultError(health.CheckResult{
		Name:   "degraded",
		Status: health.StatusDegraded,
	}); err != nil {
		t.Fatalf("expected nil error for degraded status, got %v", err)
	}

	err := healthCheckResultError(health.CheckResult{
		Name:    "jobs-runtime",
		Status:  health.StatusUnhealthy,
		Message: "backend unavailable",
	})
	if err == nil {
		t.Fatal("expected unhealthy check error")
	}
	if !strings.Contains(err.Error(), "jobs-runtime") {
		t.Fatalf("expected check name in error, got %v", err)
	}
	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != coreerrors.CodeUnavailable {
		t.Fatalf("Code = %q", appErr.Code)
	}
}

func TestRunHealthcheckWithRegistry_UsesFeatureHealthContributions(t *testing.T) {
	registry := health.NewRegistry()
	err := runHealthcheckWithRegistry(context.Background(), healthcheckRuntimeOptions{
		name: "testapp",
		cfg:  config.DefaultConfig(),
		features: []feature.Feature{
			testCLIFeature{
				healthHook: feature.Hook{
					Name: "feature-health",
					Fn: func(_ context.Context, runtime feature.Runtime) error {
						runtime.HealthRegistry().RegisterFunc("feature-health", func(_ context.Context) health.CheckResult {
							return health.CheckResult{Name: "feature-health", Status: health.StatusHealthy}
						})
						return nil
					},
				},
			},
		},
		registry: registry,
	})
	if err != nil {
		t.Fatalf("runHealthcheckWithRegistry() error = %v", err)
	}

	if _, err := registry.CheckOne(context.Background(), "feature-health"); err != nil {
		t.Fatalf("expected feature health check in shared registry, got %v", err)
	}
}

func TestRunHealthcheckWithRegistry_ReturnsApplicationDependencyFailures(t *testing.T) {
	err := runHealthcheckWithRegistry(context.Background(), healthcheckRuntimeOptions{
		name:              "testapp",
		cfg:               config.DefaultConfig(),
		checkDependencies: func(context.Context, *config.Config, logger.Logger) error { return errors.New("backend unavailable") },
	})
	if err == nil || !strings.Contains(err.Error(), "application-dependencies") {
		t.Fatalf("expected application dependency failure, got %v", err)
	}
}

func TestRunHealthcheckWithRegistry_AllowsDegradedState(t *testing.T) {
	registry := health.NewRegistry()
	registry.Register(health.NewCustomChecker("degraded-check", func(ctx context.Context) (health.Status, string, error) {
		return health.StatusDegraded, "degraded but serving", nil
	}))

	err := runHealthcheckWithRegistry(context.Background(), healthcheckRuntimeOptions{
		name:     "testapp",
		cfg:      config.DefaultConfig(),
		registry: registry,
	})
	if err != nil {
		t.Fatalf("expected degraded healthcheck to stay ready, got %v", err)
	}
}

func TestRegisterFrameworkDependencyHealthChecks_CanonicalizesJobsRuntimeCreationError(t *testing.T) {
	registry := health.NewRegistry()
	cfg := config.DefaultConfig()
	cfg.EventBus.Type = eventbusconfig.EventBusTypeKafka

	log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	registerFrameworkDependencyHealthChecks(
		registry,
		cfg,
		log,
		func(*config.Config, logger.Logger) (jobs.Runtime, error) {
			return nil, jobs.ErrNotInitialized
		},
		func(*config.Config, logger.Logger) (coordination.LockProvider, error) {
			return nil, nil
		},
	)

	result, err := registry.CheckOne(context.Background(), "jobs-runtime")
	if err != nil {
		t.Fatalf("CheckOne() error = %v", err)
	}
	if result.Status != health.StatusUnhealthy {
		t.Fatalf("Status = %s", result.Status)
	}
	if !strings.Contains(result.Error, "jobs not initialized") {
		t.Fatalf("expected canonicalized jobs error, got %q", result.Error)
	}
}

func TestRegisterFrameworkDependencyHealthChecks_CanonicalizesSchedulerLockProviderCreationError(t *testing.T) {
	registry := health.NewRegistry()
	cfg := config.DefaultConfig()
	cfg.EventBus.Type = eventbusconfig.EventBusTypeKafka
	cfg.Scheduler.Enabled = true
	cfg.Scheduler.LockProvider = "redis"
	cfg.Scheduler.LockTTL = time.Second

	log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
	registerFrameworkDependencyHealthChecks(
		registry,
		cfg,
		log,
		func(*config.Config, logger.Logger) (jobs.Runtime, error) {
			return nil, nil
		},
		func(*config.Config, logger.Logger) (coordination.LockProvider, error) {
			return nil, coordination.ErrNotInitialized
		},
	)

	result, err := registry.CheckOne(context.Background(), "scheduler-lock-provider")
	if err != nil {
		t.Fatalf("CheckOne() error = %v", err)
	}
	if result.Status != health.StatusUnhealthy {
		t.Fatalf("Status = %s", result.Status)
	}
	if !strings.Contains(result.Error, "coordination not initialized") {
		t.Fatalf("expected canonicalized coordination error, got %q", result.Error)
	}
}

func TestCoreAppPrepareGatesIntrospectionOnDebug(t *testing.T) {
	app, err := coreapp.New(coreapp.Options{
		Name: "testapp",
		Features: []feature.Feature{
			testCLIFeature{
				introspectionHook: feature.Hook{
					Name: "feature-introspection",
					Fn: func(_ context.Context, runtime feature.Runtime) error {
						runtime.IntrospectionRegistry().Set("feature", "enabled")
						return nil
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if _, ok := app.Runtime().Introspection.Get("feature"); ok {
		t.Fatal("expected introspection to stay disabled without debug")
	}
}

func TestResolveEnvPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "APP"},
		{"  ", "APP"},
		{"custom", "CUSTOM"},
		{"MyApp", "MYAPP"},
	}

	for _, tt := range tests {
		result := resolveEnvPrefix(tt.input)
		if result != tt.expected {
			t.Errorf("resolveEnvPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatSettings(t *testing.T) {
	result, err := formatSettings(nil)
	if err != nil || result != "{}\n" {
		t.Errorf("formatSettings(nil) = %q, %v", result, err)
	}

	result, err = formatSettings(map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "key") {
		t.Errorf("expected key in output: %s", result)
	}
}

func TestRedactSettingsMap(t *testing.T) {
	settings := map[string]interface{}{
		"public": "visible",
		"secret": "hidden",
		"nested": map[string]interface{}{"password": "secret123"},
	}
	secrets := map[string]interface{}{
		"secret": true,
		"nested": map[string]interface{}{"password": true},
	}

	result := redactSettingsMap(settings, secrets)
	if result["public"] != "visible" {
		t.Errorf("public value should not be redacted")
	}
	if result["secret"] != "***" {
		t.Errorf("secret should be redacted, got %v", result["secret"])
	}

	nested := result["nested"].(map[string]interface{})
	if nested["password"] != "***" {
		t.Errorf("nested password should be redacted")
	}
}

func TestShouldRedactSetting(t *testing.T) {
	tests := []struct {
		mask     interface{}
		expected bool
	}{
		{nil, false},
		{"", false},
		{"  ", false},
		{"secret", true},
		{true, true},
		{false, false},
		{0, false},
		{1, true},
		{int64(0), false},
		{int64(1), true},
		{0.0, false},
		{1.5, true},
		{[]interface{}{}, false},
		{[]interface{}{"x"}, true},
		{map[string]interface{}{}, false},
		{map[string]interface{}{"k": "v"}, true},
	}

	for _, tt := range tests {
		result := shouldRedactSetting(tt.mask)
		if result != tt.expected {
			t.Errorf("shouldRedactSetting(%v) = %v, want %v", tt.mask, result, tt.expected)
		}
	}
}
