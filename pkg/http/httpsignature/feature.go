package httpsignature

import (
	"errors"
	"fmt"
	"strings"
	"time"

	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	httpsignatureconfig "github.com/nimburion/nimburion/pkg/http/httpsignature/config"
	"github.com/spf13/viper"
)

type Extension struct {
	Security httpsignatureconfig.SecurityConfig `mapstructure:"security"`
}

func (Extension) DisabledCoreConfigSections() []string { return []string{"security"} }

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

func (e Extension) Validate() error {
	cfg := e.Security.HTTPSignature
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.KeyIDHeader) == "" {
		return errors.New("security.http_signature.key_id_header is required when security.http_signature.enabled is true")
	}
	if strings.TrimSpace(cfg.TimestampHeader) == "" {
		return errors.New("security.http_signature.timestamp_header is required when security.http_signature.enabled is true")
	}
	if strings.TrimSpace(cfg.SignatureHeader) == "" {
		return errors.New("security.http_signature.signature_header is required when security.http_signature.enabled is true")
	}
	if cfg.RequireNonce && strings.TrimSpace(cfg.NonceHeader) == "" {
		return errors.New("security.http_signature.nonce_header is required when security.http_signature.require_nonce is true")
	}
	if cfg.MaxClockSkew <= 0 {
		return errors.New("security.http_signature.max_clock_skew must be greater than zero when security.http_signature.enabled is true")
	}
	if cfg.NonceTTL <= 0 {
		return errors.New("security.http_signature.nonce_ttl must be greater than zero when security.http_signature.enabled is true")
	}
	if len(cfg.StaticKeys) == 0 {
		return errors.New("security.http_signature.static_keys must contain at least one key when security.http_signature.enabled is true")
	}
	for keyID, secret := range cfg.StaticKeys {
		if strings.TrimSpace(keyID) == "" {
			return errors.New("security.http_signature.static_keys contains an empty key id")
		}
		if strings.TrimSpace(secret) == "" {
			return fmt.Errorf("security.http_signature.static_keys.%s cannot be empty", keyID)
		}
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

type configFeature struct{}

func (configFeature) Name() string { return "http-signature-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "security", Extension: &Extension{}},
		},
	}
}

func NewConfigFeature() corefeature.Feature { return configFeature{} }
