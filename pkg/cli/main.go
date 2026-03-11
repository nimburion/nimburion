package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/nimburion/nimburion/pkg/audit"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/coordination"
	coordinationpostgres "github.com/nimburion/nimburion/pkg/coordination/postgres"
	coordinationredis "github.com/nimburion/nimburion/pkg/coordination/redis"
	coreapp "github.com/nimburion/nimburion/pkg/core/app"
	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	"github.com/nimburion/nimburion/pkg/descriptor"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/jobs"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/scheduler"
	schedulerconfig "github.com/nimburion/nimburion/pkg/scheduler/config"
	"github.com/nimburion/nimburion/pkg/version"
)

const (
	policiesAnnotationPrefix = "policies."
	defaultPolicyContext     = "run"
)

// CommandPolicy defines the supported command policy values.
// CommandPolicy defines when a CLI command should be executed.
type CommandPolicy string

// Command execution policy constants
const (
	// PolicyAlways executes the command every time
	PolicyAlways CommandPolicy = "always"
	// PolicyMigration executes during migration phase
	PolicyMigration CommandPolicy = "migration"
	// PolicyRun executes during normal runtime
	PolicyRun CommandPolicy = "run"
	// PolicyManual requires manual execution
	PolicyManual CommandPolicy = "manual"
	// PolicyScheduled executes on a schedule
	PolicyScheduled CommandPolicy = "scheduled"
)

// AppCommandOptions defines callbacks for application-oriented CLI assembly.
type AppCommandOptions struct {
	Name        string
	Description string
	ConfigPath  string
	Descriptor  descriptor.Options

	ConfigPathResolved func(string)
	EnvPrefix          string
	ConfigExtensions   []any
	Features           []corefeature.Feature
	Debug              bool
	HealthRegistry     *health.Registry

	Run func(ctx context.Context, cfg *config.Config, log logger.Logger) error

	CheckDependencies func(ctx context.Context, cfg *config.Config, log logger.Logger) error
	ValidateConfig    func(cfg *config.Config) error
	CustomCommands    []*cobra.Command
}

