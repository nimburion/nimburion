package config

import (
	"fmt"

	"github.com/spf13/viper"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
)

// Config configures HTTP security headers policy.
type Config struct {
	Enabled                   bool              `mapstructure:"enabled"`
	IsDevelopment             bool              `mapstructure:"is_development"`
	AllowedHosts              []string          `mapstructure:"allowed_hosts"`
	SSLRedirect               bool              `mapstructure:"ssl_redirect"`
	SSLTemporaryRedirect      bool              `mapstructure:"ssl_temporary_redirect"`
	SSLHost                   string            `mapstructure:"ssl_host"`
	SSLProxyHeaders           map[string]string `mapstructure:"ssl_proxy_headers"`
	DontRedirectIPV4Hostnames bool              `mapstructure:"dont_redirect_ipv4_hostnames"`
	STSSeconds                int64             `mapstructure:"sts_seconds"`
	STSIncludeSubdomains      bool              `mapstructure:"sts_include_subdomains"`
	STSPreload                bool              `mapstructure:"sts_preload"`
	CustomFrameOptions        string            `mapstructure:"custom_frame_options"`
	ContentTypeNosniff        bool              `mapstructure:"content_type_nosniff"`
	ContentSecurityPolicy     string            `mapstructure:"content_security_policy"`
	ReferrerPolicy            string            `mapstructure:"referrer_policy"`
	PermissionsPolicy         string            `mapstructure:"permissions_policy"`
	IENoOpen                  bool              `mapstructure:"ie_no_open"`
	XDNSPrefetchControl       string            `mapstructure:"x_dns_prefetch_control"`
	CrossOriginOpenerPolicy   string            `mapstructure:"cross_origin_opener_policy"`
	CrossOriginResourcePolicy string            `mapstructure:"cross_origin_resource_policy"`
	CrossOriginEmbedderPolicy string            `mapstructure:"cross_origin_embedder_policy"`
	CustomHeaders             map[string]string `mapstructure:"custom_headers"`
}

// Extension contributes the security headers config section as family-owned config surface.
type Extension struct {
	SecurityHeaders Config `mapstructure:"security_headers"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"security_headers"} }

// ApplyDefaults registers default security headers configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("security_headers.enabled", true)
	v.SetDefault("security_headers.is_development", false)
	v.SetDefault("security_headers.allowed_hosts", []string{})
	v.SetDefault("security_headers.ssl_redirect", false)
	v.SetDefault("security_headers.ssl_temporary_redirect", false)
	v.SetDefault("security_headers.ssl_proxy_headers", map[string]string{"X-Forwarded-Proto": "https"})
	v.SetDefault("security_headers.dont_redirect_ipv4_hostnames", true)
	v.SetDefault("security_headers.sts_seconds", int64(31536000))
	v.SetDefault("security_headers.sts_include_subdomains", true)
	v.SetDefault("security_headers.sts_preload", false)
	v.SetDefault("security_headers.custom_frame_options", "DENY")
	v.SetDefault("security_headers.content_type_nosniff", true)
	v.SetDefault("security_headers.content_security_policy", "default-src 'self'")
	v.SetDefault("security_headers.referrer_policy", "strict-origin-when-cross-origin")
	v.SetDefault("security_headers.permissions_policy", "geolocation=()")
	v.SetDefault("security_headers.ie_no_open", true)
	v.SetDefault("security_headers.x_dns_prefetch_control", "off")
	v.SetDefault("security_headers.cross_origin_opener_policy", "same-origin")
	v.SetDefault("security_headers.cross_origin_resource_policy", "same-origin")
	v.SetDefault("security_headers.custom_headers", map[string]string{})
}

// BindEnv binds security headers configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"security_headers.enabled", "SECURITY_HEADERS_ENABLED",
		"security_headers.is_development", "SECURITY_HEADERS_IS_DEVELOPMENT",
		"security_headers.allowed_hosts", "SECURITY_HEADERS_ALLOWED_HOSTS",
		"security_headers.ssl_redirect", "SECURITY_HEADERS_SSL_REDIRECT",
		"security_headers.ssl_temporary_redirect", "SECURITY_HEADERS_SSL_TEMPORARY_REDIRECT",
		"security_headers.ssl_host", "SECURITY_HEADERS_SSL_HOST",
		"security_headers.ssl_proxy_headers", "SECURITY_HEADERS_SSL_PROXY_HEADERS",
		"security_headers.dont_redirect_ipv4_hostnames", "SECURITY_HEADERS_DONT_REDIRECT_IPV4_HOSTNAMES",
		"security_headers.sts_seconds", "SECURITY_HEADERS_STS_SECONDS",
		"security_headers.sts_include_subdomains", "SECURITY_HEADERS_STS_INCLUDE_SUBDOMAINS",
		"security_headers.sts_preload", "SECURITY_HEADERS_STS_PRELOAD",
		"security_headers.custom_frame_options", "SECURITY_HEADERS_CUSTOM_FRAME_OPTIONS",
		"security_headers.content_type_nosniff", "SECURITY_HEADERS_CONTENT_TYPE_NOSNIFF",
		"security_headers.content_security_policy", "SECURITY_HEADERS_CONTENT_SECURITY_POLICY",
		"security_headers.referrer_policy", "SECURITY_HEADERS_REFERRER_POLICY",
		"security_headers.permissions_policy", "SECURITY_HEADERS_PERMISSIONS_POLICY",
		"security_headers.ie_no_open", "SECURITY_HEADERS_IE_NO_OPEN",
		"security_headers.x_dns_prefetch_control", "SECURITY_HEADERS_X_DNS_PREFETCH_CONTROL",
		"security_headers.cross_origin_opener_policy", "SECURITY_HEADERS_CROSS_ORIGIN_OPENER_POLICY",
		"security_headers.cross_origin_resource_policy", "SECURITY_HEADERS_CROSS_ORIGIN_RESOURCE_POLICY",
		"security_headers.cross_origin_embedder_policy", "SECURITY_HEADERS_CROSS_ORIGIN_EMBEDDER_POLICY",
		"security_headers.custom_headers", "SECURITY_HEADERS_CUSTOM_HEADERS",
	)
}

// Validate checks that security headers configuration is coherent.
func (e Extension) Validate() error {
	if e.SecurityHeaders.STSSeconds < 0 {
		return coreerrors.NewValidationWithCode("validation.security_headers.sts_seconds.invalid", "security_headers.sts_seconds cannot be negative", nil, nil)
	}
	return nil
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
	if prefix == "" {
		return suffix
	}
	return prefix + "_" + suffix
}
