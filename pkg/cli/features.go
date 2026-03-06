package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	cliopenapi "github.com/nimburion/nimburion/pkg/cli/openapi"
	"github.com/nimburion/nimburion/pkg/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/jobs"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/scheduler"
	"github.com/nimburion/nimburion/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type configLoader func(flags *pflag.FlagSet) (*config.Config, logger.Logger, error)
type jobsRuntimeBuilder func(cfg *config.Config, log logger.Logger) (jobs.Runtime, error)
type jobsBackendBuilder func(cfg *config.Config, log logger.Logger) (jobs.Backend, error)
type schedulerLockProviderBuilder func(cfg *config.Config, log logger.Logger) (scheduler.LockProvider, error)

type commandFeature struct {
	name     string
	commands []*cobra.Command
}

func (f commandFeature) Name() string { return f.name }

func (f commandFeature) Contributions() corefeature.Contributions {
	contributions := make([]corefeature.CommandContribution, 0, len(f.commands))
	for _, cmd := range f.commands {
		if cmd == nil {
			continue
		}
		contributions = append(contributions, corefeature.CommandContribution{
			Name:    cmd.Name(),
			Command: cmd,
		})
	}
	return corefeature.Contributions{CommandRegistrations: contributions}
}

func newBuiltInCommandFeatures(
	opts AppCommandOptions,
	loadConfig configLoader,
	buildJobsRuntime jobsRuntimeBuilder,
	buildJobsBackend jobsBackendBuilder,
	buildSchedulerLockProvider schedulerLockProviderBuilder,
) []corefeature.Feature {
	return []corefeature.Feature{
		newMigrateCommandFeature(opts, loadConfig),
		newJobsCommandFeature(opts, loadConfig, buildJobsRuntime, buildJobsBackend),
		newSchedulerCommandFeature(opts, loadConfig, buildJobsRuntime, buildSchedulerLockProvider),
		newOpenAPICommandFeature(opts, loadConfig),
	}
}

func newMigrateCommandFeature(opts AppCommandOptions, loadConfig configLoader) corefeature.Feature {
	if opts.RunMigrations == nil {
		return nil
	}

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}
	SetCommandPolicies(migrateCmd, map[string]CommandPolicy{"migration": PolicyMigration})
	var migrationsPath string
	migrateCmd.PersistentFlags().StringVar(&migrationsPath, "migrations-path", "", "migrations path override")

	addMigrationSubcommand := func(use, short, direction string, policy CommandPolicy) {
		subCmd := &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := setMigrationPathEnv(migrationsPath); err != nil {
					return err
				}
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.RunMigrations(cmd.Context(), cfg, log, direction, args)
			},
		}
		SetCommandPolicies(subCmd, map[string]CommandPolicy{"migration": policy})
		migrateCmd.AddCommand(subCmd)
	}

	addMigrationSubcommand("up", "Run pending migrations", "up", PolicyRun)
	addMigrationSubcommand("down", "Rollback last migration", "down", PolicyOnce)
	addMigrationSubcommand("status", "Show migration status", "status", PolicyRun)

	return commandFeature{name: "migrate", commands: []*cobra.Command{migrateCmd}}
}

func newJobsCommandFeature(
	opts AppCommandOptions,
	loadConfig configLoader,
	buildJobsRuntime jobsRuntimeBuilder,
	buildJobsBackend jobsBackendBuilder,
) corefeature.Feature {
	if !opts.IncludeJobs {
		return nil
	}

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
			if err := writeCommandf(cmd, "enqueued job %s on queue %s\n", job.ID, job.Queue); err != nil {
				return fmt.Errorf("write enqueue output: %w", err)
			}
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
	mustMarkFlagRequired(enqueueCmd, "name")
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
			return writeCommandln(cmd, string(data))
		},
	}
	SetCommandPolicies(dlqListCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	dlqListCmd.Flags().StringVar(&dlqListQueue, "queue", "", "original queue name")
	dlqListCmd.Flags().IntVar(&dlqListLimit, "limit", 50, "max number of entries to list")
	mustMarkFlagRequired(dlqListCmd, "queue")
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
			return writeCommandf(cmd, "replayed %d entries from queue %s\n", replayed, dlqReplayQueue)
		},
	}
	SetCommandPolicies(dlqReplayCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	dlqReplayCmd.Flags().StringVar(&dlqReplayQueue, "queue", "", "original queue name")
	dlqReplayCmd.Flags().StringSliceVar(&dlqReplayIDs, "id", []string{}, "DLQ entry id to replay (repeatable)")
	mustMarkFlagRequired(dlqReplayCmd, "queue")
	dlqCmd.AddCommand(dlqReplayCmd)

	jobsCmd.AddCommand(dlqCmd)
	return commandFeature{name: "jobs", commands: []*cobra.Command{jobsCmd}}
}

func newSchedulerCommandFeature(
	opts AppCommandOptions,
	loadConfig configLoader,
	buildJobsRuntime jobsRuntimeBuilder,
	buildSchedulerLockProvider schedulerLockProviderBuilder,
) corefeature.Feature {
	if !opts.IncludeScheduler {
		return nil
	}

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
			return writeCommandf(cmd, "scheduler configuration is valid (%d task(s))\n", len(tasks))
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
			return writeCommandf(cmd, "triggered scheduler task %s\n", taskName)
		},
	}
	SetCommandPolicies(triggerCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyManual})
	triggerCmd.Flags().StringVar(&triggerTaskName, "task", "", "task name to trigger")
	triggerCmd.Flags().DurationVar(&triggerDispatchTO, "dispatch-timeout", scheduler.DefaultDispatchTimeout, "max duration for task dispatch")
	triggerCmd.Flags().DurationVar(&triggerDefaultLock, "default-lock-ttl", scheduler.DefaultLockTTL, "default distributed lock TTL")
	mustMarkFlagRequired(triggerCmd, "task")
	schedulerCmd.AddCommand(triggerCmd)

	return commandFeature{name: "scheduler", commands: []*cobra.Command{schedulerCmd}}
}

func newOpenAPICommandFeature(opts AppCommandOptions, loadConfig configLoader) corefeature.Feature {
	if !opts.IncludeOpenAPI {
		return nil
	}
	openAPICmd := cliopenapi.NewCommand(cliopenapi.CommandOptions{
		RegisterRoutes: opts.RegisterRoutes,
		LoadConfig:     cliopenapi.ConfigLoader(loadConfig),
		ServiceName:    opts.Name,
		ServiceVersion: version.Current(opts.Name).Version,
	})
	if openAPICmd == nil {
		return nil
	}
	SetCommandPolicies(openAPICmd, map[string]CommandPolicy{defaultPolicyContext: PolicyOnDemand})
	for _, subcommand := range openAPICmd.Commands() {
		SetCommandPolicies(subcommand, map[string]CommandPolicy{defaultPolicyContext: PolicyOnDemand})
	}
	return commandFeature{name: "openapi", commands: []*cobra.Command{openAPICmd}}
}