// NewAppCommand creates the target-state CLI with `run` as the primary entrypoint.
func NewAppCommand(opts AppCommandOptions) *cobra.Command {
	featureConfigExtensions, featureCommands := collectFeatureCLIContributions(opts.Features)
	opts.ConfigExtensions = append(append([]any(nil), opts.ConfigExtensions...), featureConfigExtensions...)
	opts.CustomCommands = append(append([]*cobra.Command(nil), opts.CustomCommands...), featureCommands...)

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
	var appNameOverride string
	var debug bool
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config-file", "c", opts.ConfigPath, "config file path")
	rootCmd.PersistentFlags().StringVar(&secretFilePath, "secret-file", "", "path to secrets file (sets APP_SECRETS_FILE)")
	rootCmd.PersistentFlags().StringVar(&appNameOverride, "app-name", "", "application name override")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", opts.Debug, "enable framework debug surfaces such as introspection")

	loadConfig := func(cmd *cobra.Command) (*config.Config, logger.Logger, error) {
		if opts.ConfigPathResolved != nil {
			opts.ConfigPathResolved(cfgPath)
		}
		return LoadConfigAndLogger(
			cfgPath,
			opts.EnvPrefix,
			secretFilePath,
			opts.ValidateConfig,
			cmd,
			opts.ConfigExtensions,
			opts.Name,
			appNameOverride,
		)
	}

	buildJobsRuntime := func(cfg *config.Config, log logger.Logger) (jobs.Runtime, error) {
		return jobs.NewRuntimeFromConfigWithValidation(cfg.Jobs, cfg.EventBus, cfg.Validation.Kafka, log)
	}

	buildSchedulerLockProvider := func(cfg *config.Config, log logger.Logger) (coordination.LockProvider, error) {
		return defaultSchedulerLockProviderFactory(cfg, log)
	}

	for _, ext := range opts.ConfigExtensions {
		if err := config.RegisterFlagsFromStruct(rootCmd.PersistentFlags(), ext); err != nil {
			if _, writeErr := fmt.Fprintf(os.Stderr, "failed to register config flags: %v\n", err); writeErr != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
	}

	// version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			info := version.Current(opts.Name)
			fmt.Printf("Service:    %s\n", info.Service)
			fmt.Printf("Version:    %s\n", info.Version)
			fmt.Printf("Commit:     %s\n", info.Commit)
			fmt.Printf("Build Time: %s\n", info.BuildTime)
		},
	})
	SetCommandPolicies(rootCmd.Commands()[len(rootCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	// run command (primary)
	if opts.Run != nil {
		runCmd := &cobra.Command{
			Use:   "run",
			Short: "Run the application",
			RunE: func(cmd *cobra.Command, _ []string) error {
				cfg, log, err := loadConfig(cmd)
				if err != nil {
					return err
				}
				return opts.Run(cmd.Context(), cfg, log)
			},
		}
		SetCommandPolicies(runCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyRun})
		rootCmd.AddCommand(runCmd)
		rootCmd.RunE = runCmd.RunE
	}

	// healthcheck command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "healthcheck",
		Short: "Check connectivity to framework and service dependencies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, log, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			return runHealthcheckWithRegistry(cmd.Context(), healthcheckRuntimeOptions{
				name:                       opts.Name,
				cfg:                        cfg,
				log:                        log,
				features:                   opts.Features,
				debug:                      debug,
				registry:                   opts.HealthRegistry,
				checkDependencies:          opts.CheckDependencies,
				buildJobsRuntime:           buildJobsRuntime,
				buildSchedulerLockProvider: buildSchedulerLockProvider,
			})
		},
	})
	SetCommandPolicies(rootCmd.Commands()[len(rootCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "introspect",
		Short: "Show framework introspection data when debug is enabled",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !debug {
				return errors.New("framework introspection is disabled; rerun with --debug")
			}
			cfg, log, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			app, err := coreapp.New(coreapp.Options{
				Name:     opts.Name,
				Config:   cfg,
				Logger:   log,
				Debug:    debug,
				Features: opts.Features,
			})
			if err != nil {
				return err
			}
			if prepareErr := app.Prepare(cmd.Context()); prepareErr != nil {
				return prepareErr
			}

			out, err := formatSettings(app.Runtime().Introspection.Snapshot())
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	})
	SetCommandPolicies(rootCmd.Commands()[len(rootCmd.Commands())-1], map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	var descriptorFormat string
	describeCmd := &cobra.Command{
		Use:   "describe",
		Short: "Emit the machine-readable service descriptor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			descOpts := opts.Descriptor
			if descOpts.Application.Name == "" {
				descOpts.Application.Name = opts.Name
			}
			descOpts.EnvPrefix = opts.EnvPrefix
			desc, err := descriptor.Generate(rootCmd, descOpts)
			if err != nil {
				return err
			}
			raw, err := descriptor.Marshal(desc, descriptorFormat)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(raw)); err != nil {
				return err
			}
			return nil
		},
	}
	describeCmd.Flags().StringVar(&descriptorFormat, "format", "json", "descriptor output format: json or yaml")
	SetCommandPolicies(describeCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})
	rootCmd.AddCommand(describeCmd)

	// config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}
	SetCommandPolicies(configCmd, map[string]CommandPolicy{defaultPolicyContext: PolicyAlways})

	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := applySecretFileFlag(opts.EnvPrefix, secretFilePath); err != nil {
				return err
			}
			if opts.ConfigPathResolved != nil {
				opts.ConfigPathResolved(cfgPath)
			}
			cfg := &config.Config{}
			req := deriveValidationRequirements(cmd)
			provider := config.NewConfigProvider(cfgPath, opts.EnvPrefix).
				WithAppNameDefault(opts.Name).
				WithValidationRequirements(req).
				WithFlags(cmd.Flags())
			if _, err := provider.LoadWithSecrets(cfg, opts.ConfigExtensions...); err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			applyResolvedAppName(cfg, opts.Name, appNameOverride)
			if opts.ValidateConfig != nil {
				if err := opts.ValidateConfig(cfg); err != nil {
					return fmt.Errorf("custom validation failed: %w", err)
				}
			}
			if err := config.NewViperLoader("", opts.EnvPrefix).WithValidationRequirements(req).Validate(cfg); err != nil {
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := applySecretFileFlag(opts.EnvPrefix, secretFilePath); err != nil {
				return err
			}
			if opts.ConfigPathResolved != nil {
				opts.ConfigPathResolved(cfgPath)
			}
			cfg := &config.Config{}
			provider := config.NewConfigProvider(cfgPath, opts.EnvPrefix).
				WithAppNameDefault(opts.Name).
				WithFlags(cmd.Flags())
			secrets, err := provider.LoadWithSecrets(cfg, opts.ConfigExtensions...)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			applyResolvedAppName(cfg, opts.Name, appNameOverride)

			settings := provider.AllSettings()
			settings = setAppNameSetting(settings, cfg.App.Name)
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

func collectFeatureCLIContributions(features []corefeature.Feature) ([]any, []*cobra.Command) {
	var (
		configExtensions []any
		commands         []*cobra.Command
	)

	for _, feature := range features {
		if feature == nil {
			continue
		}
		contributions := feature.Contributions()
		for _, extension := range contributions.ConfigExtensions {
			if extension.Extension != nil {
				configExtensions = append(configExtensions, extension.Extension)
			}
		}
		for _, command := range contributions.CommandRegistrations {
			cobraCmd, ok := command.Command.(*cobra.Command)
			if !ok || cobraCmd == nil {
				continue
			}
			commands = append(commands, cobraCmd)
		}
	}

	return configExtensions, commands
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

type healthcheckRuntimeOptions struct {
	name                       string
	cfg                        *config.Config
	log                        logger.Logger
	features                   []corefeature.Feature
	debug                      bool
	registry                   *health.Registry
	checkDependencies          func(context.Context, *config.Config, logger.Logger) error
	buildJobsRuntime           func(*config.Config, logger.Logger) (jobs.Runtime, error)
	buildSchedulerLockProvider func(*config.Config, logger.Logger) (coordination.LockProvider, error)
}

func runHealthcheckWithRegistry(ctx context.Context, opts healthcheckRuntimeOptions) error {
	registry := opts.registry
	if registry == nil {
		registry = health.NewRegistry()
	}

	app, err := coreapp.New(coreapp.Options{
		Name:           opts.name,
		Config:         opts.cfg,
		Logger:         opts.log,
		Debug:          opts.debug,
		Features:       opts.features,
		HealthRegistry: registry,
		HealthRegistrations: []coreapp.Hook{{
			Name: "framework_dependencies",
			Fn: func(_ context.Context, runtime *coreapp.Runtime) error {
				registerFrameworkDependencyHealthChecks(runtime.HealthRegistry(), opts.cfg, opts.log, opts.buildJobsRuntime, opts.buildSchedulerLockProvider)
				if opts.checkDependencies != nil {
					runtime.HealthRegistry().RegisterFunc("application-dependencies", func(ctx context.Context) health.CheckResult {
						if err := opts.checkDependencies(ctx, opts.cfg, opts.log); err != nil {
							return health.CheckResult{
								Name:    "application-dependencies",
								Status:  health.StatusUnhealthy,
								Error:   err.Error(),
								Message: "application dependency checks failed",
							}
						}
						return health.CheckResult{
							Name:    "application-dependencies",
							Status:  health.StatusHealthy,
							Message: "application dependency checks passed",
						}
					})
				}
				return nil
			},
		}},
	})
	if err != nil {
		return err
	}
	if err := app.Prepare(ctx); err != nil {
		return err
	}

	result := registry.Check(ctx)
	if result.IsReady() {
		return nil
	}

	var errs []error
	for _, check := range result.Checks {
		if err := healthCheckResultError(check); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func registerFrameworkDependencyHealthChecks(
	registry *health.Registry,
	cfg *config.Config,
	log logger.Logger,
	buildJobsRuntime func(*config.Config, logger.Logger) (jobs.Runtime, error),
	buildSchedulerLockProvider func(*config.Config, logger.Logger) (coordination.LockProvider, error),
) {
	if registry == nil || cfg == nil {
		return
	}

	if shouldCheckJobsRuntimeHealth(cfg) {
		registry.RegisterFunc("jobs-runtime", func(ctx context.Context) health.CheckResult {
			runtime, err := buildJobsRuntime(cfg, log)
			if err != nil {
				return health.CheckResult{
					Name:   "jobs-runtime",
					Status: health.StatusUnhealthy,
					Error:  fmt.Sprintf("create jobs runtime for healthcheck: %v", coreerrors.Canonicalize(err)),
				}
			}
			defer func() {
				if closeErr := runtime.Close(); closeErr != nil {
					log.Error("failed to close jobs runtime after healthcheck", "error", closeErr)
				}
			}()
			return jobs.NewRuntimeHealthChecker("", runtime, jobsHealthTimeout(cfg)).Check(ctx)
		})
	}

	if shouldCheckSchedulerLockHealth(cfg) {
		registry.RegisterFunc("scheduler-lock-provider", func(ctx context.Context) health.CheckResult {
			lockProvider, err := buildSchedulerLockProvider(cfg, log)
			if err != nil {
				return health.CheckResult{
					Name:   "scheduler-lock-provider",
					Status: health.StatusUnhealthy,
					Error:  fmt.Sprintf("create scheduler lock provider for healthcheck: %v", coreerrors.Canonicalize(err)),
				}
			}
			defer func() {
				if closeErr := lockProvider.Close(); closeErr != nil {
					log.Error("failed to close scheduler lock provider after healthcheck", "error", closeErr)
				}
			}()
			return scheduler.NewLockProviderHealthChecker("", lockProvider, schedulerLockHealthTimeout(cfg)).Check(ctx)
		})
	}
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
	case jobsconfig.BackendRedis:
		return strings.TrimSpace(cfg.Jobs.Redis.URL) != ""
	case jobsconfig.BackendEventBus:
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
	if strings.EqualFold(strings.TrimSpace(cfg.Jobs.Backend), jobsconfig.BackendRedis) && cfg.Jobs.Redis.OperationTimeout > 0 {
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
	case schedulerconfig.LockProviderPostgres:
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
	if result.Status != health.StatusUnhealthy {
		return nil
	}
	msg := strings.TrimSpace(result.Error)
	if msg == "" {
		msg = strings.TrimSpace(result.Message)
	}
	if msg == "" {
		msg = "health check failed"
	}
	return coreerrors.NewUnavailable(fmt.Sprintf("%s: %s", result.Name, msg), nil).
		WithDetails(map[string]interface{}{"check": result.Name})
}

func defaultSchedulerLockProviderFactory(cfg *config.Config, log logger.Logger) (coordination.LockProvider, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	lockProvider := strings.ToLower(strings.TrimSpace(cfg.Scheduler.LockProvider))
	if lockProvider == "" {
		lockProvider = schedulerconfig.LockProviderRedis
	}

	switch lockProvider {
	case schedulerconfig.LockProviderRedis:
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
	case schedulerconfig.LockProviderPostgres:
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

// LoadConfigAndLogger loads configuration and initializes logger with optional custom validation
func LoadConfigAndLogger(
	cfgPath,
	envPrefix,
	secretFilePath string,
	customValidator func(*config.Config) error,
	cmd *cobra.Command,
	extensions []any,
	defaultAppName string,
	appNameOverride string,
) (*config.Config, logger.Logger, error) {
	if envPrefix == "" {
		envPrefix = "APP"
	}
	if err := applySecretFileFlag(envPrefix, secretFilePath); err != nil {
		return nil, nil, err
	}
	var flags *pflag.FlagSet
	if cmd != nil {
		flags = cmd.Flags()
	}
	req := deriveValidationRequirements(cmd)
	cfg := &config.Config{}
	provider := config.NewConfigProvider(cfgPath, envPrefix).
		WithAppNameDefault(defaultAppName).
		WithValidationRequirements(req).
		WithFlags(flags)
	if _, err := provider.LoadWithSecrets(cfg, extensions...); err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	applyResolvedAppName(cfg, defaultAppName, appNameOverride)

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

func deriveValidationRequirements(cmd *cobra.Command) config.ValidationRequirements {
	if cmd == nil {
		return config.ValidationRequirements{}
	}

	path := strings.Fields(strings.TrimSpace(cmd.CommandPath()))
	if len(path) >= 2 {
		path = path[1:]
	}
	joined := strings.Join(path, " ")

	switch joined {
	case "jobs worker", "jobs enqueue", "jobs dlq list", "jobs dlq replay", "scheduler run", "scheduler trigger":
		return config.ValidationRequirements{RequireJobsEventBus: true}
	}
	return config.ValidationRequirements{}
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
	return audit.RedactSettings(settings, secrets)
}

func shouldRedactSetting(mask interface{}) bool {
	return audit.ShouldRedact(mask)
}

// Execute runs the command and exits with appropriate code.
func Execute(cmd *cobra.Command) {
	if err := cmd.Execute(); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
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

func applyResolvedAppName(cfg *config.Config, defaultAppName, appNameOverride string) {
	if cfg == nil {
		return
	}
	cfg.App.Name = resolveAppNameValue(cfg.App.Name, defaultAppName, appNameOverride)
}

func resolveAppNameValue(currentConfigName, defaultAppName, appNameOverride string) string {
	if override := strings.TrimSpace(appNameOverride); override != "" {
		return override
	}
	if configured := strings.TrimSpace(currentConfigName); configured != "" {
		return configured
	}
	if fallback := strings.TrimSpace(defaultAppName); fallback != "" {
		return fallback
	}
	return "app"
}

func setAppNameSetting(settings map[string]interface{}, appName string) map[string]interface{} {
	if settings == nil {
		settings = map[string]interface{}{}
	}
	app, ok := settings["app"].(map[string]interface{})
	if !ok || app == nil {
		app = map[string]interface{}{}
	}
	app["name"] = appName
	settings["app"] = app
	return settings
}
