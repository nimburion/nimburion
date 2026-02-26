package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	cliopenapi "github.com/nimburion/nimburion/pkg/cli/openapi"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsfactory "github.com/nimburion/nimburion/pkg/jobs/factory"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/scheduler"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	policiesAnnotationPrefix = "policies."
	defaultPolicyContext     = "run"
)

// CommandPolicy defines the supported command policy values.
type CommandPolicy string

const (
	PolicyAlways      CommandPolicy = "always"
	PolicyNever       CommandPolicy = "never"
	PolicyOnce        CommandPolicy = "once"
	PolicyMigration   CommandPolicy = "migration"
	PolicyRun         CommandPolicy = "run"
	PolicyManual      CommandPolicy = "manual"
	PolicyOnDemand    CommandPolicy = "on_demand"
	PolicyScheduled   CommandPolicy = "scheduled"
	PolicyConditional CommandPolicy = "conditional"
)

// JobsRuntimeFactory creates a jobs runtime from service configuration.
type JobsRuntimeFactory func(
	cfg config.JobsConfig,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Runtime, error)

// JobsBackendFactory creates a lease-aware jobs backend from service configuration.
type JobsBackendFactory func(
	cfg config.JobsConfig,
	eventBusCfg config.EventBusConfig,
	validationCfg config.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Backend, error)

// SchedulerLockProviderFactory creates the scheduler lock provider.
type SchedulerLockProviderFactory func(cfg *config.Config, log logger.Logger) (scheduler.LockProvider, error)

// ServiceCommandOptions defines callbacks for service-specific logic.
type ServiceCommandOptions struct {
	Name        string
	Description string
	ConfigPath  string
	// Optional: called with the resolved path to the configuration file after flags are parsed.
	ConfigPathResolved func(string)
	EnvPrefix          string

	// Optional: config extensions to load alongside core config.
	ConfigExtensions []any

	// Required: server startup logic
	RunServer func(ctx context.Context, cfg *config.Config, log logger.Logger) error

	// Optional: migration logic
	RunMigrations func(ctx context.Context, cfg *config.Config, log logger.Logger, direction string, args []string) error

	// Optional: cache clean logic
	RunCacheClean func(ctx context.Context, cfg *config.Config, log logger.Logger, pattern string) error

	// Optional: dependency health checks
	CheckDependencies func(ctx context.Context, cfg *config.Config, log logger.Logger) error

	// Optional: custom config validation (runs after Nimburion's built-in validation)
	ValidateConfig func(cfg *config.Config) error

	// Optional: route registration callback used by the openapi generate command.
	RegisterRoutes func(r router.Router, cfg *config.Config)

	// Optional: additional custom commands
	CustomCommands []*cobra.Command

	// Optional: registers worker handlers for "jobs worker" command.
	ConfigureJobsWorker func(cfg *config.Config, log logger.Logger, worker jobs.Worker) error
	// Optional: registers tasks for "scheduler run" command.
	ConfigureScheduler func(cfg *config.Config, log logger.Logger, runtime *scheduler.Runtime) error
	// Optional: override jobs runtime factory (useful for tests/custom adapters).
	JobsRuntimeFactory JobsRuntimeFactory
	// Optional: override jobs backend factory (useful for tests/custom adapters).
	JobsBackendFactory JobsBackendFactory
	// Optional: override scheduler lock provider factory.
	SchedulerLockProviderFactory SchedulerLockProviderFactory
}

