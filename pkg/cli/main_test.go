package cli

import "testing"

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
