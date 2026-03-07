package descriptor

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nimburion/nimburion/pkg/audit"
	"github.com/nimburion/nimburion/pkg/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	// VersionV1 is the current service descriptor version.
	VersionV1 = "v1"

	defaultNimbctlSupportedRange = ">=1.0.0 <2.0.0"
	policiesAnnotationPrefix     = "policies."
)

type ApplicationKind string
type TenancyMode string
type CompatibilityPolicy string
type CommandKind string
type RunPolicy string
type InputKind string
type DependencyReadiness string
type DependencyDurability string
type EndpointKind string
type TransportFamily string
type SecurityMode string
type DeploymentMode string
type HorizontalScaling string
type MigrationStrategy string
type FeatureStability string
type FeatureCriticality string

const (
	ApplicationKindGateway   ApplicationKind = "gateway"
	ApplicationKindService   ApplicationKind = "service"
	ApplicationKindConsumer  ApplicationKind = "consumer"
	ApplicationKindProducer  ApplicationKind = "producer"
	ApplicationKindWorker    ApplicationKind = "worker"
	ApplicationKindScheduler ApplicationKind = "scheduler"

	TenancyModeSingleTenant   TenancyMode = "single_tenant"
	TenancyModeMultiTenant    TenancyMode = "multi_tenant"
	TenancyModeTenantOptional TenancyMode = "tenant_optional"

	CompatibilityPolicyStrict     CompatibilityPolicy = "strict"
	CompatibilityPolicyBestEffort CompatibilityPolicy = "best_effort"

	CommandKindRuntime     CommandKind = "runtime"
	CommandKindTransport   CommandKind = "transport"
	CommandKindConfig      CommandKind = "config"
	CommandKindMigration   CommandKind = "migration"
	CommandKindJobs        CommandKind = "jobs"
	CommandKindScheduler   CommandKind = "scheduler"
	CommandKindMaintenance CommandKind = "maintenance"
	CommandKindDebug       CommandKind = "debug"

	RunPolicyAlways    RunPolicy = "always"
	RunPolicyRun       RunPolicy = "run"
	RunPolicyMigration RunPolicy = "migration"
	RunPolicyScheduled RunPolicy = "scheduled"
	RunPolicyManual    RunPolicy = "manual"
	RunPolicyOnDemand  RunPolicy = "on_demand"

	InputKindConfigFile     InputKind = "config_file"
	InputKindSecretsFile    InputKind = "secrets_file"
	InputKindEnv            InputKind = "env"
	InputKindFlag           InputKind = "flag"
	InputKindSecretProvider InputKind = "secret_provider"

	DependencyReadinessHard     DependencyReadiness = "hard"
	DependencyReadinessOptional DependencyReadiness = "optional"

	DependencyDurabilityDurable   DependencyDurability = "durable"
	DependencyDurabilityEphemeral DependencyDurability = "ephemeral"

	EndpointKindHealth        EndpointKind = "health"
	EndpointKindReadiness     EndpointKind = "readiness"
	EndpointKindLiveness      EndpointKind = "liveness"
	EndpointKindMetrics       EndpointKind = "metrics"
	EndpointKindIntrospection EndpointKind = "introspection"

	TransportFamilyHTTP TransportFamily = "http"
	TransportFamilyGRPC TransportFamily = "grpc"

	SecurityModeInsecure            SecurityMode = "insecure"
	SecurityModeTLS                 SecurityMode = "tls"
	SecurityModeMTLS                SecurityMode = "mtls"
	SecurityModeExternalTermination SecurityMode = "external_termination"

	DeploymentModeStateless DeploymentMode = "stateless"
	DeploymentModeStateful  DeploymentMode = "stateful"

	HorizontalScalingSafe               HorizontalScaling = "safe"
	HorizontalScalingLeaderLockRequired HorizontalScaling = "leader_lock_required"
	HorizontalScalingSingleInstance     HorizontalScaling = "single_instance"

	MigrationStrategyManual         MigrationStrategy = "manual"
	MigrationStrategyPredeploy      MigrationStrategy = "predeploy"
	MigrationStrategyPostdeploy     MigrationStrategy = "postdeploy"
	MigrationStrategyOnline         MigrationStrategy = "online"
	MigrationStrategyOffline        MigrationStrategy = "offline"
	MigrationStrategyExpandContract MigrationStrategy = "expand_contract"

	FeatureStabilityExperimental FeatureStability = "experimental"
	FeatureStabilityBeta         FeatureStability = "beta"
	FeatureStabilityStable       FeatureStability = "stable"
	FeatureStabilityDeprecated   FeatureStability = "deprecated"

	FeatureCriticalityCore     FeatureCriticality = "core"
	FeatureCriticalityOptional FeatureCriticality = "optional"
)

