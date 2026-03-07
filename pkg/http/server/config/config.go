package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// HTTPConfig configures the public API server.
type HTTPConfig struct {
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	MaxRequestSize int64         `mapstructure:"max_request_size"`
}

// ManagementConfig configures the management server.
type ManagementConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	AuthEnabled  bool          `mapstructure:"auth_enabled"`
	MTLSEnabled  bool          `mapstructure:"mtls_enabled"`
	TLSCertFile  string        `mapstructure:"tls_cert_file"`
	TLSKeyFile   string        `mapstructure:"tls_key_file"`
	TLSCAFile    string        `mapstructure:"tls_ca_file"`
}

// Extension contributes the http and management config sections as HTTP-server-owned config surface.
type Extension struct {
	HTTP       HTTPConfig       `mapstructure:"http"`
	Management ManagementConfig `mapstructure:"management"`
}

// DisabledCoreConfigSections disables the legacy monolithic root sections when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"http", "management"} }

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
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"http.port", "HTTP_PORT",
		"http.read_timeout", "HTTP_READ_TIMEOUT",
		"http.write_timeout", "HTTP_WRITE_TIMEOUT",
		"http.idle_timeout", "HTTP_IDLE_TIMEOUT",
		"http.max_request_size", "HTTP_MAX_REQUEST_SIZE",
		"management.enabled", "MGMT_ENABLED",
		"management.port", "MGMT_PORT",
		"management.read_timeout", "MGMT_READ_TIMEOUT",
		"management.write_timeout", "MGMT_WRITE_TIMEOUT",
		"management.auth_enabled", "MGMT_AUTH_ENABLED",
		"management.mtls_enabled", "MGMT_MTLS_ENABLED",
		"management.tls_cert_file", "MGMT_TLS_CERT_FILE",
		"management.tls_key_file", "MGMT_TLS_KEY_FILE",
		"management.tls_ca_file", "MGMT_TLS_CA_FILE",
	)
}

func (e Extension) Validate() error {
	if e.HTTP.Port <= 0 || e.HTTP.Port > 65535 {
		return fmt.Errorf("invalid http.port: %d (must be between 1 and 65535)", e.HTTP.Port)
	}
	if e.Management.MTLSEnabled {
		if strings.TrimSpace(e.Management.TLSCertFile) == "" {
			return errors.New("management.tls_cert_file is required when management.mtls_enabled is true")
		}
		if strings.TrimSpace(e.Management.TLSKeyFile) == "" {
			return errors.New("management.tls_key_file is required when management.mtls_enabled is true")
		}
		if strings.TrimSpace(e.Management.TLSCAFile) == "" {
			return errors.New("management.tls_ca_file is required when management.mtls_enabled is true")
		}
	}
	if e.Management.Enabled && (e.Management.Port <= 0 || e.Management.Port > 65535) {
		return errors.New("management.port must be between 1 and 65535 when management is enabled")
	}
	if e.HTTP.MaxRequestSize < 0 {
		return errors.New("http.max_request_size cannot be negative")
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
