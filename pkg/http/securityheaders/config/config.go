package config

import (
	"errors"
	"github.com/spf13/viper"
)

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

type Extension struct {
	SecurityHeaders Config `mapstructure:"security_headers"`
}

func (Extension) DisabledCoreConfigSections() []string { return []string{"security_headers"} }

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

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"security_headers.enabled", "SECURITY_HEADERS_ENABLED",
		"security_headers.is_development", "SECURITY_HEADERS_IS_DEVELOPMENT",
		"security_headers.allowed_hosts", "SECURITY_HEADERS_ALLOWED_HOSTS",
		"security_headers.ssl_redirect", "SECURITY_HEADERS_SSL_REDIRECT",
		"security_headers.ssl_temporary_redirect", "SECURITY_HEADERS_SSL_TEMPORARY_REDIRECT",
		"security_headers.ssl_host", "SECURITY_HEADERS_SSL_HOST",
		"security_headers.dont_redirect_ipv4_hostnames", "SECURITY_HEADERS_DONT_REDIRECT_IPV4_HOSTNAMES",
		"security_headers.sts_seconds", "SECURITY_HEADERS_STS_SECONDS",
		"security_headers.sts_include_subdomains", "SECURITY_HEADERS_STS_INCLUDE_SUBDOMAINS",
		"security_headers.sts_preload", "SECURITY_HEADERS_STS_PRELOAD",
	)
}

func (e Extension) Validate() error {
	if e.SecurityHeaders.STSSeconds < 0 {
		return errors.New("security_headers.sts_seconds cannot be negative")
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
	if prefix == "" {
		return suffix
	}
	return prefix + "_" + suffix
}
