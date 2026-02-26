package cli

import (
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/scheduler"
)

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

func TestNewServiceCommand_AddsCompletionByDefault(t *testing.T) {
	cmd := NewServiceCommand(ServiceCommandOptions{
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

func TestNewServiceCommand_AddsJobsWorkerCommand(t *testing.T) {
	cmd := NewServiceCommand(ServiceCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
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

func TestNewServiceCommand_AddsJobsOpsCommands(t *testing.T) {
	cmd := NewServiceCommand(ServiceCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
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

func TestNewServiceCommand_AddsSchedulerRunCommand(t *testing.T) {
	cmd := NewServiceCommand(ServiceCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
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

func TestNewServiceCommand_AddsSchedulerOpsCommands(t *testing.T) {
	cmd := NewServiceCommand(ServiceCommandOptions{
		Name:        "testsvc",
		Description: "test service",
		ConfigPath:  "",
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

func TestNewServiceCommand_AddsHealthcheckByDefault(t *testing.T) {
	cmd := NewServiceCommand(ServiceCommandOptions{
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
		"public":  "visible",
		"secret":  "hidden",
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
