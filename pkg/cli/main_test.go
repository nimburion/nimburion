package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	coreapp "github.com/nimburion/nimburion/pkg/core/app"
	"github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
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
			got := resolveServiceNameValue(tt.currentConfigName, tt.defaultService, tt.override)
			if got != tt.want {
				t.Fatalf("resolveServiceNameValue() = %q, want %q", got, tt.want)
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
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
		IncludeJobs: true,
		ConfigureJobsWorker: func(cfg *config.Config, log logger.Logger, worker jobs.Worker) error {
			return nil
		},
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
	cmd := NewAppCommand(AppCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
		IncludeJobs: true,
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
	cmd := NewAppCommand(AppCommandOptions{
		Name:             "testsvc",
		Description:      "test service",
		ConfigPath:       "",
		IncludeScheduler: true,
		ConfigureScheduler: func(cfg *config.Config, log logger.Logger, runtime *scheduler.Runtime) error {
			return nil
		},
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
	cmd := NewAppCommand(AppCommandOptions{
		Name:             "testsvc",
		Description:      "test service",
		ConfigPath:       "",
		IncludeScheduler: true,
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

func TestResolveWorkerQueues(t *testing.T) {
	tests := []struct {
		name          string
		flagQueues    []string
		defaultQueue  string
		expectedQueue []string
	}{
		{
			name:          "uses flags",
			flagQueues:    []string{"payments", "emails"},
			defaultQueue:  "default",
			expectedQueue: []string{"payments", "emails"},
		},
		{
			name:          "falls back to config default",
			flagQueues:    []string{"", " "},
			defaultQueue:  "jobs-default",
			expectedQueue: []string{"jobs-default"},
		},
		{
			name:          "falls back to framework default",
			flagQueues:    nil,
			defaultQueue:  "",
			expectedQueue: []string{"default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWorkerQueues(tt.flagQueues, tt.defaultQueue)
			if len(got) != len(tt.expectedQueue) {
				t.Fatalf("expected %d queues, got %d", len(tt.expectedQueue), len(got))
			}
			for idx := range got {
				if got[idx] != tt.expectedQueue[idx] {
					t.Fatalf("queue[%d] = %q, want %q", idx, got[idx], tt.expectedQueue[idx])
				}
			}
		})
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
}

func TestShouldCheckJobsRuntimeHealth(t *testing.T) {
	cfg := config.DefaultConfig()
	if shouldCheckJobsRuntimeHealth(cfg) {
		t.Fatal("expected default config to skip jobs runtime health checks")
	}

	cfg.EventBus.Type = config.EventBusTypeKafka
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
