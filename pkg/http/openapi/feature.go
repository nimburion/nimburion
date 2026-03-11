package openapi

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nimburion/nimburion/pkg/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	httprouter "github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// ConfigLoader loads the service config using command flags.
type ConfigLoader func(cmd *cobra.Command) (*config.Config, logger.Logger, error)

// CommandFeatureOptions defines the inputs required to contribute the OpenAPI command.
type CommandFeatureOptions struct {
	RegisterRoutes func(r httprouter.Router, cfg *config.Config)
	LoadConfig     ConfigLoader
	ServiceName    string
	ServiceVersion string
	Stdout         io.Writer
}

type commandFeature struct {
	command *cobra.Command
}

func (f commandFeature) Name() string { return "http-openapi" }

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

// NewCommandFeature contributes the OpenAPI CLI command owned by the HTTP family.
func NewCommandFeature(opts CommandFeatureOptions) corefeature.Feature {
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

	return commandFeature{command: cmd}
}

func runGenerate(cmd *cobra.Command, opts CommandFeatureOptions, outputPath, titleOverride string) error {
	cfg, _, err := opts.LoadConfig(cmd)
	if err != nil {
		return err
	}

	routes := CollectRoutes(func(r httprouter.Router) {
		opts.RegisterRoutes(r, cfg)
	})
	if len(routes) == 0 {
		return fmt.Errorf("no routes were registered, cannot generate OpenAPI specification")
	}

	title := strings.TrimSpace(titleOverride)
	if title == "" {
		title = strings.TrimSpace(cfg.App.Name)
	}
	if title == "" {
		title = strings.TrimSpace(opts.ServiceName)
	}

	spec := BuildSpec(title, strings.TrimSpace(opts.ServiceVersion), routes)
	if err := WriteSpec(outputPath, spec); err != nil {
		return err
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if _, err := fmt.Fprintf(stdout, "✓ OpenAPI spec generated at %s (%d routes)\n", outputPath, len(routes)); err != nil {
		return fmt.Errorf("write openapi generation output: %w", err)
	}
	return nil
}
