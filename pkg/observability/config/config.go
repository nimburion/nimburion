package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config configures logging, tracing, and HTTP request observability behavior.
type Config struct {
	LogLevel          string               `mapstructure:"log_level"`
	LogFormat         string               `mapstructure:"log_format"`
	ServiceName       string               `mapstructure:"service_name"`
	TracingEnabled    bool                 `mapstructure:"tracing_enabled"`
	TracingSampleRate float64              `mapstructure:"tracing_sample_rate"`
	TracingEndpoint   string               `mapstructure:"tracing_endpoint"`
	AsyncLogging      AsyncLoggingConfig   `mapstructure:"async_logging"`
	RequestLogging    RequestLoggingConfig `mapstructure:"request_logging"`
	RequestTracing    RequestTracingConfig `mapstructure:"request_tracing"`
	RequestTimeout    RequestTimeoutConfig `mapstructure:"request_timeout"`
}

type AsyncLoggingConfig struct {
	Enabled      bool `mapstructure:"enabled"`
	QueueSize    int  `mapstructure:"queue_size"`
	WorkerCount  int  `mapstructure:"worker_count"`
	DropWhenFull bool `mapstructure:"drop_when_full"`
}

type RequestLoggingConfig struct {
	Enabled              bool                   `mapstructure:"enabled"`
	LogStart             bool                   `mapstructure:"log_start"`
	Output               string                 `mapstructure:"output"`
	Fields               []string               `mapstructure:"fields"`
	ExcludedPathPrefixes []string               `mapstructure:"excluded_path_prefixes"`
	PathPolicies         []RequestLogPathPolicy `mapstructure:"path_policies"`
}

type RequestLogPathPolicy struct {
	PathPrefix string `mapstructure:"path_prefix"`
	Mode       string `mapstructure:"mode"`
}

type RequestTracingConfig struct {
	Enabled              bool                     `mapstructure:"enabled"`
	ExcludedPathPrefixes []string                 `mapstructure:"excluded_path_prefixes"`
	PathPolicies         []RequestTracePathPolicy `mapstructure:"path_policies"`
}

type RequestTracePathPolicy struct {
	PathPrefix string `mapstructure:"path_prefix"`
	Mode       string `mapstructure:"mode"`
}

type RequestTimeoutConfig struct {
	Enabled              bool                       `mapstructure:"enabled"`
	Default              time.Duration              `mapstructure:"default"`
	ExcludedPathPrefixes []string                   `mapstructure:"excluded_path_prefixes"`
	PathPolicies         []RequestTimeoutPathPolicy `mapstructure:"path_policies"`
}

type RequestTimeoutPathPolicy struct {
	PathPrefix string `mapstructure:"path_prefix"`
	Mode       string `mapstructure:"mode"`
}

