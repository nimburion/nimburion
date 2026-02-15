package securityheaders

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// Config defines security headers and transport hardening options.
type Config struct {
	Enabled       bool
	IsDevelopment bool

	AllowedHosts []string

	SSLRedirect          bool
	SSLTemporaryRedirect bool
	SSLHost              string
	SSLProxyHeaders      map[string]string

	// DontRedirectIPV4Hostnames allows IPv4 LB health checks over HTTP.
	DontRedirectIPV4Hostnames bool

	STSSeconds                int64
	STSIncludeSubdomains      bool
	STSPreload                bool
	CustomFrameOptions        string
	ContentTypeNosniff        bool
	ContentSecurityPolicy     string
	ReferrerPolicy            string
	PermissionsPolicy         string
	IENoOpen                  bool
	XDNSPrefetchControl       string
	CrossOriginOpenerPolicy   string
	CrossOriginResourcePolicy string
	CrossOriginEmbedderPolicy string

	CustomHeaders map[string]string
}

// DefaultConfig returns strict but safe defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:                   true,
		IsDevelopment:             false,
		AllowedHosts:              []string{},
		SSLRedirect:               false,
		SSLTemporaryRedirect:      false,
		SSLHost:                   "",
		SSLProxyHeaders:           map[string]string{"X-Forwarded-Proto": "https"},
		DontRedirectIPV4Hostnames: true,
		STSSeconds:                31536000, // 1 year
		STSIncludeSubdomains:      true,
		STSPreload:                false,
		CustomFrameOptions:        "DENY",
		ContentTypeNosniff:        true,
		ContentSecurityPolicy:     "default-src 'self'",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "geolocation=(), microphone=(), camera=()",
		IENoOpen:                  true,
		XDNSPrefetchControl:       "off",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		CrossOriginEmbedderPolicy: "",
		CustomHeaders:             map[string]string{},
	}
}

// Middleware applies security headers and optional HTTPS redirect.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalize(cfg)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled || cfg.IsDevelopment {
				return next(c)
			}

			if !checkAllowedHosts(c.Request(), cfg.AllowedHosts) {
				return c.String(http.StatusForbidden, "forbidden host")
			}

			secureReq := isSecureRequest(c.Request(), cfg)
			if cfg.SSLRedirect && !secureReq {
				redirectURL := redirectTarget(c.Request(), cfg)
				status := http.StatusMovedPermanently
				if cfg.SSLTemporaryRedirect {
					status = http.StatusTemporaryRedirect
				}
				http.Redirect(c.Response(), c.Request(), redirectURL, status)
				return nil
			}

			applyHeaders(c.Response().Header(), cfg, secureReq)
			return next(c)
		}
	}
}

func normalize(cfg Config) Config {
	defaults := DefaultConfig()

	if cfg.SSLProxyHeaders == nil {
		cfg.SSLProxyHeaders = defaults.SSLProxyHeaders
	}
	if cfg.CustomHeaders == nil {
		cfg.CustomHeaders = defaults.CustomHeaders
	}
	if cfg.AllowedHosts == nil {
		cfg.AllowedHosts = defaults.AllowedHosts
	}
	if strings.TrimSpace(cfg.CustomFrameOptions) == "" {
		cfg.CustomFrameOptions = defaults.CustomFrameOptions
	}
	if strings.TrimSpace(cfg.ContentSecurityPolicy) == "" {
		cfg.ContentSecurityPolicy = defaults.ContentSecurityPolicy
	}
	if strings.TrimSpace(cfg.ReferrerPolicy) == "" {
		cfg.ReferrerPolicy = defaults.ReferrerPolicy
	}
	if strings.TrimSpace(cfg.PermissionsPolicy) == "" {
		cfg.PermissionsPolicy = defaults.PermissionsPolicy
	}
	if strings.TrimSpace(cfg.XDNSPrefetchControl) == "" {
		cfg.XDNSPrefetchControl = defaults.XDNSPrefetchControl
	}
	if strings.TrimSpace(cfg.CrossOriginOpenerPolicy) == "" {
		cfg.CrossOriginOpenerPolicy = defaults.CrossOriginOpenerPolicy
	}
	if strings.TrimSpace(cfg.CrossOriginResourcePolicy) == "" {
		cfg.CrossOriginResourcePolicy = defaults.CrossOriginResourcePolicy
	}
	if cfg.STSSeconds == 0 {
		cfg.STSSeconds = defaults.STSSeconds
	}

	return cfg
}

