package openapi

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nimburion/nimburion/pkg/config"
	appconfig "github.com/nimburion/nimburion/pkg/core/app/config"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

func TestNewCommandFeature_NilWhenRegisterRoutesMissing(t *testing.T) {
	feature := NewCommandFeature(CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			return nil, nil, nil
		},
	})
	if feature != nil {
		t.Fatal("expected nil openapi feature when register routes callback is missing")
	}
}

func TestNewCommandFeature_NilWhenLoadConfigMissing(t *testing.T) {
	feature := NewCommandFeature(CommandFeatureOptions{
		RegisterRoutes: func(router.Router, *config.Config) {},
	})
	if feature != nil {
		t.Fatal("expected nil openapi feature when load config callback is missing")
	}
}

func TestNewCommandFeature_ContributesGenerateCommand(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "generated-openapi.yaml")
	var stdout bytes.Buffer

	feature := NewCommandFeature(CommandFeatureOptions{
		ServiceName:    "svc",
		ServiceVersion: "1.2.3",
		Stdout:         &stdout,
		RegisterRoutes: func(r router.Router, _ *config.Config) {
			r.GET("/ping", func(_ router.Context) error { return nil })
		},
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			return &config.Config{
				App: appconfig.AppConfig{Name: "svc"},
			}, nil, nil
		},
	})
	if feature == nil {
		t.Fatal("expected openapi feature to be created")
	}

	contributions := feature.Contributions()
	if len(contributions.CommandRegistrations) != 1 {
		t.Fatalf("expected one command contribution, got %d", len(contributions.CommandRegistrations))
	}

	cmd, ok := contributions.CommandRegistrations[0].Command.(*cobra.Command)
	if !ok || cmd == nil {
		t.Fatal("expected contributed openapi command")
	}

	cmd.SetArgs([]string{"generate", "--output", outputPath, "--title", "Service API"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute openapi generate: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated spec: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "openapi: 3.0.3") {
		t.Fatalf("expected openapi version in generated file, got: %s", content)
	}
	if !strings.Contains(content, "/ping") {
		t.Fatalf("expected /ping route in generated file, got: %s", content)
	}
	if !strings.Contains(stdout.String(), "OpenAPI spec generated at") {
		t.Fatalf("expected command output message, got: %q", stdout.String())
	}
}
