package config

import (
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/spf13/viper"
)

// Config configures OAuth2/OIDC authentication integration.
type Config struct {
	Enabled      bool          `mapstructure:"enabled"`
	Issuer       string        `mapstructure:"issuer"`
	JWKSUrl      string        `mapstructure:"jwks_url"`
	JWKSCacheTTL time.Duration `mapstructure:"jwks_cache_ttl"`
	Audience     string        `mapstructure:"audience"`
	Claims       ClaimsConfig  `mapstructure:"claims"`
}

// ClaimsConfig configures claim mappings and declarative guard rules.
type ClaimsConfig struct {
	Rules    []ClaimRule         `mapstructure:"rules"`
	Mappings map[string][]string `mapstructure:"mappings"`
}

// ClaimRule configures one declarative claim guard rule.
type ClaimRule struct {
	Claim    string   `mapstructure:"claim"`
	Aliases  []string `mapstructure:"aliases"`
	Source   string   `mapstructure:"source"`
	Key      string   `mapstructure:"key"`
	Operator string   `mapstructure:"operator"`
	Values   []string `mapstructure:"values"`
	Optional bool     `mapstructure:"optional"`
}

// Extension contributes the auth config section as family-owned config surface.
type Extension struct {
	Auth Config `mapstructure:"auth"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"auth"} }

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.jwks_cache_ttl", time.Hour)
	v.SetDefault("auth.claims.mappings", map[string][]string{
		"subject":   {"sub"},
		"issuer":    {"iss"},
		"audience":  {"aud"},
		"scopes":    {"scope", "scopes"},
		"roles":     {"role", "roles"},
		"tenant_id": {"tenant_id", "tenantId"},
	})
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"auth.enabled", "AUTH_ENABLED",
		"auth.issuer", "AUTH_ISSUER",
		"auth.jwks_url", "AUTH_JWKS_URL",
		"auth.jwks_cache_ttl", "AUTH_JWKS_CACHE_TTL",
		"auth.audience", "AUTH_AUDIENCE",
	)
}

func (e Extension) Validate() error {
	if !e.Auth.Enabled {
		return nil
	}
	if strings.TrimSpace(e.Auth.Issuer) == "" {
		return validationError("validation.auth.issuer.required", "auth.issuer is required when auth is enabled")
	}
	if strings.TrimSpace(e.Auth.JWKSUrl) == "" {
		return validationError("validation.auth.jwks_url.required", "auth.jwks_url is required when auth is enabled")
	}
	if strings.TrimSpace(e.Auth.Audience) == "" {
		return validationError("validation.auth.audience.required", "auth.audience is required when auth is enabled")
	}
	if len(e.Auth.Claims.Rules) == 0 {
		return validationError("validation.auth.claims.rules.required", "auth.claims.rules must contain at least one rule when auth is enabled")
	}
	validSources := []string{"route", "header", "query"}
	validOperators := []string{"required", "equals", "one_of", "regex"}
	for index, rule := range e.Auth.Claims.Rules {
		if strings.TrimSpace(rule.Claim) == "" {
			return validationErrorf("validation.auth.claims.rules.claim.required", "auth.claims.rules[%d].claim is required", index)
		}
		operator := strings.ToLower(strings.TrimSpace(rule.Operator))
		if operator == "" {
			operator = "required"
		}
		if !contains(validOperators, operator) {
			return validationErrorf("validation.auth.claims.rules.operator.invalid", "auth.claims.rules[%d].operator must be one of %v", index, validOperators)
		}
		switch operator {
		case "equals":
			source := strings.ToLower(strings.TrimSpace(rule.Source))
			if !contains(validSources, source) {
				return validationErrorf("validation.auth.claims.rules.source.invalid", "auth.claims.rules[%d].source must be one of %v when operator=equals", index, validSources)
			}
			if strings.TrimSpace(rule.Key) == "" {
				return validationErrorf("validation.auth.claims.rules.key.required", "auth.claims.rules[%d].key is required when operator=equals", index)
			}
		case "one_of":
			if len(rule.Values) == 0 {
				return validationErrorf("validation.auth.claims.rules.values.required", "auth.claims.rules[%d].values must contain at least one value when operator=one_of", index)
			}
		case "regex":
			if len(rule.Values) != 1 || strings.TrimSpace(rule.Values[0]) == "" {
				return validationErrorf("validation.auth.claims.rules.regex.required", "auth.claims.rules[%d].values must contain exactly one regex pattern when operator=regex", index)
			}
		}
	}
	for claimName, aliases := range e.Auth.Claims.Mappings {
		if strings.TrimSpace(claimName) == "" {
			return validationError("validation.auth.claims.mappings.claim_name.empty", "auth.claims.mappings contains an empty claim name")
		}
		for aliasIndex, alias := range aliases {
			if strings.TrimSpace(alias) == "" {
				return validationErrorf("validation.auth.claims.mappings.alias.empty", "auth.claims.mappings.%s[%d] cannot be empty", claimName, aliasIndex)
			}
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