// Extension contributes the observability config section as family-owned config surface.
type Extension struct {
	Observability Config `mapstructure:"observability"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"observability"} }

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("observability.log_level", "info")
	v.SetDefault("observability.log_format", "json")
	v.SetDefault("observability.tracing_enabled", false)
	v.SetDefault("observability.tracing_sample_rate", 0.1)
	v.SetDefault("observability.async_logging.enabled", false)
	v.SetDefault("observability.async_logging.queue_size", 1024)
	v.SetDefault("observability.async_logging.worker_count", 1)
	v.SetDefault("observability.async_logging.drop_when_full", true)
	v.SetDefault("observability.request_logging.enabled", true)
	v.SetDefault("observability.request_logging.log_start", true)
	v.SetDefault("observability.request_logging.output", "logger")
	v.SetDefault("observability.request_logging.fields", []string{"request_id", "method", "path", "status", "duration_ms", "remote_addr", "error"})
	v.SetDefault("observability.request_tracing.enabled", true)
	v.SetDefault("observability.request_timeout.enabled", false)
	v.SetDefault("observability.request_timeout.default", 15*time.Second)
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"observability.log_level", "OBSERVABILITY_LOG_LEVEL",
		"observability.log_format", "OBSERVABILITY_LOG_FORMAT",
		"observability.service_name", "OBSERVABILITY_SERVICE_NAME",
		"observability.tracing_enabled", "OBSERVABILITY_TRACING_ENABLED",
		"observability.tracing_sample_rate", "OBSERVABILITY_TRACING_SAMPLE_RATE",
		"observability.tracing_endpoint", "OBSERVABILITY_TRACING_ENDPOINT",
		"observability.async_logging.enabled", "OBSERVABILITY_ASYNC_LOGGING_ENABLED",
		"observability.async_logging.queue_size", "OBSERVABILITY_ASYNC_LOGGING_QUEUE_SIZE",
		"observability.async_logging.worker_count", "OBSERVABILITY_ASYNC_LOGGING_WORKER_COUNT",
		"observability.async_logging.drop_when_full", "OBSERVABILITY_ASYNC_LOGGING_DROP_WHEN_FULL",
		"observability.request_logging.enabled", "OBSERVABILITY_REQUEST_LOGGING_ENABLED",
		"observability.request_logging.log_start", "OBSERVABILITY_REQUEST_LOGGING_LOG_START",
		"observability.request_logging.output", "OBSERVABILITY_REQUEST_LOGGING_OUTPUT",
		"observability.request_logging.fields", "OBSERVABILITY_REQUEST_LOGGING_FIELDS",
		"observability.request_tracing.enabled", "OBSERVABILITY_REQUEST_TRACING_ENABLED",
		"observability.request_timeout.enabled", "OBSERVABILITY_REQUEST_TIMEOUT_ENABLED",
		"observability.request_timeout.default", "OBSERVABILITY_REQUEST_TIMEOUT_DEFAULT",
	)
}

func (e Extension) Validate() error {
	e.Observability.RequestLogging.Fields = normalizeStringSlice(e.Observability.RequestLogging.Fields)
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, strings.ToLower(strings.TrimSpace(e.Observability.LogLevel))) {
		return fmt.Errorf("invalid observability.log_level: %s (must be one of: %v)", e.Observability.LogLevel, validLogLevels)
	}
	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, strings.ToLower(strings.TrimSpace(e.Observability.LogFormat))) {
		return fmt.Errorf("invalid observability.log_format: %s (must be one of: %v)", e.Observability.LogFormat, validLogFormats)
	}
	if e.Observability.AsyncLogging.Enabled {
		if e.Observability.AsyncLogging.QueueSize <= 0 {
			return errors.New("observability.async_logging.queue_size must be greater than 0 when async logging is enabled")
		}
		if e.Observability.AsyncLogging.WorkerCount <= 0 {
			return errors.New("observability.async_logging.worker_count must be greater than 0 when async logging is enabled")
		}
	}
	validOutputs := []string{"logger", "stdout", "stderr"}
	output := strings.ToLower(strings.TrimSpace(e.Observability.RequestLogging.Output))
	if output == "" {
		output = "logger"
	}
	if !contains(validOutputs, output) {
		return fmt.Errorf("observability.request_logging.output must be one of %v", validOutputs)
	}
	validFields := []string{
		"request_id", "method", "path", "status", "duration_ms", "error",
		"remote_addr", "remote_port", "request_method", "request_uri", "uri",
		"args", "query_string", "request_time", "time_local", "host",
		"server_protocol", "scheme", "http_referer", "http_user_agent",
		"x_forwarded_for", "remote_user", "request_length",
	}
	for index, field := range e.Observability.RequestLogging.Fields {
		normalizedField := strings.ToLower(strings.TrimSpace(field))
		if !contains(validFields, normalizedField) {
			return fmt.Errorf("observability.request_logging.fields[%d] must be one of %v", index, validFields)
		}
	}
	validRequestLoggingModes := []string{"off", "minimal", "full"}
	for index, policy := range e.Observability.RequestLogging.PathPolicies {
		if strings.TrimSpace(policy.PathPrefix) == "" {
			return fmt.Errorf("observability.request_logging.path_policies[%d].path_prefix is required", index)
		}
		if !contains(validRequestLoggingModes, strings.ToLower(strings.TrimSpace(policy.Mode))) {
			return fmt.Errorf("observability.request_logging.path_policies[%d].mode must be one of %v", index, validRequestLoggingModes)
		}
	}
	validRequestTracingModes := []string{"off", "minimal", "full"}
	for index, policy := range e.Observability.RequestTracing.PathPolicies {
		if strings.TrimSpace(policy.PathPrefix) == "" {
			return fmt.Errorf("observability.request_tracing.path_policies[%d].path_prefix is required", index)
		}
		if !contains(validRequestTracingModes, strings.ToLower(strings.TrimSpace(policy.Mode))) {
			return fmt.Errorf("observability.request_tracing.path_policies[%d].mode must be one of %v", index, validRequestTracingModes)
		}
	}
	if e.Observability.RequestTimeout.Enabled && e.Observability.RequestTimeout.Default <= 0 {
		return errors.New("observability.request_timeout.default must be greater than zero when request timeout is enabled")
	}
	validRequestTimeoutModes := []string{"off", "on"}
	for index, policy := range e.Observability.RequestTimeout.PathPolicies {
		if strings.TrimSpace(policy.PathPrefix) == "" {
			return fmt.Errorf("observability.request_timeout.path_policies[%d].path_prefix is required", index)
		}
		if !contains(validRequestTimeoutModes, strings.ToLower(strings.TrimSpace(policy.Mode))) {
			return fmt.Errorf("observability.request_timeout.path_policies[%d].mode must be one of %v", index, validRequestTimeoutModes)
		}
	}
	if e.Observability.TracingEnabled && strings.TrimSpace(e.Observability.TracingEndpoint) == "" {
		return errors.New("observability.tracing_endpoint is required when tracing is enabled")
	}
	return nil
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	for index := 0; index < len(values); index += 2 {
		if err := v.BindEnv(values[index], prefixedEnv(prefix, values[index+1])); err != nil {
			return err
		}
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.TrimSpace(prefix) + "_" + suffix
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
