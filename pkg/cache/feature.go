package cache

import (
	"context"

	cachefamilyconfig "github.com/nimburion/nimburion/pkg/cache/config"
	"github.com/nimburion/nimburion/pkg/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/spf13/cobra"
)

const (
	commandPolicyAnnotationPrefix = "policies."
	defaultCommandPolicyContext   = "run"
	policyRun                     = "run"
)

// ConfigLoader loads config and logger for contributed cache commands.
type ConfigLoader func(cmd *cobra.Command) (*config.Config, logger.Logger, error)

// Cleaner executes the cache clean operation owned by the cache family.
type Cleaner func(ctx context.Context, cfg *config.Config, log logger.Logger, pattern string) error

// CommandFeatureOptions defines the inputs required to contribute cache commands.
type CommandFeatureOptions struct {
	LoadConfig ConfigLoader
	Clean      Cleaner
}

type commandFeature struct {
	command *cobra.Command
}

func (f commandFeature) Name() string { return "cache" }

func (f commandFeature) Contributions() corefeature.Contributions {
	if f.command == nil {
		return corefeature.Contributions{}
	}
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "cache", Extension: &cachefamilyconfig.Extension{}},
		},
		CommandRegistrations: []corefeature.CommandContribution{
			{Name: f.command.Name(), Command: f.command},
		},
	}
}

// NewCommandFeature contributes the cache command tree owned by the cache family.
func NewCommandFeature(opts CommandFeatureOptions) corefeature.Feature {
	if opts.LoadConfig == nil || opts.Clean == nil {
		return nil
	}

	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache management commands",
	}
	setCommandPolicy(cacheCmd, policyRun)

	var pattern string
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean cache entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			return opts.Clean(cmd.Context(), cfg, log, pattern)
		},
	}
	setCommandPolicy(cleanCmd, policyRun)
	cleanCmd.Flags().StringVarP(&pattern, "pattern", "p", "*", "cache key pattern to clean")
	cacheCmd.AddCommand(cleanCmd)

	return commandFeature{command: cacheCmd}
}

func setCommandPolicy(cmd *cobra.Command, policy string) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[commandPolicyAnnotationPrefix+defaultCommandPolicyContext] = policy
}