// NewServiceCommand creates a standardized CLI with serve, migrate, cache, version, healthcheck, and config subcommands.
func NewServiceCommand(opts ServiceCommandOptions) *cobra.Command {
	if opts.EnvPrefix == "" {
		opts.EnvPrefix = "APP"
	}

	rootCmd := &cobra.Command{
		Use:   opts.Name,
		Short: opts.Description,
	}
	SetCommandPolicies(rootCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	var cfgPath string
	var secretFilePath string
	var serviceNameOverride string
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config-file", "c", opts.ConfigPath, "config file path")
	rootCmd.PersistentFlags().StringVar(&secretFilePath, "secret-file", "", "path to secrets file (sets APP_SECRETS_FILE)")
	rootCmd.PersistentFlags().StringVar(&serviceNameOverride, "service-name", "", "service name override")

	loadConfig := func(flags *pflag.FlagSet) (*config.Config, logger.Logger, error) {
		if opts.ConfigPathResolved != nil {
			opts.ConfigPathResolved(cfgPath)
		}
		return LoadConfigAndLogger(
			cfgPath,
			opts.EnvPrefix,
			secretFilePath,
			opts.ValidateConfig,
			flags,
			opts.ConfigExtensions,
			opts.Name,
			serviceNameOverride,
		)
	}

	buildJobsRuntime := func(cfg *config.Config, log logger.Logger) (jobs.Runtime, error) {
		runtimeFactory := opts.JobsRuntimeFactory
		if runtimeFactory == nil {
			runtimeFactory = jobsfactory.NewRuntimeWithValidation
		}
		return runtimeFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
	}

	buildJobsBackend := func(cfg *config.Config, log logger.Logger) (jobs.Backend, error) {
		backendFactory := opts.JobsBackendFactory
		if backendFactory == nil {
			backendFactory = jobsfactory.NewBackendWithValidation
		}
		return backendFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
	}

	buildSchedulerLockProvider := func(cfg *config.Config, log logger.Logger) (scheduler.LockProvider, error) {
		lockFactory := opts.SchedulerLockProviderFactory
		if lockFactory != nil {
			return lockFactory(cfg, log)
		}
		return defaultSchedulerLockProviderFactory(cfg, log)
	}

	for _, ext := range opts.ConfigExtensions {
		if err := config.RegisterFlagsFromStruct(rootCmd.PersistentFlags(), ext); err != nil {
			fmt.Fprintf(os.Stderr, "failed to register config flags: %v\n", err)
			os.Exit(1)
		}
	}

	// version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			info := version.Current(opts.Name)
			fmt.Printf("Service:    %s\n", info.Service)
			fmt.Printf("Version:    %s\n", info.Version)
			fmt.Printf("Commit:     %s\n", info.Commit)
			fmt.Printf("Build Time: %s\n", info.BuildTime)
		},
	})
	SetCommandPolicies(rootCmd.Commands()[len(rootCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	// serve command (required)
	if opts.RunServer != nil {
		serveCmd := &cobra.Command{
			Use:   "serve",
			Short: "Start the HTTP server",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.RunServer(cmd.Context(), cfg, log)
			},
		}
		SetCommandPolicies(serveCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyRun})
		rootCmd.AddCommand(serveCmd)
		rootCmd.RunE = serveCmd.RunE
	}

	// migrate command with subcommands (optional)
	if opts.RunMigrations != nil {
		migrateCmd := &cobra.Command{
			Use:   "migrate",
			Short: "Database migration commands",
		}
		SetCommandPolicies(migrateCmd, map[string]CommandPolicy{"migration": PolicyMigration})
		var migrationsPath string
		migrateCmd.PersistentFlags().StringVar(&migrationsPath, "migrations-path", "", "migrations path override")

		upCmd := &cobra.Command{
			Use:   "up",
			Short: "Run pending migrations",
			RunE: func(cmd *cobra.Command, args []string) error {
				if migrationsPath != "" {
					_ = os.Setenv("APP_MIGRATIONS_PATH", migrationsPath)
					_ = os.Setenv("APP_PLATFORM_MIGRATIONS_PATH", migrationsPath)
				}
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.RunMigrations(cmd.Context(), cfg, log, "up", args)
			},
		}
		SetCommandPolicies(upCmd, map[string]CommandPolicy{"migration": PolicyRun})
		migrateCmd.AddCommand(upCmd)

		downCmd := &cobra.Command{
			Use:   "down",
			Short: "Rollback last migration",
			RunE: func(cmd *cobra.Command, args []string) error {
				if migrationsPath != "" {
					_ = os.Setenv("APP_MIGRATIONS_PATH", migrationsPath)
					_ = os.Setenv("APP_PLATFORM_MIGRATIONS_PATH", migrationsPath)
				}
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.RunMigrations(cmd.Context(), cfg, log, "down", args)
			},
		}
		SetCommandPolicies(downCmd, map[string]CommandPolicy{"migration": PolicyOnce})
		migrateCmd.AddCommand(downCmd)

		statusCmd := &cobra.Command{
			Use:   "status",
			Short: "Show migration status",
			RunE: func(cmd *cobra.Command, args []string) error {
				if migrationsPath != "" {
					_ = os.Setenv("APP_MIGRATIONS_PATH", migrationsPath)
					_ = os.Setenv("APP_PLATFORM_MIGRATIONS_PATH", migrationsPath)
				}
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.RunMigrations(cmd.Context(), cfg, log, "status", args)
			},
		}
		SetCommandPolicies(statusCmd, map[string]CommandPolicy{"migration": PolicyRun})
		migrateCmd.AddCommand(statusCmd)

		rootCmd.AddCommand(migrateCmd)
	}

	// cache command (optional)
	if opts.RunCacheClean != nil {
		cacheCmd := &cobra.Command{
			Use:   "cache",
			Short: "Cache management commands",
		}
		SetCommandPolicies(cacheCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyRun})

		var pattern string
		cleanCmd := &cobra.Command{
			Use:   "clean",
			Short: "Clean cache entries",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.RunCacheClean(cmd.Context(), cfg, log, pattern)
			},
		}
		SetCommandPolicies(cleanCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyRun})
		cleanCmd.Flags().StringVarP(&pattern, "pattern", "p", "*", "cache key pattern to clean")
		cacheCmd.AddCommand(cleanCmd)

		rootCmd.AddCommand(cacheCmd)
	}

	// healthcheck command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "healthcheck",
		Short: "Check connectivity to framework and service dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			var errs []error
			if err := runFrameworkDependencyHealthChecks(cmd.Context(), cfg, log, buildJobsRuntime, buildSchedulerLockProvider); err != nil {
				errs = append(errs, err)
			}
			if opts.CheckDependencies != nil {
				if err := opts.CheckDependencies(cmd.Context(), cfg, log); err != nil {
					errs = append(errs, err)
				}
			}
			return errors.Join(errs...)
		},
	})
	SetCommandPolicies(rootCmd.Commands()[len(rootCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	// jobs command
	jobsCmd := &cobra.Command{
		Use:   "jobs",
		Short: "Jobs runtime commands",
	}
	SetCommandPolicies(jobsCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyRun})

	var (
		queues         []string
		concurrency    int
		leaseTTL       time.Duration
		reserveTO      time.Duration
		stopTO         time.Duration
		maxAttempts    int
		initialBackoff time.Duration
		maxBackoff     time.Duration
		attemptTO      time.Duration
		dlqEnabled     bool
		dlqSuffix      string
	)

	workerCmd := &cobra.Command{
		Use:   "worker",
		Short: "Run jobs worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.ConfigureJobsWorker == nil {
				return errors.New("configure jobs worker callback is required for jobs worker command")
			}

			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			backend, err := buildJobsBackend(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs backend: %w", err)
			}
			defer func() {
				if closeErr := backend.Close(); closeErr != nil {
					log.Error("failed to close jobs backend", "error", closeErr)
				}
			}()

			resolvedConcurrency := cfg.Jobs.Worker.Concurrency
			if cmd.Flags().Changed("concurrency") {
				resolvedConcurrency = concurrency
			}
			resolvedLeaseTTL := cfg.Jobs.Worker.LeaseTTL
			if cmd.Flags().Changed("lease-ttl") {
				resolvedLeaseTTL = leaseTTL
			}
			resolvedReserveTO := cfg.Jobs.Worker.ReserveTimeout
			if cmd.Flags().Changed("reserve-timeout") {
				resolvedReserveTO = reserveTO
			}
			resolvedStopTO := cfg.Jobs.Worker.StopTimeout
			if cmd.Flags().Changed("stop-timeout") {
				resolvedStopTO = stopTO
			}
			resolvedMaxAttempts := cfg.Jobs.Retry.MaxAttempts
			if cmd.Flags().Changed("max-attempts") {
				resolvedMaxAttempts = maxAttempts
			}
			resolvedInitialBackoff := cfg.Jobs.Retry.InitialBackoff
			if cmd.Flags().Changed("initial-backoff") {
				resolvedInitialBackoff = initialBackoff
			}
			resolvedMaxBackoff := cfg.Jobs.Retry.MaxBackoff
			if cmd.Flags().Changed("max-backoff") {
				resolvedMaxBackoff = maxBackoff
			}
			resolvedAttemptTO := cfg.Jobs.Retry.AttemptTimeout
			if cmd.Flags().Changed("attempt-timeout") {
				resolvedAttemptTO = attemptTO
			}
			resolvedDLQEnabled := cfg.Jobs.DLQ.Enabled
			if cmd.Flags().Changed("dlq-enabled") {
				resolvedDLQEnabled = dlqEnabled
			}
			resolvedDLQSuffix := cfg.Jobs.DLQ.QueueSuffix
			if cmd.Flags().Changed("dlq-suffix") {
				resolvedDLQSuffix = dlqSuffix
			}

			workerQueues := resolveWorkerQueues(queues, cfg.Jobs.DefaultQueue)
			worker, err := jobs.NewWorker(backend, log, jobs.WorkerConfig{
				Queues:         workerQueues,
				Concurrency:    resolvedConcurrency,
				LeaseTTL:       resolvedLeaseTTL,
				ReserveTimeout: resolvedReserveTO,
				StopTimeout:    resolvedStopTO,
				Retry: jobs.RetryPolicy{
					MaxAttempts:    resolvedMaxAttempts,
					InitialBackoff: resolvedInitialBackoff,
					MaxBackoff:     resolvedMaxBackoff,
					AttemptTimeout: resolvedAttemptTO,
				},
				DLQ: jobs.DLQPolicy{
					Enabled:     resolvedDLQEnabled,
					QueueSuffix: resolvedDLQSuffix,
				},
			})
			if err != nil {
				return fmt.Errorf("create worker: %w", err)
			}
			if err := opts.ConfigureJobsWorker(cfg, log, worker); err != nil {
				return fmt.Errorf("configure jobs worker: %w", err)
			}

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return worker.Start(runCtx)
		},
	}
	SetCommandPolicies(workerCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyScheduled})
	workerCmd.Flags().StringSliceVar(&queues, "queue", []string{}, "queue names to consume (repeatable)")
	workerCmd.Flags().IntVar(&concurrency, "concurrency", 1, "number of workers per queue")
	workerCmd.Flags().DurationVar(&leaseTTL, "lease-ttl", jobs.DefaultLeaseTTL, "lease duration per reserved job")
	workerCmd.Flags().DurationVar(&reserveTO, "reserve-timeout", jobs.DefaultWorkerReserveTimeout, "reserve call timeout")
	workerCmd.Flags().DurationVar(&stopTO, "stop-timeout", jobs.DefaultWorkerStopTimeout, "graceful stop timeout")
	workerCmd.Flags().IntVar(&maxAttempts, "max-attempts", jobs.DefaultWorkerMaxAttempts, "maximum processing attempts per job")
	workerCmd.Flags().DurationVar(&initialBackoff, "initial-backoff", jobs.DefaultWorkerInitialBackoff, "retry initial backoff")
	workerCmd.Flags().DurationVar(&maxBackoff, "max-backoff", jobs.DefaultWorkerMaxBackoff, "retry max backoff")
	workerCmd.Flags().DurationVar(&attemptTO, "attempt-timeout", jobs.DefaultWorkerAttemptTimeout, "timeout for a single job execution")
	workerCmd.Flags().BoolVar(&dlqEnabled, "dlq-enabled", true, "enable dead-letter queue routing")
	workerCmd.Flags().StringVar(&dlqSuffix, "dlq-suffix", jobs.DefaultDLQSuffix, "suffix for dead-letter queues")
	jobsCmd.AddCommand(workerCmd)

	var (
		enqueueName           string
		enqueueQueue          string
		enqueuePayload        string
		enqueueTenantID       string
		enqueueCorrelationID  string
		enqueueIdempotencyKey string
		enqueueRunAt          string
		enqueueMaxAttempts    int
		enqueueHeaders        []string
	)

	enqueueCmd := &cobra.Command{
		Use:   "enqueue",
		Short: "Enqueue a job manually",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			jobsRuntime, err := buildJobsRuntime(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs runtime: %w", err)
			}
			defer func() {
				if closeErr := jobsRuntime.Close(); closeErr != nil {
					log.Error("failed to close jobs runtime", "error", closeErr)
				}
			}()

			jobName := strings.TrimSpace(enqueueName)
			if jobName == "" {
				return errors.New("job name is required")
			}
			queue := strings.TrimSpace(enqueueQueue)
			if queue == "" {
				queue = strings.TrimSpace(cfg.Jobs.DefaultQueue)
			}
			if queue == "" {
				queue = "default"
			}

			payload := strings.TrimSpace(enqueuePayload)
			if payload == "" {
				payload = "{}"
			}

			headers, err := parseStringMapFlag(enqueueHeaders)
			if err != nil {
				return err
			}

			runAt := time.Time{}
			if raw := strings.TrimSpace(enqueueRunAt); raw != "" {
				parsedRunAt, parseErr := time.Parse(time.RFC3339, raw)
				if parseErr != nil {
					return fmt.Errorf("invalid --run-at value %q: %w", raw, parseErr)
				}
				runAt = parsedRunAt.UTC()
			}

			job := &jobs.Job{
				ID:             uuid.NewString(),
				Name:           jobName,
				Queue:          queue,
				Payload:        []byte(payload),
				Headers:        headers,
				ContentType:    jobs.DefaultContentType,
				TenantID:       strings.TrimSpace(enqueueTenantID),
				CorrelationID:  strings.TrimSpace(enqueueCorrelationID),
				IdempotencyKey: strings.TrimSpace(enqueueIdempotencyKey),
				RunAt:          runAt,
				MaxAttempts:    enqueueMaxAttempts,
				CreatedAt:      time.Now().UTC(),
			}
			if err := jobsRuntime.Enqueue(cmd.Context(), job); err != nil {
				return fmt.Errorf("enqueue job: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "enqueued job %s on queue %s\n", job.ID, job.Queue)
			return nil
		},
	}
	SetCommandPolicies(enqueueCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	enqueueCmd.Flags().StringVar(&enqueueName, "name", "", "job name")
	enqueueCmd.Flags().StringVar(&enqueueQueue, "queue", "", "target queue (default: jobs.default_queue)")
	enqueueCmd.Flags().StringVar(&enqueuePayload, "payload", "{}", "job payload as JSON string")
	enqueueCmd.Flags().StringVar(&enqueueTenantID, "tenant-id", "", "tenant id")
	enqueueCmd.Flags().StringVar(&enqueueCorrelationID, "correlation-id", "", "correlation id")
	enqueueCmd.Flags().StringVar(&enqueueIdempotencyKey, "idempotency-key", "", "idempotency key")
	enqueueCmd.Flags().StringVar(&enqueueRunAt, "run-at", "", "schedule run time in RFC3339 format")
	enqueueCmd.Flags().IntVar(&enqueueMaxAttempts, "max-attempts", 0, "max attempts override for this job")
	enqueueCmd.Flags().StringSliceVar(&enqueueHeaders, "header", []string{}, "job header in key=value format (repeatable)")
	_ = enqueueCmd.MarkFlagRequired("name")
	jobsCmd.AddCommand(enqueueCmd)

	dlqCmd := &cobra.Command{
		Use:   "dlq",
		Short: "Dead-letter queue operations",
	}
	SetCommandPolicies(dlqCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})

	var (
		dlqListQueue string
		dlqListLimit int
	)

	dlqListCmd := &cobra.Command{
		Use:   "list",
		Short: "List dead-letter queue entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			backend, err := buildJobsBackend(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs backend: %w", err)
			}
			defer func() {
				if closeErr := backend.Close(); closeErr != nil {
					log.Error("failed to close jobs backend", "error", closeErr)
				}
			}()

			dlqStore, ok := backend.(jobs.DLQStore)
			if !ok {
				return errors.New("dlq operations are not supported by current jobs backend")
			}
			entries, err := dlqStore.ListDLQ(cmd.Context(), dlqListQueue, dlqListLimit)
			if err != nil {
				return fmt.Errorf("list dlq entries: %w", err)
			}
			data, err := json.MarshalIndent(entries, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal dlq entries: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	SetCommandPolicies(dlqListCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	dlqListCmd.Flags().StringVar(&dlqListQueue, "queue", "", "original queue name")
	dlqListCmd.Flags().IntVar(&dlqListLimit, "limit", 50, "max number of entries to list")
	_ = dlqListCmd.MarkFlagRequired("queue")
	dlqCmd.AddCommand(dlqListCmd)

	var (
		dlqReplayQueue string
		dlqReplayIDs   []string
	)

	dlqReplayCmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay dead-letter queue entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			backend, err := buildJobsBackend(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs backend: %w", err)
			}
			defer func() {
				if closeErr := backend.Close(); closeErr != nil {
					log.Error("failed to close jobs backend", "error", closeErr)
				}
			}()

			dlqStore, ok := backend.(jobs.DLQStore)
			if !ok {
				return errors.New("dlq operations are not supported by current jobs backend")
			}
			if len(dlqReplayIDs) == 0 {
				return errors.New("at least one --id value is required")
			}
			replayed, err := dlqStore.ReplayDLQ(cmd.Context(), dlqReplayQueue, dlqReplayIDs)
			if err != nil {
				return fmt.Errorf("replay dlq entries: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "replayed %d entries from queue %s\n", replayed, dlqReplayQueue)
			return nil
		},
	}
	SetCommandPolicies(dlqReplayCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	dlqReplayCmd.Flags().StringVar(&dlqReplayQueue, "queue", "", "original queue name")
	dlqReplayCmd.Flags().StringSliceVar(&dlqReplayIDs, "id", []string{}, "DLQ entry id to replay (repeatable)")
	_ = dlqReplayCmd.MarkFlagRequired("queue")
	dlqCmd.AddCommand(dlqReplayCmd)

	jobsCmd.AddCommand(dlqCmd)
	rootCmd.AddCommand(jobsCmd)

	// scheduler command
	schedulerCmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Distributed scheduler commands",
	}
	SetCommandPolicies(schedulerCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyScheduled})

	var (
		runDispatchTO     time.Duration
		runDefaultLockTTL time.Duration
	)

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run distributed scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			jobsRuntime, err := buildJobsRuntime(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs runtime: %w", err)
			}
			defer func() {
				if closeErr := jobsRuntime.Close(); closeErr != nil {
					log.Error("failed to close jobs runtime", "error", closeErr)
				}
			}()

			lockProvider, err := buildSchedulerLockProvider(cfg, log)
			if err != nil {
				return fmt.Errorf("create scheduler lock provider: %w", err)
			}
			defer func() {
				if closeErr := lockProvider.Close(); closeErr != nil {
					log.Error("failed to close scheduler lock provider", "error", closeErr)
				}
			}()

			runtimeCfg := scheduler.Config{
				DispatchTimeout: cfg.Scheduler.DispatchTimeout,
				DefaultLockTTL:  cfg.Scheduler.LockTTL,
			}
			if cmd.Flags().Changed("dispatch-timeout") {
				runtimeCfg.DispatchTimeout = runDispatchTO
			}
			if cmd.Flags().Changed("default-lock-ttl") {
				runtimeCfg.DefaultLockTTL = runDefaultLockTTL
			}

			runtime, err := scheduler.NewRuntime(jobsRuntime, lockProvider, log, runtimeCfg)
			if err != nil {
				return fmt.Errorf("create scheduler runtime: %w", err)
			}
			if err := registerSchedulerTasksFromConfig(runtime, cfg); err != nil {
				return fmt.Errorf("register scheduler tasks from config: %w", err)
			}
			if opts.ConfigureScheduler != nil {
				if err := opts.ConfigureScheduler(cfg, log, runtime); err != nil {
					return fmt.Errorf("configure scheduler runtime: %w", err)
				}
			}

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runtime.Start(runCtx)
		},
	}
	SetCommandPolicies(runCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyScheduled})
	runCmd.Flags().DurationVar(&runDispatchTO, "dispatch-timeout", scheduler.DefaultDispatchTimeout, "max duration for single scheduled dispatch")
	runCmd.Flags().DurationVar(&runDefaultLockTTL, "default-lock-ttl", scheduler.DefaultLockTTL, "default distributed lock TTL")
	schedulerCmd.AddCommand(runCmd)

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate scheduler tasks and runtime configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}
			tasks, err := schedulerTasksFromConfig(cfg)
			if err != nil {
				return err
			}
			for _, task := range tasks {
				if err := task.Validate(); err != nil {
					return fmt.Errorf("invalid scheduler task %q: %w", task.Name, err)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scheduler configuration is valid (%d task(s))\n", len(tasks))
			return nil
		},
	}
	SetCommandPolicies(validateCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	schedulerCmd.AddCommand(validateCmd)

	var (
		triggerTaskName    string
		triggerDispatchTO  time.Duration
		triggerDefaultLock time.Duration
	)

	triggerCmd := &cobra.Command{
		Use:   "trigger",
		Short: "Trigger one scheduler task manually",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := loadConfig(cmd.Flags())
			if err != nil {
				return err
			}

			jobsRuntime, err := buildJobsRuntime(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs runtime: %w", err)
			}
			defer func() {
				if closeErr := jobsRuntime.Close(); closeErr != nil {
					log.Error("failed to close jobs runtime", "error", closeErr)
				}
			}()

			lockProvider, err := buildSchedulerLockProvider(cfg, log)
			if err != nil {
				return fmt.Errorf("create scheduler lock provider: %w", err)
			}
			defer func() {
				if closeErr := lockProvider.Close(); closeErr != nil {
					log.Error("failed to close scheduler lock provider", "error", closeErr)
				}
			}()

			runtimeCfg := scheduler.Config{
				DispatchTimeout: cfg.Scheduler.DispatchTimeout,
				DefaultLockTTL:  cfg.Scheduler.LockTTL,
			}
			if cmd.Flags().Changed("dispatch-timeout") {
				runtimeCfg.DispatchTimeout = triggerDispatchTO
			}
			if cmd.Flags().Changed("default-lock-ttl") {
				runtimeCfg.DefaultLockTTL = triggerDefaultLock
			}

			runtime, err := scheduler.NewRuntime(jobsRuntime, lockProvider, log, runtimeCfg)
			if err != nil {
				return fmt.Errorf("create scheduler runtime: %w", err)
			}
			if err := registerSchedulerTasksFromConfig(runtime, cfg); err != nil {
				return fmt.Errorf("register scheduler tasks from config: %w", err)
			}
			if opts.ConfigureScheduler != nil {
				if err := opts.ConfigureScheduler(cfg, log, runtime); err != nil {
					return fmt.Errorf("configure scheduler runtime: %w", err)
				}
			}

			taskName := strings.TrimSpace(triggerTaskName)
			if taskName == "" {
				return errors.New("task name is required")
			}
			triggerCtx := cmd.Context()
			if runtimeCfg.DispatchTimeout > 0 {
				var cancel context.CancelFunc
				triggerCtx, cancel = context.WithTimeout(triggerCtx, runtimeCfg.DispatchTimeout)
				defer cancel()
			}
			if err := runtime.Trigger(triggerCtx, taskName); err != nil {
				return fmt.Errorf("trigger scheduler task %q: %w", taskName, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "triggered scheduler task %s\n", taskName)
			return nil
		},
	}
	SetCommandPolicies(triggerCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	triggerCmd.Flags().StringVar(&triggerTaskName, "task", "", "task name to trigger")
	triggerCmd.Flags().DurationVar(&triggerDispatchTO, "dispatch-timeout", scheduler.DefaultDispatchTimeout, "max duration for task dispatch")
	triggerCmd.Flags().DurationVar(&triggerDefaultLock, "default-lock-ttl", scheduler.DefaultLockTTL, "default distributed lock TTL")
	_ = triggerCmd.MarkFlagRequired("task")
	schedulerCmd.AddCommand(triggerCmd)
	rootCmd.AddCommand(schedulerCmd)

	// config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}
	SetCommandPolicies(configCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applySecretFileFlag(opts.EnvPrefix, secretFilePath); err != nil {
				return err
			}
			if opts.ConfigPathResolved != nil {
				opts.ConfigPathResolved(cfgPath)
			}
			cfg := &config.Config{}
			provider := config.NewConfigProvider(cfgPath, opts.EnvPrefix).
				WithServiceNameDefault(opts.Name).
				WithFlags(cmd.Flags())
			if _, err := provider.LoadWithSecrets(cfg, opts.ConfigExtensions...); err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			applyResolvedServiceName(cfg, opts.Name, serviceNameOverride)
			if opts.ValidateConfig != nil {
				if err := opts.ValidateConfig(cfg); err != nil {
					return fmt.Errorf("custom validation failed: %w", err)
				}
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}
			fmt.Println("✓ Configuration is valid")
			return nil
		},
	})
	SetCommandPolicies(configCmd.Commands()[len(configCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	var showSecrets bool
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applySecretFileFlag(opts.EnvPrefix, secretFilePath); err != nil {
				return err
			}
			if opts.ConfigPathResolved != nil {
				opts.ConfigPathResolved(cfgPath)
			}
			cfg := &config.Config{}
			provider := config.NewConfigProvider(cfgPath, opts.EnvPrefix).
				WithServiceNameDefault(opts.Name).
				WithFlags(cmd.Flags())
			secrets, err := provider.LoadWithSecrets(cfg, opts.ConfigExtensions...)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			applyResolvedServiceName(cfg, opts.Name, serviceNameOverride)

			settings := provider.AllSettings()
			settings = setServiceNameSetting(settings, cfg.Service.Name)
			if !showSecrets {
				settings = redactSettingsMap(settings, secrets)
			}
			formatted, err := formatSettings(settings)
			if err != nil {
				return err
			}
			fmt.Print(formatted)
			return nil
		},
	}
	showCmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "show secret values")
	SetCommandPolicies(showCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})
	configCmd.AddCommand(showCmd)

	rootCmd.AddCommand(configCmd)

	// openapi command (optional)
	if openAPICmd := cliopenapi.NewCommand(cliopenapi.CommandOptions{
		RegisterRoutes: opts.RegisterRoutes,
		LoadConfig:     loadConfig,
		ServiceName:    opts.Name,
		ServiceVersion: version.Current(opts.Name).Version,
	}); openAPICmd != nil {
		SetCommandPolicies(openAPICmd, map[string]CommandPolicy{defaultPolicyContext: PolicyOnDemand})
		for _, subcommand := range openAPICmd.Commands() {
			SetCommandPolicies(subcommand, map[string]CommandPolicy{defaultPolicyContext: PolicyOnDemand})
		}
		rootCmd.AddCommand(openAPICmd)
	}

	// Add custom service-specific commands
	for _, customCmd := range opts.CustomCommands {
		ensureDefaultPolicy(customCmd)
		rootCmd.AddCommand(customCmd)
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = false
	rootCmd.InitDefaultCompletionCmd()
	for _, subCmd := range rootCmd.Commands() {
		if subCmd != nil && subCmd.Name() == "completion" {
			SetCommandPolicies(subCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})
			break
		}
	}

	return rootCmd
}

