package httpsignature

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	httpsignatureconfig "github.com/nimburion/nimburion/pkg/http/httpsignature/config"
)

// Extension contributes the HTTP signature config section as family-owned config surface.
type Extension struct {
	Security httpsignatureconfig.SecurityConfig `mapstructure:"security"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"security"} }

// ApplyDefaults registers default HTTP signature configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("security.http_signature.enabled", false)
	v.SetDefault("security.http_signature.key_id_header", "X-Key-Id")
	v.SetDefault("security.http_signature.timestamp_header", "X-Timestamp")
	v.SetDefault("security.http_signature.nonce_header", "X-Nonce")
	v.SetDefault("security.http_signature.signature_header", "X-Signature")
	v.SetDefault("security.http_signature.max_clock_skew", 5*time.Minute)
	v.SetDefault("security.http_signature.nonce_ttl", 10*time.Minute)
	v.SetDefault("security.http_signature.require_nonce", true)
	v.SetDefault("security.http_signature.excluded_path_prefixes", []string{"/health", "/metrics", "/ready", "/version"})
	v.SetDefault("security.http_signature.static_keys", map[string]string{})
}

// BindEnv binds HTTP signature configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"security.http_signature.enabled", "SECURITY_HTTP_SIGNATURE_ENABLED",
		"security.http_signature.key_id_header", "SECURITY_HTTP_SIGNATURE_KEY_ID_HEADER",
		"security.http_signature.timestamp_header", "SECURITY_HTTP_SIGNATURE_TIMESTAMP_HEADER",
		"security.http_signature.nonce_header", "SECURITY_HTTP_SIGNATURE_NONCE_HEADER",
		"security.http_signature.signature_header", "SECURITY_HTTP_SIGNATURE_SIGNATURE_HEADER",
		"security.http_signature.max_clock_skew", "SECURITY_HTTP_SIGNATURE_MAX_CLOCK_SKEW",
		"security.http_signature.nonce_ttl", "SECURITY_HTTP_SIGNATURE_NONCE_TTL",
		"security.http_signature.require_nonce", "SECURITY_HTTP_SIGNATURE_REQUIRE_NONCE",
		"security.http_signature.excluded_path_prefixes", "SECURITY_HTTP_SIGNATURE_EXCLUDED_PATH_PREFIXES",
	)
}

// Validate checks that enabled HTTP signature configuration is coherent.
func (e Extension) Validate() error {
	cfg := e.Security.HTTPSignature
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.KeyIDHeader) == "" {
		return validationError("validation.security.http_signature.key_id_header.required", "security.http_signature.key_id_header is required when security.http_signature.enabled is true")
	}
	if strings.TrimSpace(cfg.TimestampHeader) == "" {
		return validationError("validation.security.http_signature.timestamp_header.required", "security.http_signature.timestamp_header is required when security.http_signature.enabled is true")
	}
	if strings.TrimSpace(cfg.SignatureHeader) == "" {
		return validationError("validation.security.http_signature.signature_header.required", "security.http_signature.signature_header is required when security.http_signature.enabled is true")
	}
	if cfg.RequireNonce && strings.TrimSpace(cfg.NonceHeader) == "" {
		return validationError("validation.security.http_signature.nonce_header.required", "security.http_signature.nonce_header is required when security.http_signature.require_nonce is true")
	}
	if cfg.MaxClockSkew <= 0 {
		return validationError("validation.security.http_signature.max_clock_skew.invalid", "security.http_signature.max_clock_skew must be greater than zero when security.http_signature.enabled is true")
	}
	if cfg.NonceTTL <= 0 {
		return validationError("validation.security.http_signature.nonce_ttl.invalid", "security.http_signature.nonce_ttl must be greater than zero when security.http_signature.enabled is true")
	}
	if len(cfg.StaticKeys) == 0 {
		return validationError("validation.security.http_signature.static_keys.required", "security.http_signature.static_keys must contain at least one key when security.http_signature.enabled is true")
	}
	for keyID, secret := range cfg.StaticKeys {
		if strings.TrimSpace(keyID) == "" {
			return validationError("validation.security.http_signature.static_keys.key_id.empty", "security.http_signature.static_keys contains an empty key id")
		}
		if strings.TrimSpace(secret) == "" {
			return validationErrorf("validation.security.http_signature.static_keys.secret.empty", "security.http_signature.static_keys.%s cannot be empty", keyID)
		}
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

type configFeature struct{}

func (configFeature) Name() string { return "http-signature-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "security", Extension: &Extension{}},
		},
	}
}

// NewConfigFeature returns the feature that registers the HTTP signature config extension.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
