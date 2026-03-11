package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/coordination"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/spf13/cobra"
)

type testJobsRuntime struct{}

func (testJobsRuntime) Enqueue(context.Context, *jobs.Job) error              { return nil }
func (testJobsRuntime) Subscribe(context.Context, string, jobs.Handler) error { return nil }
func (testJobsRuntime) Unsubscribe(string) error                              { return nil }
func (testJobsRuntime) HealthCheck(context.Context) error                     { return nil }
func (testJobsRuntime) Close() error                                          { return nil }

func TestSchedulerRunCommand_CanonicalizesLockProviderErrors(t *testing.T) {
	feature := NewCommandFeature(CommandFeatureOptions{
		LoadConfig: func(*cobra.Command) (*config.Config, logger.Logger, error) {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			cfg := config.DefaultConfig()
			cfg.Scheduler.Enabled = true
			return cfg, log, nil
		},
		JobsRuntimeFactory: func(
			_ jobsconfig.Config,
			_ eventbusconfig.Config,
			_ schemavalidationconfig.KafkaValidationConfig,
			_ logger.Logger,
		) (jobs.Runtime, error) {
			return testJobsRuntime{}, nil
		},
		LockProviderFactory: func(*config.Config, logger.Logger) (coordination.LockProvider, error) {
			return nil, coordination.ErrNotInitialized
		},
	})

	command, ok := feature.Contributions().CommandRegistrations[0].Command.(*cobra.Command)
	if !ok {
		t.Fatalf("expected *cobra.Command, got %T", feature.Contributions().CommandRegistrations[0].Command)
	}
	command.SetArgs([]string{"run"})

	err := command.Execute()
	if err == nil {
		t.Fatal("expected command error")
	}
	if !strings.Contains(err.Error(), "create scheduler lock provider") {
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