// SetCommandPolicies stores policies as a map[string]string on command annotations using the "policies." prefix.
func SetCommandPolicies(cmd *cobra.Command, policies map[string]CommandPolicy) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	for _, key := range policyAnnotationKeys(cmd.Annotations) {
		delete(cmd.Annotations, key)
	}
	for context, policy := range policies {
		trimmedContext := strings.TrimSpace(context)
		if trimmedContext == "" {
			continue
		}
		cmd.Annotations[policiesAnnotationPrefix+trimmedContext] = string(policy)
	}
}

// GetCommandPolicies returns command policies from annotations.
func GetCommandPolicies(cmd *cobra.Command) map[string]string {
	out := map[string]string{}
	if cmd == nil {
		return out
	}
	for key, value := range cmd.Annotations {
		if !strings.HasPrefix(key, policiesAnnotationPrefix) {
			continue
		}
		context := strings.TrimPrefix(key, policiesAnnotationPrefix)
		if strings.TrimSpace(context) == "" {
			continue
		}
		out[context] = value
	}
	return out
}

func ensureDefaultPolicy(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if len(GetCommandPolicies(cmd)) == 0 {
		SetCommandPolicies(cmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})
	}
}

func policyAnnotationKeys(annotations map[string]string) []string {
	keys := make([]string, 0, len(annotations))
	for key := range annotations {
		if strings.HasPrefix(key, policiesAnnotationPrefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func resolveWorkerQueues(flagQueues []string, configDefault string) []string {
	queues := make([]string, 0, len(flagQueues)+1)
	for _, queue := range flagQueues {
		trimmed := strings.TrimSpace(queue)
		if trimmed != "" {
			queues = append(queues, trimmed)
		}
	}
	if len(queues) > 0 {
		return queues
	}
	if trimmed := strings.TrimSpace(configDefault); trimmed != "" {
		return []string{trimmed}
	}
	return []string{"default"}
}

func parseStringMapFlag(raw []string) (map[string]string, error) {
	out := map[string]string{}
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header %q: expected key=value", item)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid header %q: key is required", item)
		}
		out[key] = strings.TrimSpace(parts[1])
	}
	return out, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func runFrameworkDependencyHealthChecks(
	ctx context.Context,
	cfg *config.Config,
	log logger.Logger,
	buildJobsRuntime func(*config.Config, logger.Logger) (jobs.Runtime, error),
	buildSchedulerLockProvider func(*config.Config, logger.Logger) (scheduler.LockProvider, error),
) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	var errs []error

	var jobsRuntime jobs.Runtime
	if shouldCheckJobsRuntimeHealth(cfg) {
		runtime, err := buildJobsRuntime(cfg, log)
		if err != nil {
			errs = append(errs, fmt.Errorf("create jobs runtime for healthcheck: %w", err))
		} else {
			jobsRuntime = runtime
			checker := jobs.NewRuntimeHealthChecker("", jobsRuntime, jobsHealthTimeout(cfg))
			if checkErr := healthCheckResultError(checker.Check(ctx)); checkErr != nil {
				errs = append(errs, checkErr)
			}
		}
	}
	if jobsRuntime != nil {
		if closeErr := jobsRuntime.Close(); closeErr != nil {
			errs = append(errs, fmt.Errorf("close jobs runtime after healthcheck: %w", closeErr))
		}
	}

	if shouldCheckSchedulerLockHealth(cfg) {
		lockProvider, err := buildSchedulerLockProvider(cfg, log)
		if err != nil {
			errs = append(errs, fmt.Errorf("create scheduler lock provider for healthcheck: %w", err))
		} else {
			checker := scheduler.NewLockProviderHealthChecker("", lockProvider, schedulerLockHealthTimeout(cfg))
			if checkErr := healthCheckResultError(checker.Check(ctx)); checkErr != nil {
				errs = append(errs, checkErr)
			}
			if closeErr := lockProvider.Close(); closeErr != nil {
				errs = append(errs, fmt.Errorf("close scheduler lock provider after healthcheck: %w", closeErr))
			}
		}
	}

	return errors.Join(errs...)
}