type Descriptor struct {
	DescriptorVersion string         `json:"descriptor_version" yaml:"descriptor_version"`
	Application       Application    `json:"application" yaml:"application"`
	Compatibility     Compatibility  `json:"compatibility" yaml:"compatibility"`
	Runtime           Runtime        `json:"runtime" yaml:"runtime"`
	Commands          []Command      `json:"commands" yaml:"commands"`
	Config            ConfigContract `json:"config" yaml:"config"`
	Dependencies      []Dependency   `json:"dependencies" yaml:"dependencies"`
	Management        Management     `json:"management" yaml:"management"`
	Transports        []Transport    `json:"transports" yaml:"transports"`
	Deployment        Deployment     `json:"deployment" yaml:"deployment"`
	Migrations        Migrations     `json:"migrations" yaml:"migrations"`
	Features          []Feature      `json:"features" yaml:"features"`
	Artifacts         Artifacts      `json:"artifacts" yaml:"artifacts"`
}

type Application struct {
	Name        string          `json:"name" yaml:"name"`
	Kind        ApplicationKind `json:"kind" yaml:"kind"`
	Module      string          `json:"module,omitempty" yaml:"module,omitempty"`
	Version     string          `json:"version,omitempty" yaml:"version,omitempty"`
	TenancyMode TenancyMode     `json:"tenancy_mode,omitempty" yaml:"tenancy_mode,omitempty"`
}

type Compatibility struct {
	Framework CompatibilityFramework `json:"framework" yaml:"framework"`
	Nimbctl   CompatibilityNimbctl   `json:"nimbctl" yaml:"nimbctl"`
	Policy    CompatibilityPolicy    `json:"policy,omitempty" yaml:"policy,omitempty"`
}

type CompatibilityFramework struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

type CompatibilityNimbctl struct {
	SupportedRange string `json:"supported_range" yaml:"supported_range"`
	TestedRange    string `json:"tested_range,omitempty" yaml:"tested_range,omitempty"`
}

type Runtime struct {
	DefaultCommand           string `json:"default_command" yaml:"default_command"`
	ConfigFileFlag           string `json:"config_file_flag,omitempty" yaml:"config_file_flag,omitempty"`
	SecretsFileFlag          string `json:"secrets_file_flag,omitempty" yaml:"secrets_file_flag,omitempty"`
	SupportsGracefulShutdown bool   `json:"supports_graceful_shutdown,omitempty" yaml:"supports_graceful_shutdown,omitempty"`
}

type Command struct {
	Name      string      `json:"name" yaml:"name"`
	Path      []string    `json:"path" yaml:"path"`
	Kind      CommandKind `json:"kind" yaml:"kind"`
	RunPolicy RunPolicy   `json:"run_policy" yaml:"run_policy"`
	Default   bool        `json:"default,omitempty" yaml:"default,omitempty"`
	Transport string      `json:"transport,omitempty" yaml:"transport,omitempty"`
}

type ConfigContract struct {
	Render           RenderContract   `json:"render" yaml:"render"`
	Validate         ValidateContract `json:"validate" yaml:"validate"`
	Inputs           ConfigInputs     `json:"inputs" yaml:"inputs"`
	Profiles         ConfigProfiles   `json:"profiles" yaml:"profiles"`
	SensitivityModel SensitivityModel `json:"sensitivity_model" yaml:"sensitivity_model"`
}

