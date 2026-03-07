package migrate

import (
	"context"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestNewCommandFeature(t *testing.T) {
	var called bool

	feature := NewCommandFeature(CommandFeatureOptions{
		LoadConfig: func(flags *pflag.FlagSet) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		Run: func(ctx context.Context, cfg *config.Config, log logger.Logger, direction string, args []string) error {
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
