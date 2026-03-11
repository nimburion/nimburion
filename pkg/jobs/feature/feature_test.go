package jobsfeature

import (
	"errors"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/spf13/cobra"
)

func TestJobsWorkerCommand_CanonicalizesBackendFactoryErrors(t *testing.T) {
	feature := NewCommandFeature(CommandFeatureOptions{
		LoadConfig: func(*cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			return config.DefaultConfig(), log, nil
		},
		BackendFactory: func(
			_ jobsconfig.Config,
			_ eventbusconfig.Config,
			_ schemavalidationconfig.KafkaValidationConfig,
			_ logger.Logger,
		) (jobs.Backend, error) {
			return nil, jobs.ErrNotInitialized
		},
		ConfigureWorker: func(*config.Config, logger.Logger, jobs.Worker) error { return nil },
	})

	command, ok := feature.Contributions().CommandRegistrations[0].Command.(*cobra.Command)
	if !ok {
		t.Fatalf("expected *cobra.Command, got %T", feature.Contributions().CommandRegistrations[0].Command)
	}
	command.SetArgs([]string{"worker"})

	err := command.Execute()
	if err == nil {
		t.Fatal("expected command error")
	}
	if !strings.Contains(err.Error(), "create jobs backend") {
		t.Fatalf("unexpected error: %v", err)
	}

	var appErr *coreerrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected canonical AppError, got %T", err)
	}
	if appErr.Code != coreerrors.CodeNotInitialized {
		t.Fatalf("AppError.Code = %q", appErr.Code)
	}
}
