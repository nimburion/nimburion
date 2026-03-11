package config

import "time"

// Config configures HTTP signature verification behavior.
type Config struct {
	Enabled              bool              `mapstructure:"enabled"`
	KeyIDHeader          string            `mapstructure:"key_id_header"`
	TimestampHeader      string            `mapstructure:"timestamp_header"`
	NonceHeader          string            `mapstructure:"nonce_header"`
	SignatureHeader      string            `mapstructure:"signature_header"`
	MaxClockSkew         time.Duration     `mapstructure:"max_clock_skew"`
	NonceTTL             time.Duration     `mapstructure:"nonce_ttl"`
	RequireNonce         bool              `mapstructure:"require_nonce"`
	ExcludedPathPrefixes []string          `mapstructure:"excluded_path_prefixes"`
	StaticKeys           map[string]string `mapstructure:"static_keys"`
}

// SecurityConfig preserves the public `security.http_signature` nesting owned by the HTTP signature family.
type SecurityConfig struct {
	HTTPSignature Config `mapstructure:"http_signature"`
}
