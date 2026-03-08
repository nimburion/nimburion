package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/coordination"
	coordinationpostgres "github.com/nimburion/nimburion/pkg/coordination/postgres"
	coordinationredis "github.com/nimburion/nimburion/pkg/coordination/redis"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	schedulerfamilyconfig "github.com/nimburion/nimburion/pkg/scheduler/config"
	"github.com/spf13/cobra"
)

const (
	commandPolicyAnnotationPrefix = "policies."
	defaultCommandPolicyContext   = "run"
	policyManual                  = "manual"
	policyScheduled               = "scheduled"
)

// ConfigLoader loads config and logger for contributed scheduler commands.
type ConfigLoader func(cmd *cobra.Command) (*config.Config, logger.Logger, error)

// JobsRuntimeFactory creates the jobs runtime used by scheduler commands.
type JobsRuntimeFactory func(
	cfg jobsconfig.Config,
	eventBusCfg eventbusconfig.Config,
	validationCfg schemavalidationconfig.KafkaValidationConfig,
	log logger.Logger,
) (jobs.Runtime, error)

// LockProviderFactory creates the distributed scheduler lock provider.
type LockProviderFactory func(cfg *config.Config, log logger.Logger) (coordination.LockProvider, error)

// RuntimeConfigurer customizes a scheduler runtime before execution.
type RuntimeConfigurer func(cfg *config.Config, log logger.Logger, runtime *Runtime) error

// CommandFeatureOptions defines the inputs required to contribute scheduler commands.
type CommandFeatureOptions struct {
	LoadConfig          ConfigLoader
	JobsRuntimeFactory  JobsRuntimeFactory
	LockProviderFactory LockProviderFactory
	ConfigureRuntime    RuntimeConfigurer
}

type commandFeature struct {
	command *cobra.Command
}

func (f commandFeature) Name() string { return "scheduler" }

func (f commandFeature) Contributions() corefeature.Contributions {
	if f.command == nil {
		return corefeature.Contributions{}
	}
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "scheduler", Extension: &schedulerfamilyconfig.Extension{}},
		},
		CommandRegistrations: []corefeature.CommandContribution{
			{Name: f.command.Name(), Command: f.command},
		},
	}
}

