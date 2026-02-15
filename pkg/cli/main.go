package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	cliopenapi "github.com/nimburion/nimburion/pkg/cli/openapi"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
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