type RenderContract struct {
	Supported bool         `json:"supported" yaml:"supported"`
	Path      []string     `json:"path,omitempty" yaml:"path,omitempty"`
	Formats   []string     `json:"formats,omitempty" yaml:"formats,omitempty"`
	Profiles  []string     `json:"profiles,omitempty" yaml:"profiles,omitempty"`
	Outputs   []OutputSpec `json:"outputs,omitempty" yaml:"outputs,omitempty"`
}

type ValidateContract struct {
	Supported bool     `json:"supported" yaml:"supported"`
	Path      []string `json:"path,omitempty" yaml:"path,omitempty"`
}

type ConfigInputs struct {
	Regular   []InputSpec `json:"regular" yaml:"regular"`
	Sensitive []InputSpec `json:"sensitive" yaml:"sensitive"`
}

type InputSpec struct {
	Kind     InputKind `json:"kind" yaml:"kind"`
	Flag     string    `json:"flag,omitempty" yaml:"flag,omitempty"`
	Format   string    `json:"format,omitempty" yaml:"format,omitempty"`
	Required bool      `json:"required,omitempty" yaml:"required,omitempty"`
	Prefix   string    `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Provider string    `json:"provider,omitempty" yaml:"provider,omitempty"`
}

type OutputSpec struct {
	Name     string `json:"name" yaml:"name"`
	Kind     string `json:"kind" yaml:"kind"`
	Format   string `json:"format" yaml:"format"`
	Required bool   `json:"required,omitempty" yaml:"required,omitempty"`
}

type SensitivityModel struct {
	ClassificationEnum []string `json:"classification_enum,omitempty" yaml:"classification_enum,omitempty"`
	RedactionEnum      []string `json:"redaction_enum,omitempty" yaml:"redaction_enum,omitempty"`
	ProvenanceVisible  bool     `json:"provenance_visible,omitempty" yaml:"provenance_visible,omitempty"`
}

type ConfigProfiles struct {
	Environments []Profile `json:"environments,omitempty" yaml:"environments,omitempty"`
}

type Profile struct {
	Name        string `json:"name" yaml:"name"`
	Default     bool   `json:"default,omitempty" yaml:"default,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Dependency struct {
	Name       string               `json:"name" yaml:"name"`
	Family     string               `json:"family" yaml:"family"`
	Role       string               `json:"role" yaml:"role"`
	Required   bool                 `json:"required" yaml:"required"`
	Readiness  DependencyReadiness  `json:"readiness" yaml:"readiness"`
	Durability DependencyDurability `json:"durability,omitempty" yaml:"durability,omitempty"`
}

