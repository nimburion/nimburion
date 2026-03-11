package migrate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

const (
	defaultSubcommand = "up"
	defaultSteps      = 1
)

// PendingMigration contains an unapplied migration entry for status output.
type PendingMigration struct {
	Version int64
	Name    string
}

// Status is the normalized migration status used by the CLI helper.
type Status struct {
	AppliedVersions []int64
	Pending         []PendingMigration
}

// Operations defines service-specific migration hooks.
type Operations struct {
	Up     func(ctx context.Context) (int, error)
	Down   func(ctx context.Context, steps int) (int, error)
	Status func(ctx context.Context) (*Status, error)
}

// Options configures migration command behavior.
type Options struct {
	ServiceName string
	Path        string
	Timeout     time.Duration
	Logger      logger.Logger
}

// Run executes migrate subcommands using shared parsing, timeout and logging.
func Run(args []string, opts Options, ops Operations) error {
	subcommand, steps, err := ParseArgs(args)
	if err != nil {
		return err
	}
	return RunParsed(subcommand, steps, opts, ops)
}

// RunParsed executes a parsed migration command.
func RunParsed(subcommand string, steps int, opts Options, ops Operations) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	if err := validateOperations(ops); err != nil {
		return err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch subcommand {
	case "up":
		applied, err := ops.Up(ctx)
		if err != nil {
			return err
		}
		opts.Logger.Info("migrations applied", "count", applied, "path", opts.Path)
		return nil
	case "down":
		if steps <= 0 {
			return errors.New("steps must be greater than zero")
		}
		reverted, err := ops.Down(ctx, steps)
		if err != nil {
			return err
		}
		opts.Logger.Info("migrations reverted", "count", reverted, "steps", steps, "path", opts.Path)
		return nil
	case "status":
		status, err := ops.Status(ctx)
		if err != nil {
			return err
		}
		opts.Logger.Info("migration status", "applied", len(status.AppliedVersions), "pending", len(status.Pending), "path", opts.Path)
		for _, version := range status.AppliedVersions {
			opts.Logger.Info("migration applied", "version", version)
		}
		for _, pending := range status.Pending {
			opts.Logger.Info("migration pending", "version", pending.Version, "name", pending.Name)
		}
		return nil
	default:
		return usageError(opts.ServiceName)
	}
}

// ParseArgs parses [up|down|status] [steps], defaulting to "up".
func ParseArgs(args []string) (string, int, error) {
	subcommand := defaultSubcommand
	if len(args) > 0 {
		subcommand = args[0]
	}

	steps := defaultSteps
	if len(args) > 1 {
		parsed, err := strconv.Atoi(args[1])
		if err != nil {
			return "", 0, fmt.Errorf("invalid down steps %q", args[1])
		}
		steps = parsed
	}

	return subcommand, steps, nil
}

func validateOptions(opts Options) error {
	if opts.Logger == nil {
		return errors.New("migration logger is required")
	}
	if opts.ServiceName == "" {
		return errors.New("migration service name is required")
	}
	if opts.Path == "" {
		return errors.New("migration path is required")
	}
	return nil
}

func validateOperations(ops Operations) error {
	if ops.Up == nil || ops.Down == nil || ops.Status == nil {
		return errors.New("migration operations are incomplete")
	}
	return nil
}

func usageError(serviceName string) error {
	return fmt.Errorf("usage: %s migrate [up|down|status] [steps]", serviceName)
}