func applyHeaders(h http.Header, cfg Config, secureReq bool) {
	if strings.TrimSpace(cfg.CustomFrameOptions) != "" {
		h.Set("X-Frame-Options", cfg.CustomFrameOptions)
	}
	if cfg.ContentTypeNosniff {
		h.Set("X-Content-Type-Options", "nosniff")
	}
	if strings.TrimSpace(cfg.ContentSecurityPolicy) != "" {
		h.Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
	}
	if strings.TrimSpace(cfg.ReferrerPolicy) != "" {
		h.Set("Referrer-Policy", cfg.ReferrerPolicy)
	}
	if strings.TrimSpace(cfg.PermissionsPolicy) != "" {
		h.Set("Permissions-Policy", cfg.PermissionsPolicy)
	}
	if cfg.IENoOpen {
		h.Set("X-Download-Options", "noopen")
	}
	if strings.TrimSpace(cfg.XDNSPrefetchControl) != "" {
		h.Set("X-DNS-Prefetch-Control", cfg.XDNSPrefetchControl)
	}
	if strings.TrimSpace(cfg.CrossOriginOpenerPolicy) != "" {
		h.Set("Cross-Origin-Opener-Policy", cfg.CrossOriginOpenerPolicy)
	}
	if strings.TrimSpace(cfg.CrossOriginResourcePolicy) != "" {
		h.Set("Cross-Origin-Resource-Policy", cfg.CrossOriginResourcePolicy)
	}
	if strings.TrimSpace(cfg.CrossOriginEmbedderPolicy) != "" {
		h.Set("Cross-Origin-Embedder-Policy", cfg.CrossOriginEmbedderPolicy)
	}

	// HSTS should only be sent on secure requests.
	if secureReq && cfg.STSSeconds > 0 {
		stsValue := fmt.Sprintf("max-age=%d", cfg.STSSeconds)
		if cfg.STSIncludeSubdomains {
			stsValue += "; includeSubDomains"
		}
		if cfg.STSPreload {
			stsValue += "; preload"
		}
		h.Set("Strict-Transport-Security", stsValue)
	}

	for key, value := range cfg.CustomHeaders {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		h.Set(key, value)
	}
}

func checkAllowedHosts(req *http.Request, allowedHosts []string) bool {
	if len(allowedHosts) == 0 {
		return true
	}

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	host = stripPort(host)

	for _, allowed := range allowedHosts {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return true
		}
	}
	return false
}

func isSecureRequest(req *http.Request, cfg Config) bool {
	if strings.EqualFold(req.URL.Scheme, "https") || req.TLS != nil {
		return true
	}

	for headerName, expectedValue := range cfg.SSLProxyHeaders {
		values, ok := req.Header[headerName]
		if !ok || len(values) == 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(values[0]), strings.TrimSpace(expectedValue)) {
			return true
		}
	}

	if cfg.DontRedirectIPV4Hostnames && isIPv4(req.Host) {
		return true
	}
	return false
}

func redirectTarget(req *http.Request, cfg Config) string {
	target := *req.URL
	target.Scheme = "https"
	target.Host = req.Host
	if strings.TrimSpace(cfg.SSLHost) != "" {
		target.Host = cfg.SSLHost
	}
	return target.String()
}

func stripPort(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}

func isIPv4(host string) bool {
	ip := net.ParseIP(stripPort(host))
	return ip != nil && ip.To4() != nil
}