type Management struct {
	Supported bool       `json:"supported" yaml:"supported"`
	Transport string     `json:"transport,omitempty" yaml:"transport,omitempty"`
	Endpoints []Endpoint `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
}

type Endpoint struct {
	Kind          EndpointKind `json:"kind" yaml:"kind"`
	Transport     string       `json:"transport,omitempty" yaml:"transport,omitempty"`
	Target        string       `json:"target" yaml:"target"`
	Authenticated bool         `json:"authenticated,omitempty" yaml:"authenticated,omitempty"`
	Reports       []string     `json:"reports,omitempty" yaml:"reports,omitempty"`
}

type Transport struct {
	Family        TransportFamily    `json:"family" yaml:"family"`
	Services      []Service          `json:"services,omitempty" yaml:"services,omitempty"`
	Reflection    *Capability        `json:"reflection,omitempty" yaml:"reflection,omitempty"`
	HealthService *Capability        `json:"health_service,omitempty" yaml:"health_service,omitempty"`
	Security      *TransportSecurity `json:"security,omitempty" yaml:"security,omitempty"`
	ProtoPackages []ProtoPackage     `json:"proto_packages,omitempty" yaml:"proto_packages,omitempty"`
}

type Service struct {
	Name    string `json:"name" yaml:"name"`
	Package string `json:"package,omitempty" yaml:"package,omitempty"`
}

type Capability struct {
	Supported bool `json:"supported" yaml:"supported"`
}

type TransportSecurity struct {
	Mode SecurityMode `json:"mode" yaml:"mode"`
}

type ProtoPackage struct {
	Name      string `json:"name" yaml:"name"`
	Ownership string `json:"ownership,omitempty" yaml:"ownership,omitempty"`
}

type Deployment struct {
	Mode              DeploymentMode    `json:"mode,omitempty" yaml:"mode,omitempty"`
	HorizontalScaling HorizontalScaling `json:"horizontal_scaling,omitempty" yaml:"horizontal_scaling,omitempty"`
	Capabilities      []string          `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

type Migrations struct {
	Supported          bool              `json:"supported" yaml:"supported"`
	Command            []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Strategy           MigrationStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	BlocksRuntimeStart bool              `json:"blocks_runtime_start,omitempty" yaml:"blocks_runtime_start,omitempty"`
	MixedVersionSafe   bool              `json:"mixed_version_safe,omitempty" yaml:"mixed_version_safe,omitempty"`
	RequiresLock       bool              `json:"requires_lock,omitempty" yaml:"requires_lock,omitempty"`
}

type Feature struct {
	Name        string             `json:"name" yaml:"name"`
	Stability   FeatureStability   `json:"stability" yaml:"stability"`
	Criticality FeatureCriticality `json:"criticality,omitempty" yaml:"criticality,omitempty"`
}

type Artifacts struct {
	ConfigSchema         *Artifact `json:"config_schema,omitempty" yaml:"config_schema,omitempty"`
	ProtoBundle          *Artifact `json:"proto_bundle,omitempty" yaml:"proto_bundle,omitempty"`
	GRPCDescriptorSet    *Artifact `json:"grpc_descriptor_set,omitempty" yaml:"grpc_descriptor_set,omitempty"`
	GRPCBufImage         *Artifact `json:"grpc_buf_image,omitempty" yaml:"grpc_buf_image,omitempty"`
	GRPCContractManifest *Artifact `json:"grpc_contract_manifest,omitempty" yaml:"grpc_contract_manifest,omitempty"`
}

type Artifact struct {
	Format   string `json:"format" yaml:"format"`
	Location string `json:"location" yaml:"location"`
	Version  string `json:"version,omitempty" yaml:"version,omitempty"`
}

type Options struct {
	Application Application

	FrameworkVersion      string
	NimbctlSupportedRange string
	NimbctlTestedRange    string
	CompatibilityPolicy   CompatibilityPolicy

	EnvPrefix string

	Dependencies []Dependency
	Management   *Management
	Transports   []Transport
	Deployment   *Deployment
	Migrations   *Migrations
	Features     []Feature
	Artifacts    Artifacts
}

// Generate builds a v1 descriptor from the application command tree and explicit metadata.
func Generate(root *cobra.Command, opts Options) (Descriptor, error) {
	if root == nil {
		return Descriptor{}, errors.New("root command is required")
	}

	application := opts.Application
	if strings.TrimSpace(application.Name) == "" {
		application.Name = strings.TrimSpace(root.Name())
	}
	if application.Kind == "" {
		application.Kind = ApplicationKindService
	}
	if strings.TrimSpace(application.Version) == "" {
		application.Version = version.Current(application.Name).Version
	}

	commands := commandInventory(root, opts.Transports)
	defaultCommand := resolveDefaultCommand(commands)
	if defaultCommand == "" {
		return Descriptor{}, errors.New("default runtime command is required")
	}
	for i := range commands {
		if commands[i].Name == defaultCommand {
			commands[i].Default = true
		}
	}

	migrations := normalizeMigrations(opts.Migrations, commands)
	features := normalizeFeatures(opts.Features, opts.Transports, migrations)
	transports := normalizeTransports(opts.Transports)

	out := Descriptor{
		DescriptorVersion: VersionV1,
		Application:       application,
		Compatibility: Compatibility{
			Framework: CompatibilityFramework{
				Name:    "nimburion",
				Version: defaultString(opts.FrameworkVersion, version.Current(application.Name).Version),
			},
			Nimbctl: CompatibilityNimbctl{
				SupportedRange: defaultString(opts.NimbctlSupportedRange, defaultNimbctlSupportedRange),
				TestedRange:    opts.NimbctlTestedRange,
			},
			Policy: defaultCompatibilityPolicy(opts.CompatibilityPolicy),
		},
		Runtime: Runtime{
			DefaultCommand:           defaultCommand,
			ConfigFileFlag:           "--config-file",
			SecretsFileFlag:          "--secret-file",
			SupportsGracefulShutdown: true,
		},
		Commands: commands,
		Config: ConfigContract{
			Render: RenderContract{
				Supported: commandExists(commands, []string{"config", "show"}),
				Path:      commandPath(commands, []string{"config", "show"}),
				Formats:   []string{"yaml"},
				Outputs: []OutputSpec{
					{Name: "config", Kind: "config", Format: "yaml", Required: true},
				},
			},
			Validate: ValidateContract{
				Supported: commandExists(commands, []string{"config", "validate"}),
				Path:      commandPath(commands, []string{"config", "validate"}),
			},
			Inputs: ConfigInputs{
				Regular: []InputSpec{
					{Kind: InputKindConfigFile, Flag: "--config-file", Format: "yaml"},
					{Kind: InputKindEnv, Prefix: strings.TrimSpace(opts.EnvPrefix)},
				},
				Sensitive: []InputSpec{
					{Kind: InputKindSecretsFile, Flag: "--secret-file", Format: "yaml"},
				},
			},
			SensitivityModel: SensitivityModel{
				ClassificationEnum: []string{
					string(audit.ClassificationPublic),
					string(audit.ClassificationSensitive),
					string(audit.ClassificationSecret),
				},
				RedactionEnum: []string{
					string(audit.RedactionNone),
					string(audit.RedactionMask),
					string(audit.RedactionFull),
				},
			},
		},
		Dependencies: opts.Dependencies,
		Management:   normalizeManagement(opts.Management),
		Transports:   transports,
		Deployment:   normalizeDeployment(opts.Deployment),
		Migrations:   migrations,
		Features:     features,
		Artifacts:    opts.Artifacts,
	}

	return out, nil
}

// Marshal renders a descriptor in one supported format.
func Marshal(desc Descriptor, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "json":
		return json.MarshalIndent(desc, "", "  ")
	case "yaml", "yml":
		return yaml.Marshal(desc)
	default:
		return nil, fmt.Errorf("unsupported descriptor format %q", format)
	}
}