func shouldCheckJobsRuntimeHealth(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.Scheduler.Enabled {
		return true
	}

	backend := strings.ToLower(strings.TrimSpace(cfg.Jobs.Backend))
	switch backend {
	case config.JobsBackendRedis:
		return strings.TrimSpace(cfg.Jobs.Redis.URL) != ""
	case config.JobsBackendEventBus:
		return strings.TrimSpace(cfg.EventBus.Type) != ""
	default:
		return false
	}
}

func shouldCheckSchedulerLockHealth(cfg *config.Config) bool {
	return cfg != nil && cfg.Scheduler.Enabled
}

func jobsHealthTimeout(cfg *config.Config) time.Duration {
	if cfg == nil {
		return 3 * time.Second
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Jobs.Backend), config.JobsBackendRedis) && cfg.Jobs.Redis.OperationTimeout > 0 {
		return cfg.Jobs.Redis.OperationTimeout
	}
	if cfg.EventBus.OperationTimeout > 0 {
		return cfg.EventBus.OperationTimeout
	}
	return 3 * time.Second
}

func schedulerLockHealthTimeout(cfg *config.Config) time.Duration {
	if cfg == nil {
		return 3 * time.Second
	}
	lockProvider := strings.ToLower(strings.TrimSpace(cfg.Scheduler.LockProvider))
	switch lockProvider {
	case config.SchedulerLockProviderPostgres:
		if cfg.Scheduler.Postgres.OperationTimeout > 0 {
			return cfg.Scheduler.Postgres.OperationTimeout
		}
	default:
		if cfg.Scheduler.Redis.OperationTimeout > 0 {
			return cfg.Scheduler.Redis.OperationTimeout
		}
	}
	return 3 * time.Second
}

