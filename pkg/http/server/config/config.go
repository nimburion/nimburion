package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// HTTPConfig configures the public API server.
type HTTPConfig struct {
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	MaxRequestSize int64         `mapstructure:"max_request_size"`
	RequireTLS     bool          `mapstructure:"require_tls"`
}

// ManagementConfig configures the management server.
type ManagementConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	AuthEnabled    bool          `mapstructure:"auth_enabled"`
	AllowlistCIDRs []string      `mapstructure:"allowlist_cidrs"`
	MTLSEnabled    bool          `mapstructure:"mtls_enabled"`
	RequireTLS     bool          `mapstructure:"require_tls"`
	TLSCertFile    string        `mapstructure:"tls_cert_file"`
	TLSKeyFile     string        `mapstructure:"tls_key_file"`
	TLSCAFile      string        `mapstructure:"tls_ca_file"`
}

// Extension contributes the http and management config sections as HTTP-server-owned config surface.
type Extension struct {
	HTTP       HTTPConfig       `mapstructure:"http"`
	Management ManagementConfig `mapstructure:"management"`
}

// DisabledCoreConfigSections disables the legacy monolithic root sections when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"http", "management"} }

// ApplyDefaults registers default HTTP server configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.read_timeout", 30*time.Second)
	v.SetDefault("http.write_timeout", 30*time.Second)
	v.SetDefault("http.idle_timeout", 120*time.Second)
	v.SetDefault("http.max_request_size", int64(1<<20))
	v.SetDefault("management.enabled", true)
	v.SetDefault("management.port", 9090)
	v.SetDefault("management.read_timeout", 10*time.Second)
	v.SetDefault("management.write_timeout", 10*time.Second)
	v.SetDefault("management.auth_enabled", false)
	v.SetDefault("management.mtls_enabled", false)
	v.SetDefault("management.allowlist_cidrs", []string{})
	v.SetDefault("management.require_tls", false)
	v.SetDefault("http.require_tls", false)
}

// BindEnv binds HTTP server configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"http.port", "HTTP_PORT",
		"http.read_timeout", "HTTP_READ_TIMEOUT",
		"http.write_timeout", "HTTP_WRITE_TIMEOUT",
		"http.idle_timeout", "HTTP_IDLE_TIMEOUT",
		"http.max_request_size", "HTTP_MAX_REQUEST_SIZE",
		"http.require_tls", "HTTP_REQUIRE_TLS",
		"management.enabled", "MGMT_ENABLED",
		"management.port", "MGMT_PORT",
		"management.read_timeout", "MGMT_READ_TIMEOUT",
		"management.write_timeout", "MGMT_WRITE_TIMEOUT",
		"management.auth_enabled", "MGMT_AUTH_ENABLED",
		"management.allowlist_cidrs", "MGMT_ALLOWLIST_CIDRS",
		"management.mtls_enabled", "MGMT_MTLS_ENABLED",
		"management.require_tls", "MGMT_REQUIRE_TLS",
		"management.tls_cert_file", "MGMT_TLS_CERT_FILE",
		"management.tls_key_file", "MGMT_TLS_KEY_FILE",
		"management.tls_ca_file", "MGMT_TLS_CA_FILE",
	)
}

// Validate checks that HTTP server configuration is coherent.
func (e Extension) Validate() error {
	if e.HTTP.Port <= 0 || e.HTTP.Port > 65535 {
		return validationErrorf("validation.http.port.invalid", "invalid http.port: %d (must be between 1 and 65535)", e.HTTP.Port)
	}
	if e.Management.MTLSEnabled {
		if strings.TrimSpace(e.Management.TLSCertFile) == "" {
			return validationError("validation.management.tls_cert_file.required", "management.tls_cert_file is required when management.mtls_enabled is true")
		}
		if strings.TrimSpace(e.Management.TLSKeyFile) == "" {
			return validationError("validation.management.tls_key_file.required", "management.tls_key_file is required when management.mtls_enabled is true")
		}
		if strings.TrimSpace(e.Management.TLSCAFile) == "" {
			return validationError("validation.management.tls_ca_file.required", "management.tls_ca_file is required when management.mtls_enabled is true")
		}
	}
	if e.Management.Enabled && (e.Management.Port <= 0 || e.Management.Port > 65535) {
		return validationError("validation.management.port.invalid", "management.port must be between 1 and 65535 when management is enabled")
	}
	if e.HTTP.MaxRequestSize < 0 {
		return validationError("validation.http.max_request_size.invalid", "http.max_request_size cannot be negative")
	}
	return nil
}

func validationError(code, message string) error {
	return coreerrors.NewValidationWithCode(code, message, nil, nil)
}

func validationErrorf(code, format string, args ...any) error {
	return validationError(code, fmt.Sprintf(format, args...))
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	if len(values)%2 != 0 {
		return fmt.Errorf("bindEnvPairs requires even number of values, got %d", len(values))
	}
	for len(values) > 0 {
		key, suffix := values[0], values[1]
		if err := v.BindEnv(key, prefixedEnv(prefix, suffix)); err != nil {
			return err
		}
		values = values[2:]
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.TrimSpace(prefix) + "_" + suffix
}
