package cli

import (
	"context"
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

	cliopenapi "github.com/nimburion/nimburion/pkg/cli/openapi"
	"github.com/nimburion/nimburion/pkg/config"
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

	// healthcheck command (optional)
	if opts.CheckDependencies != nil {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "healthcheck",
			Short: "Check connectivity to dependencies (database, cache, eventbus, IDP)",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}
				return opts.CheckDependencies(cmd.Context(), cfg, log)
			},
		})
		SetCommandPolicies(rootCmd.Commands()[len(rootCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})
	}

	// jobs command (optional)
	if opts.ConfigureJobsWorker != nil {
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
			bufferSize     int
		)

		workerCmd := &cobra.Command{
			Use:   "worker",
			Short: "Run jobs worker",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}

				runtimeFactory := opts.JobsRuntimeFactory
				if runtimeFactory == nil {
					runtimeFactory = jobsfactory.NewRuntimeWithValidation
				}
				jobsRuntime, err := runtimeFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
				if err != nil {
					return fmt.Errorf("create jobs runtime: %w", err)
				}
				defer func() {
					if closeErr := jobsRuntime.Close(); closeErr != nil {
						log.Error("failed to close jobs runtime", "error", closeErr)
					}
				}()

				backend, err := jobs.NewRuntimeBackend(jobsRuntime, log, jobs.RuntimeBackendConfig{
					BufferSize: bufferSize,
					DLQSuffix:  dlqSuffix,
				})
				if err != nil {
					return fmt.Errorf("create jobs backend: %w", err)
				}
				defer func() {
					if closeErr := backend.Close(); closeErr != nil {
						log.Error("failed to close jobs backend", "error", closeErr)
					}
				}()

				workerQueues := resolveWorkerQueues(queues, cfg.Jobs.DefaultQueue)
				worker, err := jobs.NewWorker(backend, log, jobs.WorkerConfig{
					Queues:         workerQueues,
					Concurrency:    concurrency,
					LeaseTTL:       leaseTTL,
					ReserveTimeout: reserveTO,
					StopTimeout:    stopTO,
					Retry: jobs.RetryPolicy{
						MaxAttempts:    maxAttempts,
						InitialBackoff: initialBackoff,
						MaxBackoff:     maxBackoff,
						AttemptTimeout: attemptTO,
					},
					DLQ: jobs.DLQPolicy{
						Enabled:     dlqEnabled,
						QueueSuffix: dlqSuffix,
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
		workerCmd.Flags().IntVar(&bufferSize, "buffer-size", 128, "in-memory reserve buffer size")
		jobsCmd.AddCommand(workerCmd)
		rootCmd.AddCommand(jobsCmd)
	}

	// scheduler command (optional)
	if opts.ConfigureScheduler != nil {
		schedulerCmd := &cobra.Command{
			Use:   "scheduler",
			Short: "Distributed scheduler commands",
		}
		SetCommandPolicies(schedulerCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyScheduled})

		var (
			dispatchTO       time.Duration
			defaultLockTTL   time.Duration
			redisURL         string
			redisPrefix      string
			redisOperationTO time.Duration
		)

		runCmd := &cobra.Command{
			Use:   "run",
			Short: "Run distributed scheduler",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, log, err := loadConfig(cmd.Flags())
				if err != nil {
					return err
				}

				runtimeFactory := opts.JobsRuntimeFactory
				if runtimeFactory == nil {
					runtimeFactory = jobsfactory.NewRuntimeWithValidation
				}
				jobsRuntime, err := runtimeFactory(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
				if err != nil {
					return fmt.Errorf("create jobs runtime: %w", err)
				}
				defer func() {
					if closeErr := jobsRuntime.Close(); closeErr != nil {
						log.Error("failed to close jobs runtime", "error", closeErr)
					}
				}()

				lockFactory := opts.SchedulerLockProviderFactory
				if lockFactory == nil {
					lockFactory = func(runtimeCfg *config.Config, runtimeLog logger.Logger) (scheduler.LockProvider, error) {
						url := strings.TrimSpace(redisURL)
						if url == "" {
							url = strings.TrimSpace(runtimeCfg.Cache.URL)
						}
						if url == "" {
							return nil, errors.New("redis url is required (set --redis-url or cache.url)")
						}
						return scheduler.NewRedisLockProvider(scheduler.RedisLockProviderConfig{
							URL:              url,
							Prefix:           redisPrefix,
							OperationTimeout: redisOperationTO,
						}, runtimeLog)
					}
				}

				lockProvider, err := lockFactory(cfg, log)
				if err != nil {
					return fmt.Errorf("create scheduler lock provider: %w", err)
				}
				defer func() {
					if closeErr := lockProvider.Close(); closeErr != nil {
						log.Error("failed to close scheduler lock provider", "error", closeErr)
					}
				}()

				runtime, err := scheduler.NewRuntime(jobsRuntime, lockProvider, log, scheduler.Config{
					DispatchTimeout: dispatchTO,
					DefaultLockTTL:  defaultLockTTL,
				})
				if err != nil {
					return fmt.Errorf("create scheduler runtime: %w", err)
				}
				if err := opts.ConfigureScheduler(cfg, log, runtime); err != nil {
					return fmt.Errorf("configure scheduler runtime: %w", err)
				}

				runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
				defer stop()
				return runtime.Start(runCtx)
			},
		}
		SetCommandPolicies(runCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyScheduled})
		runCmd.Flags().DurationVar(&dispatchTO, "dispatch-timeout", scheduler.DefaultDispatchTimeout, "max duration for single scheduled dispatch")
		runCmd.Flags().DurationVar(&defaultLockTTL, "default-lock-ttl", scheduler.DefaultLockTTL, "default distributed lock TTL")
		runCmd.Flags().StringVar(&redisURL, "redis-url", "", "redis URL for scheduler locks (fallback: cache.url)")
		runCmd.Flags().StringVar(&redisPrefix, "redis-prefix", "nimburion:scheduler:lock", "redis key prefix for scheduler locks")
		runCmd.Flags().DurationVar(&redisOperationTO, "redis-operation-timeout", 3*time.Second, "redis operation timeout for lock provider")
		schedulerCmd.AddCommand(runCmd)
		rootCmd.AddCommand(schedulerCmd)
	}

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