func healthCheckResultError(result health.CheckResult) error {
	if result.Status == health.StatusHealthy {
		return nil
	}
	msg := strings.TrimSpace(result.Error)
	if msg == "" {
		msg = strings.TrimSpace(result.Message)
	}
	if msg == "" {
		msg = "health check failed"
	}
	return fmt.Errorf("%s: %s", result.Name, msg)
}

func schedulerTasksFromConfig(cfg *config.Config) ([]scheduler.Task, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	defaultTimezone := strings.TrimSpace(cfg.Scheduler.Timezone)
	tasks := make([]scheduler.Task, 0, len(cfg.Scheduler.Tasks))
	seen := map[string]struct{}{}

	for idx, configured := range cfg.Scheduler.Tasks {
		name := strings.TrimSpace(configured.Name)
		if name == "" {
			return nil, fmt.Errorf("scheduler.tasks[%d].name is required", idx)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("scheduler.tasks contains duplicate name %q", name)
		}
		seen[name] = struct{}{}

		payload := strings.TrimSpace(configured.Payload)
		if payload == "" {
			payload = "{}"
		}

		timezone := strings.TrimSpace(configured.Timezone)
		if timezone == "" {
			timezone = defaultTimezone
		}

		task := scheduler.Task{
			Name:           name,
			Schedule:       strings.TrimSpace(configured.Cron),
			Queue:          strings.TrimSpace(configured.Queue),
			JobName:        strings.TrimSpace(configured.JobName),
			Payload:        []byte(payload),
			Headers:        cloneStringMap(configured.Headers),
			TenantID:       strings.TrimSpace(configured.TenantID),
			IdempotencyKey: strings.TrimSpace(configured.IdempotencyKey),
			Timezone:       timezone,
			LockTTL:        configured.LockTTL,
			MisfirePolicy:  strings.TrimSpace(configured.MisfirePolicy),
		}
		if err := task.Validate(); err != nil {
			return nil, fmt.Errorf("scheduler.tasks[%s]: %w", name, err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func registerSchedulerTasksFromConfig(runtime *scheduler.Runtime, cfg *config.Config) error {
	if runtime == nil {
		return errors.New("scheduler runtime is required")
	}
	tasks, err := schedulerTasksFromConfig(cfg)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := runtime.Register(task); err != nil {
			return err
		}
	}
	return nil
}

func defaultSchedulerLockProviderFactory(cfg *config.Config, log logger.Logger) (scheduler.LockProvider, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	lockProvider := strings.ToLower(strings.TrimSpace(cfg.Scheduler.LockProvider))
	if lockProvider == "" {
		lockProvider = config.SchedulerLockProviderRedis
	}

	switch lockProvider {
	case config.SchedulerLockProviderRedis:
		redisURL := strings.TrimSpace(cfg.Scheduler.Redis.URL)
		if redisURL == "" {
			redisURL = strings.TrimSpace(cfg.Cache.URL)
		}
		if redisURL == "" {
			return nil, errors.New("scheduler redis url is required (set scheduler.redis.url or cache.url)")
		}
		return scheduler.NewRedisLockProvider(scheduler.RedisLockProviderConfig{
			URL:              redisURL,
			Prefix:           cfg.Scheduler.Redis.Prefix,
			OperationTimeout: cfg.Scheduler.Redis.OperationTimeout,
		}, log)
	case config.SchedulerLockProviderPostgres:
		postgresURL := strings.TrimSpace(cfg.Scheduler.Postgres.URL)
		if postgresURL == "" {
			postgresURL = strings.TrimSpace(cfg.Database.URL)
		}
		if postgresURL == "" {
			return nil, errors.New("scheduler postgres url is required (set scheduler.postgres.url or database.url)")
		}
		return scheduler.NewPostgresLockProvider(scheduler.PostgresLockProviderConfig{
			URL:              postgresURL,
			Table:            cfg.Scheduler.Postgres.Table,
			OperationTimeout: cfg.Scheduler.Postgres.OperationTimeout,
		}, log)
	default:
		return nil, fmt.Errorf("unsupported scheduler.lock_provider %q", cfg.Scheduler.LockProvider)
	}
}

func LoadConfigAndLogger(
	cfgPath,
	envPrefix,
	secretFilePath string,
	customValidator func(*config.Config) error,
	flags *pflag.FlagSet,
	extensions []any,
	defaultServiceName string,
	serviceNameOverride string,
) (*config.Config, logger.Logger, error) {
	if envPrefix == "" {
		envPrefix = "APP"
	}
	if err := applySecretFileFlag(envPrefix, secretFilePath); err != nil {
		return nil, nil, err
	}
	cfg := &config.Config{}
	provider := config.NewConfigProvider(cfgPath, envPrefix).
		WithServiceNameDefault(defaultServiceName).
		WithFlags(flags)
	if _, err := provider.LoadWithSecrets(cfg, extensions...); err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	applyResolvedServiceName(cfg, defaultServiceName, serviceNameOverride)

	// Run custom validation if provided (Nimburion's validation already ran in Load())
	if customValidator != nil {
		if err := customValidator(cfg); err != nil {
			return nil, nil, fmt.Errorf("custom validation failed: %w", err)
		}
	}

	logCfg := logger.Config{
		Level:  logger.LogLevel(cfg.Observability.LogLevel),
		Format: logger.LogFormat(cfg.Observability.LogFormat),
	}
	log, err := logger.NewZapLogger(logCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create logger: %w", err)
	}

	logConfigIfDebug(log, cfg)
	return cfg, log, nil
}

func applySecretFileFlag(envPrefix, secretFilePath string) error {
	if secretFilePath == "" {
		return nil
	}
	info, err := os.Stat(secretFilePath)
	if err != nil {
		return fmt.Errorf("secret file %s is not accessible: %w", secretFilePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("secret file %s must not be a directory", secretFilePath)
	}
	return os.Setenv(resolveEnvPrefix(envPrefix)+"_SECRETS_FILE", filepath.Clean(secretFilePath))
}

func formatSettings(settings map[string]interface{}) (string, error) {
	if settings == nil {
		return "{}\n", nil
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(data), nil
}

func redactSettingsMap(settings, secrets map[string]interface{}) map[string]interface{} {
	if len(settings) == 0 || len(secrets) == 0 {
		return settings
	}
	out := make(map[string]interface{}, len(settings))
	for key, value := range settings {
		mask, ok := secrets[key]
		if !ok {
			out[key] = value
			continue
		}
		out[key] = redactSettingValue(value, mask)
	}
	return out
}

func redactSettingValue(value, mask interface{}) interface{} {
	maskMap, maskIsMap := mask.(map[string]interface{})
	if maskIsMap {
		valueMap, valueIsMap := value.(map[string]interface{})
		if !valueIsMap {
			if shouldRedactSetting(mask) {
				return "***"
			}
			return value
		}
		out := make(map[string]interface{}, len(valueMap))
		for key, item := range valueMap {
			childMask, ok := maskMap[key]
			if !ok {
				out[key] = item
				continue
			}
			out[key] = redactSettingValue(item, childMask)
		}
		return out
	}
	if shouldRedactSetting(mask) {
		return "***"
	}
	return value
}

func shouldRedactSetting(mask interface{}) bool {
	if mask == nil {
		return false
	}
	switch value := mask.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case bool:
		return value
	case int:
		return value != 0
	case int8:
		return value != 0
	case int16:
		return value != 0
	case int32:
		return value != 0
	case int64:
		return value != 0
	case uint:
		return value != 0
	case uint8:
		return value != 0
	case uint16:
		return value != 0
	case uint32:
		return value != 0
	case uint64:
		return value != 0
	case float32:
		return value != 0
	case float64:
		return value != 0
	case []interface{}:
		return len(value) > 0
	case map[string]interface{}:
		return len(value) > 0
	default:
		return !reflect.ValueOf(mask).IsZero()
	}
}

// Execute runs the command and exits with appropriate code.
func Execute(cmd *cobra.Command) {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func logConfigIfDebug(log logger.Logger, cfg *config.Config) {
	if log == nil || cfg == nil {
		return
	}

	if !strings.EqualFold(cfg.Observability.LogLevel, string(logger.DebugLevel)) {
		return
	}

	log.Debug("effective configuration", "config", fmt.Sprintf("%+v", cfg))
}

func resolveEnvPrefix(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return "APP"
	}
	return strings.ToUpper(trimmed)
}

func applyResolvedServiceName(cfg *config.Config, defaultServiceName, serviceNameOverride string) {
	if cfg == nil {
		return
	}
	cfg.Service.Name = resolveServiceNameValue(cfg.Service.Name, defaultServiceName, serviceNameOverride)
}

func resolveServiceNameValue(currentConfigName, defaultServiceName, serviceNameOverride string) string {
	if override := strings.TrimSpace(serviceNameOverride); override != "" {
		return override
	}
	if configured := strings.TrimSpace(currentConfigName); configured != "" {
		return configured
	}
	if fallback := strings.TrimSpace(defaultServiceName); fallback != "" {
		return fallback
	}
	return "app"
}

func setServiceNameSetting(settings map[string]interface{}, serviceName string) map[string]interface{} {
	if settings == nil {
		settings = map[string]interface{}{}
	}
	service, ok := settings["service"].(map[string]interface{})
	if !ok || service == nil {
		service = map[string]interface{}{}
	}
	service["name"] = serviceName
	settings["service"] = service
	return settings
}
