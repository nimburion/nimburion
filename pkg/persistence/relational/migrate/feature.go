package migrate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nimburion/nimburion/pkg/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// ConfigLoader loads config and logger for contributed migration commands.
type ConfigLoader func(cmd *cobra.Command) (*config.Config, logger.Logger, error)

// CommandRunner executes a migration subcommand owned by a feature family.
type CommandRunner func(ctx context.Context, cfg *config.Config, log logger.Logger, direction string, args []string) error

// CommandFeatureOptions defines the inputs required to contribute migrate commands.
type CommandFeatureOptions struct {
	LoadConfig ConfigLoader
	Run        CommandRunner
	EnvPrefix  string
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
				if err := setMigrationPathEnv(opts.EnvPrefix, migrationsPath); err != nil {
					return err
				}
				cfg, log, err := opts.LoadConfig(cmd)
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

func setMigrationPathEnv(envPrefix, migrationsPath string) error {
	if migrationsPath == "" {
		return nil
	}
	prefix := resolveEnvPrefix(envPrefix)
	migrationsEnv := prefix + "_MIGRATIONS_PATH"
	platformMigrationsEnv := prefix + "_PLATFORM_MIGRATIONS_PATH"

	if err := os.Setenv(migrationsEnv, migrationsPath); err != nil {
		return fmt.Errorf("set %s: %w", migrationsEnv, err)
	}
	if err := os.Setenv(platformMigrationsEnv, migrationsPath); err != nil {
		return fmt.Errorf("set %s: %w", platformMigrationsEnv, err)
	}
	return nil
}

func resolveEnvPrefix(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "APP"
	}
	return strings.ToUpper(trimmed)
}
