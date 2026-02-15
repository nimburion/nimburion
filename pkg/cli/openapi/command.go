package openapi

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	serveropenapi "github.com/nimburion/nimburion/pkg/server/openapi"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ConfigLoader loads the service config using command flags.
type ConfigLoader func(flags *pflag.FlagSet) (*config.Config, logger.Logger, error)

// CommandOptions configures the OpenAPI command tree.
type CommandOptions struct {
	RegisterRoutes func(r router.Router, cfg *config.Config)
	LoadConfig     ConfigLoader
	ServiceName    string
	ServiceVersion string
	Stdout         io.Writer
}

// NewCommand creates the "openapi" command and its subcommands.
func NewCommand(opts CommandOptions) *cobra.Command {
	if opts.RegisterRoutes == nil || opts.LoadConfig == nil {
		return nil
	}

	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "OpenAPI specification commands",
	}

	var outputPath string
	var titleOverride string
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate OpenAPI specification from registered routes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGenerate(cmd, opts, outputPath, titleOverride)
		},
	}
	generateCmd.Flags().StringVarP(&outputPath, "output", "o", "openapi.generated.yaml", "output file path (.yaml or .json)")
	generateCmd.Flags().StringVar(&titleOverride, "title", "", "OpenAPI title override")
	cmd.AddCommand(generateCmd)

	return cmd
}

func runGenerate(
	cmd *cobra.Command,
	opts CommandOptions,
	outputPath string,
	titleOverride string,
) error {
	cfg, _, err := opts.LoadConfig(cmd.Flags())
	if err != nil {
		return err
	}

	routes := serveropenapi.CollectRoutes(func(r router.Router) {
		opts.RegisterRoutes(r, cfg)
	})
	if len(routes) == 0 {
		return fmt.Errorf("no routes were registered, cannot generate OpenAPI specification")
	}

	title := strings.TrimSpace(titleOverride)
	if title == "" {
		title = strings.TrimSpace(cfg.Service.Name)
	}
	if title == "" {
		title = strings.TrimSpace(opts.ServiceName)
	}

	spec := serveropenapi.BuildSpec(title, strings.TrimSpace(opts.ServiceVersion), routes)
	if err := serveropenapi.WriteSpec(outputPath, spec); err != nil {
		return err
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	_, _ = fmt.Fprintf(stdout, "✓ OpenAPI spec generated at %s (%d routes)\n", outputPath, len(routes))
	return nil
}