func commandInventory(root *cobra.Command, transports []Transport) []Command {
	var commands []Command
	var visit func(*cobra.Command, []string)
	visit = func(cmd *cobra.Command, parent []string) {
		if cmd == nil || cmd.Hidden {
			return
		}
		name := strings.TrimSpace(cmd.Name())
		if name == "" || name == "help" || name == "completion" {
			for _, sub := range cmd.Commands() {
				visit(sub, parent)
			}
			return
		}

		path := append(append([]string(nil), parent...), name)
		if runnable(cmd) {
			commands = append(commands, Command{
				Name:      strings.Join(path, "-"),
				Path:      append([]string(nil), path...),
				Kind:      inferCommandKind(path),
				RunPolicy: inferRunPolicy(cmd),
				Transport: inferCommandTransport(path, transports),
			})
		}
		for _, sub := range cmd.Commands() {
			visit(sub, path)
		}
	}

	for _, cmd := range root.Commands() {
		visit(cmd, nil)
	}
	sort.Slice(commands, func(i, j int) bool {
		return strings.Join(commands[i].Path, " ") < strings.Join(commands[j].Path, " ")
	})
	return commands
}

func runnable(cmd *cobra.Command) bool {
	return cmd.Run != nil || cmd.RunE != nil
}

func inferCommandKind(path []string) CommandKind {
	if len(path) == 0 {
		return CommandKindMaintenance
	}
	switch path[0] {
	case "run":
		return CommandKindRuntime
	case "config":
		return CommandKindConfig
	case "migrate":
		return CommandKindMigration
	case "jobs":
		return CommandKindJobs
	case "scheduler":
		return CommandKindScheduler
	case "healthcheck", "introspect", "describe":
		return CommandKindDebug
	case "openapi":
		return CommandKindTransport
	default:
		return CommandKindMaintenance
	}
}

