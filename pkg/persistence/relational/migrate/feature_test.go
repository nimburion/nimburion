package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

func TestNewCommandFeature(t *testing.T) {
	var called bool

	feature := NewCommandFeature(CommandFeatureOptions{
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		Run: func(_ context.Context, _ *config.Config, _ logger.Logger, direction string, _ []string) error {
			called = true
			if direction != "status" {
				t.Fatalf("expected status direction, got %s", direction)
			}
			return nil
		},
	})
	if feature == nil {
		t.Fatal("expected feature")
	}

	contributions := feature.Contributions()
	if len(contributions.CommandRegistrations) != 1 {
		t.Fatalf("expected one command contribution, got %d", len(contributions.CommandRegistrations))
	}

	cmd, ok := contributions.CommandRegistrations[0].Command.(*cobra.Command)
	if !ok {
		t.Fatal("expected cobra command contribution")
	}
	cmd.SetArgs([]string{"status"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute migrate status: %v", err)
	}
	if !called {
		t.Fatal("expected run callback to be called")
	}
}

func TestNewCommandFeature_MigrationsPathUsesConfiguredEnvPrefix(t *testing.T) {
	const (
		envPrefix      = "service"
		migrationsPath = "/tmp/migrations"
	)

	t.Setenv("SERVICE_MIGRATIONS_PATH", "")
	t.Setenv("SERVICE_PLATFORM_MIGRATIONS_PATH", "")

	feature := NewCommandFeature(CommandFeatureOptions{
		EnvPrefix: envPrefix,
		LoadConfig: func(_ *cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		Run: func(_ context.Context, _ *config.Config, _ logger.Logger, _ string, _ []string) error {
			got, ok := os.LookupEnv("SERVICE_MIGRATIONS_PATH")
			if !ok || got != migrationsPath {
				t.Fatalf("expected SERVICE_MIGRATIONS_PATH=%q, got %q", migrationsPath, got)
			}
			got, ok = os.LookupEnv("SERVICE_PLATFORM_MIGRATIONS_PATH")
			if !ok || got != migrationsPath {
				t.Fatalf("expected SERVICE_PLATFORM_MIGRATIONS_PATH=%q, got %q", migrationsPath, got)
			}
			return nil
		},
	})
	if feature == nil {
		t.Fatal("expected feature")
	}

	contributions := feature.Contributions()
	cmd, ok := contributions.CommandRegistrations[0].Command.(*cobra.Command)
	if !ok {
		t.Fatal("expected cobra command contribution")
	}
	cmd.SetArgs([]string{"status", "--migrations-path", migrationsPath})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute migrate status: %v", err)
	}
}
