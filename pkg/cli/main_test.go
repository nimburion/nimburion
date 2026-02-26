package cli

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
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
