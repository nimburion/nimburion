package openapi

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/spf13/pflag"
)

func TestNewCommand_NilWhenRegisterRoutesMissing(t *testing.T) {
	cmd := NewCommand(CommandOptions{
		LoadConfig: func(_ *pflag.FlagSet) (*config.Config, logger.Logger, error) {
			return nil, nil, nil
		},
	})
	if cmd != nil {
		t.Fatal("expected nil openapi command when register routes callback is missing")
	}
}

func TestNewCommand_NilWhenLoadConfigMissing(t *testing.T) {
	cmd := NewCommand(CommandOptions{
		RegisterRoutes: func(router.Router, *config.Config) {},
	})
	if cmd != nil {
		t.Fatal("expected nil openapi command when load config callback is missing")
	}
}

func TestGenerateCommand_WritesSpecFromRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "generated-openapi.yaml")
	var stdout bytes.Buffer

	opts := CommandOptions{
		ServiceName:    "svc",
		ServiceVersion: "1.2.3",
		Stdout:         &stdout,
		RegisterRoutes: func(r router.Router, _ *config.Config) {
			r.GET("/ping", func(c router.Context) error { return nil })
		},
		LoadConfig: func(_ *pflag.FlagSet) (*config.Config, logger.Logger, error) {
			return &config.Config{
				Service: config.ServiceConfig{Name: "svc"},
			}, nil, nil
		},
	}

	cmd := NewCommand(opts)
	if cmd == nil {
		t.Fatal("expected openapi command to be created")
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