func inferRunPolicy(cmd *cobra.Command) RunPolicy {
	if cmd == nil {
		return RunPolicyAlways
	}
	for key, value := range cmd.Annotations {
		if key != policiesAnnotationPrefix+"run" {
			continue
		}
		switch value {
		case string(RunPolicyRun), string(RunPolicyMigration), string(RunPolicyScheduled), string(RunPolicyManual), string(RunPolicyAlways), string(RunPolicyOnDemand):
			return RunPolicy(value)
		}
	}
	return RunPolicyAlways
}

func inferCommandTransport(path []string, transports []Transport) string {
	if len(path) == 0 {
		return ""
	}
	switch path[0] {
	case "openapi":
		return string(TransportFamilyHTTP)
	}
	if len(transports) == 1 {
		return string(transports[0].Family)
	}
	return ""
}

func resolveDefaultCommand(commands []Command) string {
	for _, command := range commands {
		if len(command.Path) == 1 && command.Path[0] == "run" {
			return command.Name
		}
	}
	if len(commands) > 0 {
		return commands[0].Name
	}
	return ""
}

func commandExists(commands []Command, path []string) bool {
	return len(commandPath(commands, path)) > 0
}

func commandPath(commands []Command, path []string) []string {
	target := strings.Join(path, " ")
	for _, command := range commands {
		if strings.Join(command.Path, " ") == target {
			return append([]string(nil), command.Path...)
		}
	}
	return nil
}

func normalizeManagement(in *Management) Management {
	if in == nil {
		return Management{Supported: false}
	}
	out := *in
	sort.Slice(out.Endpoints, func(i, j int) bool {
		return string(out.Endpoints[i].Kind)+out.Endpoints[i].Target < string(out.Endpoints[j].Kind)+out.Endpoints[j].Target
	})
	return out
}

func normalizeTransports(in []Transport) []Transport {
	if len(in) == 0 {
		return nil
	}
	out := append([]Transport(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Family < out[j].Family })
	return out
}

func normalizeDeployment(in *Deployment) Deployment {
	if in == nil {
		return Deployment{}
	}
	out := *in
	sort.Strings(out.Capabilities)
	return out
}

func normalizeMigrations(in *Migrations, commands []Command) Migrations {
	if in != nil {
		return *in
	}
	path := commandPath(commands, []string{"migrate"})
	if len(path) == 0 {
		return Migrations{Supported: false}
	}
	return Migrations{
		Supported: true,
		Command:   path,
		Strategy:  MigrationStrategyManual,
	}
}

func normalizeFeatures(in []Feature, transports []Transport, migrations Migrations) []Feature {
	featureMap := map[string]Feature{}
	for _, feature := range in {
		if strings.TrimSpace(feature.Name) == "" {
			continue
		}
		if feature.Stability == "" {
			feature.Stability = FeatureStabilityStable
		}
		featureMap[feature.Name] = feature
	}
	for _, transport := range transports {
		name := string(transport.Family)
		if _, ok := featureMap[name]; !ok {
			featureMap[name] = Feature{Name: name, Stability: FeatureStabilityStable, Criticality: FeatureCriticalityCore}
		}
		if transport.Reflection != nil && transport.Reflection.Supported {
			featureMap["grpc_reflection"] = Feature{Name: "grpc_reflection", Stability: FeatureStabilityStable, Criticality: FeatureCriticalityOptional}
		}
		if transport.HealthService != nil && transport.HealthService.Supported {
			featureMap["grpc_health"] = Feature{Name: "grpc_health", Stability: FeatureStabilityStable, Criticality: FeatureCriticalityOptional}
		}
	}
	if migrations.Supported {
		if _, ok := featureMap["migrations"]; !ok {
			featureMap["migrations"] = Feature{Name: "migrations", Stability: FeatureStabilityStable, Criticality: FeatureCriticalityCore}
		}
	}
	out := make([]Feature, 0, len(featureMap))
	for _, feature := range featureMap {
		out = append(out, feature)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultCompatibilityPolicy(policy CompatibilityPolicy) CompatibilityPolicy {
	if policy == "" {
		return CompatibilityPolicyStrict
	}
	return policy
}