// NewCommandFeature contributes the scheduler command tree owned by the scheduler family.
func NewCommandFeature(opts CommandFeatureOptions) corefeature.Feature {
	if opts.LoadConfig == nil {
		return nil
	}

	buildJobsRuntime := func(cfg *config.Config, log logger.Logger) (jobs.Runtime, error) {
		runtimeFactory := opts.JobsRuntimeFactory
		if runtimeFactory == nil {
			runtimeFactory = jobs.NewRuntimeFromConfigWithValidation
		}
		return runtimeFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
	}

	buildLockProvider := func(cfg *config.Config, log logger.Logger) (coordination.LockProvider, error) {
		lockFactory := opts.LockProviderFactory
		if lockFactory != nil {
			return lockFactory(cfg, log)
		}
		return defaultLockProviderFactory(cfg, log)
	}

	schedulerCmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Distributed scheduler commands",
	}
	setCommandPolicy(schedulerCmd, policyScheduled)

	var (
		runDispatchTO     time.Duration
		runDefaultLockTTL time.Duration
	)
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run distributed scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			jobsRuntime, err := buildJobsRuntime(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs runtime: %w", err)
			}
			defer func() { _ = jobsRuntime.Close() }()
			lockProvider, err := buildLockProvider(cfg, log)
			if err != nil {
				return fmt.Errorf("create scheduler lock provider: %w", err)
			}
			defer func() { _ = lockProvider.Close() }()

			runtimeCfg := Config{
				DispatchTimeout: cfg.Scheduler.DispatchTimeout,
				DefaultLockTTL:  cfg.Scheduler.LockTTL,
			}
			if cmd.Flags().Changed("dispatch-timeout") {
				runtimeCfg.DispatchTimeout = runDispatchTO
			}
			if cmd.Flags().Changed("default-lock-ttl") {
				runtimeCfg.DefaultLockTTL = runDefaultLockTTL
			}

			runtime, err := NewRuntime(jobsRuntime, lockProvider, log, runtimeCfg)
			if err != nil {
				return fmt.Errorf("create scheduler runtime: %w", err)
			}
			if err := registerTasksFromConfig(runtime, cfg); err != nil {
				return fmt.Errorf("register scheduler tasks from config: %w", err)
			}
			if opts.ConfigureRuntime != nil {
				if err := opts.ConfigureRuntime(cfg, log, runtime); err != nil {
					return fmt.Errorf("configure scheduler runtime: %w", err)
				}
			}
			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runtime.Start(runCtx)
		},
	}
	setCommandPolicy(runCmd, policyScheduled)
	runCmd.Flags().DurationVar(&runDispatchTO, "dispatch-timeout", DefaultDispatchTimeout, "max duration for single scheduled dispatch")
	runCmd.Flags().DurationVar(&runDefaultLockTTL, "default-lock-ttl", DefaultLockTTL, "default distributed lock TTL")
	schedulerCmd.AddCommand(runCmd)

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate scheduler tasks and runtime configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			tasks, err := tasksFromConfig(cfg)
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
	setCommandPolicy(validateCmd, policyManual)
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
			cfg, log, err := opts.LoadConfig(cmd)
			if err != nil {
				return err
			}
			jobsRuntime, err := buildJobsRuntime(cfg, log)
			if err != nil {
				return fmt.Errorf("create jobs runtime: %w", err)
			}
			defer func() { _ = jobsRuntime.Close() }()
			lockProvider, err := buildLockProvider(cfg, log)
			if err != nil {
				return fmt.Errorf("create scheduler lock provider: %w", err)
			}
			defer func() { _ = lockProvider.Close() }()

			runtimeCfg := Config{
				DispatchTimeout: cfg.Scheduler.DispatchTimeout,
				DefaultLockTTL:  cfg.Scheduler.LockTTL,
			}
			if cmd.Flags().Changed("dispatch-timeout") {
				runtimeCfg.DispatchTimeout = triggerDispatchTO
			}
			if cmd.Flags().Changed("default-lock-ttl") {
				runtimeCfg.DefaultLockTTL = triggerDefaultLock
			}
			runtime, err := NewRuntime(jobsRuntime, lockProvider, log, runtimeCfg)
			if err != nil {
				return fmt.Errorf("create scheduler runtime: %w", err)
			}
			if err := registerTasksFromConfig(runtime, cfg); err != nil {
				return fmt.Errorf("register scheduler tasks from config: %w", err)
			}
			if opts.ConfigureRuntime != nil {
				if err := opts.ConfigureRuntime(cfg, log, runtime); err != nil {
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
	setCommandPolicy(triggerCmd, policyManual)
	triggerCmd.Flags().StringVar(&triggerTaskName, "task", "", "task name to trigger")
	triggerCmd.Flags().DurationVar(&triggerDispatchTO, "dispatch-timeout", DefaultDispatchTimeout, "max duration for task dispatch")
	triggerCmd.Flags().DurationVar(&triggerDefaultLock, "default-lock-ttl", DefaultLockTTL, "default distributed lock TTL")
	_ = triggerCmd.MarkFlagRequired("task")
	schedulerCmd.AddCommand(triggerCmd)

	return commandFeature{command: schedulerCmd}
}

func tasksFromConfig(cfg *config.Config) ([]Task, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	defaultTimezone := strings.TrimSpace(cfg.Scheduler.Timezone)
	tasks := make([]Task, 0, len(cfg.Scheduler.Tasks))
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
		task := Task{
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

func registerTasksFromConfig(runtime *Runtime, cfg *config.Config) error {
	if runtime == nil {
		return errors.New("scheduler runtime is required")
	}
	tasks, err := tasksFromConfig(cfg)
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

func defaultLockProviderFactory(cfg *config.Config, log logger.Logger) (coordination.LockProvider, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	lockProvider := strings.ToLower(strings.TrimSpace(cfg.Scheduler.LockProvider))
	if lockProvider == "" {
		lockProvider = schedulerfamilyconfig.LockProviderRedis
	}
	switch lockProvider {
	case schedulerfamilyconfig.LockProviderRedis:
		redisURL := strings.TrimSpace(cfg.Scheduler.Redis.URL)
		if redisURL == "" {
			redisURL = strings.TrimSpace(cfg.Cache.URL)
		}
		if redisURL == "" {
			return nil, errors.New("scheduler redis url is required (set scheduler.redis.url or cache.url)")
		}
		return coordinationredis.NewRedisLockProvider(coordinationredis.RedisLockProviderConfig{
			URL:              redisURL,
			Prefix:           cfg.Scheduler.Redis.Prefix,
			OperationTimeout: cfg.Scheduler.Redis.OperationTimeout,
		}, log)
	case schedulerfamilyconfig.LockProviderPostgres:
		postgresURL := strings.TrimSpace(cfg.Scheduler.Postgres.URL)
		if postgresURL == "" {
			postgresURL = strings.TrimSpace(cfg.Database.URL)
		}
		if postgresURL == "" {
			return nil, errors.New("scheduler postgres url is required (set scheduler.postgres.url or database.url)")
		}
		return coordinationpostgres.NewPostgresLockProvider(coordinationpostgres.PostgresLockProviderConfig{
			URL:              postgresURL,
			Table:            cfg.Scheduler.Postgres.Table,
			OperationTimeout: cfg.Scheduler.Postgres.OperationTimeout,
		}, log)
	default:
		return nil, fmt.Errorf("unsupported scheduler.lock_provider %q", cfg.Scheduler.LockProvider)
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func writeCommandf(cmd *cobra.Command, format string, args ...any) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), format, args...)
	return err
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
