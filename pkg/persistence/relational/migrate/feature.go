package migrate

import (
	"context"
	"fmt"
	"os"

	"github.com/nimburion/nimburion/pkg/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ConfigLoader loads config and logger for contributed migration commands.
type ConfigLoader func(flags *pflag.FlagSet) (*config.Config, logger.Logger, error)

// CommandRunner executes a migration subcommand owned by a feature family.
type CommandRunner func(ctx context.Context, cfg *config.Config, log logger.Logger, direction string, args []string) error

// CommandFeatureOptions defines the inputs required to contribute migrate commands.
type CommandFeatureOptions struct {
	LoadConfig ConfigLoader
	Run        CommandRunner
}

type commandFeature struct {
	command *cobra.Command
}

func (f commandFeature) Name() string { return "relational-migrate" }

func (f commandFeature) Contributions() corefeature.Contributions {
	if f.command == nil {
		return corefeature.Contributions{}
	}
	return corefeature.Contributions{
		CommandRegistrations: []corefeature.CommandContribution{
			{Name: f.command.Name(), Command: f.command},
		},
	}
}

// NewCommandFeature contributes a migrate command tree owned by the relational family.
func NewCommandFeature(opts CommandFeatureOptions) corefeature.Feature {
	if opts.LoadConfig == nil || opts.Run == nil {
		return nil
	}

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	var migrationsPath string
	migrateCmd.PersistentFlags().StringVar(&migrationsPath, "migrations-path", "", "migrations path override")

	addMigrationSubcommand := func(use, short, direction string) {
		migrateCmd.AddCommand(&cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := setMigrationPathEnv(migrationsPath); err != nil {
					return err
				}
				cfg, log, err := opts.LoadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.Run(cmd.Context(), cfg, log, direction, args)
			},
		})
	}

	addMigrationSubcommand("up", "Run pending migrations", "up")
	addMigrationSubcommand("down", "Rollback last migration", "down")
	addMigrationSubcommand("status", "Show migration status", "status")

	return commandFeature{command: migrateCmd}
}

func setMigrationPathEnv(migrationsPath string) error {
	if migrationsPath == "" {
		return nil
	}
	if err := os.Setenv("APP_MIGRATIONS_PATH", migrationsPath); err != nil {
		return fmt.Errorf("set APP_MIGRATIONS_PATH: %w", err)
	}
	if err := os.Setenv("APP_PLATFORM_MIGRATIONS_PATH", migrationsPath); err != nil {
		return fmt.Errorf("set APP_PLATFORM_MIGRATIONS_PATH: %w", err)
	}
	return nil
}
