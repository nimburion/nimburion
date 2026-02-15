package cors

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// Config configures CORS middleware behavior.
//
// Notes:
//   - Only one origin strategy should be used:
//     AllowAllOrigins, AllowOrigins, AllowOriginFunc, AllowOriginWithContextFunc.
//   - If both AllowOriginFunc and AllowOriginWithContextFunc are set,
//     the context-aware function takes precedence.
type Config struct {
	Enabled bool

	AllowAllOrigins            bool
	AllowOrigins               []string
	AllowOriginFunc            func(string) bool
	AllowOriginWithContextFunc func(router.Context, string) bool

	AllowMethods              []string
	AllowPrivateNetwork       bool
	AllowHeaders              []string
	AllowCredentials          bool
	ExposeHeaders             []string
	MaxAge                    time.Duration
	AllowWildcard             bool
	AllowBrowserExtensions    bool
	CustomSchemas             []string
	AllowWebSockets           bool
	AllowFiles                bool
	OptionsResponseStatusCode int
}

// DefaultConfig returns CORS middleware defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:                   false,
		AllowAllOrigins:           false,
		AllowOrigins:              []string{},
		AllowMethods:              []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowPrivateNetwork:       false,
		AllowHeaders:              []string{},
		AllowCredentials:          false,
		ExposeHeaders:             []string{},
		MaxAge:                    12 * time.Hour,
		AllowWildcard:             false,
		AllowBrowserExtensions:    false,
		CustomSchemas:             []string{},
		AllowWebSockets:           false,
		AllowFiles:                false,
		OptionsResponseStatusCode: http.StatusNoContent,
	}
}

// Middleware returns a router middleware implementing CORS.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalize(cfg)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled {
				return next(c)
			}

			req := c.Request()
			res := c.Response()
			origin := req.Header.Get("Origin")
			if origin == "" {
				return next(c)
			}

			if !cfg.isOriginAllowed(c, origin) {
				if isPreflight(req) {
					res.WriteHeader(http.StatusForbidden)
					return nil
				}
				return next(c)
			}

			applyVary(res.Header())
			cfg.setOriginHeaders(res.Header(), origin)

			if len(cfg.ExposeHeaders) > 0 {
				res.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
			}

			if isPreflight(req) {
				res.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
				if len(cfg.AllowHeaders) > 0 {
					res.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
				} else if requested := req.Header.Get("Access-Control-Request-Headers"); requested != "" {
					res.Header().Set("Access-Control-Allow-Headers", requested)
				}
				if cfg.MaxAge > 0 {
					res.Header().Set("Access-Control-Max-Age", formatMaxAge(cfg.MaxAge))
				}
				if cfg.AllowPrivateNetwork && strings.EqualFold(req.Header.Get("Access-Control-Request-Private-Network"), "true") {
					res.Header().Set("Access-Control-Allow-Private-Network", "true")
				}
				res.WriteHeader(cfg.OptionsResponseStatusCode)
				return nil
			}

			return next(c)
		}
	}
}

func normalize(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.AllowOrigins == nil {
		cfg.AllowOrigins = defaults.AllowOrigins
	}
	if cfg.AllowMethods == nil {
		cfg.AllowMethods = defaults.AllowMethods
	}
	if cfg.AllowHeaders == nil {
		cfg.AllowHeaders = defaults.AllowHeaders
	}
	if cfg.ExposeHeaders == nil {
		cfg.ExposeHeaders = defaults.ExposeHeaders
	}
	if cfg.CustomSchemas == nil {
		cfg.CustomSchemas = defaults.CustomSchemas
	}
	if cfg.OptionsResponseStatusCode == 0 {
		cfg.OptionsResponseStatusCode = defaults.OptionsResponseStatusCode
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = defaults.MaxAge
	}

	for i := range cfg.AllowMethods {
		cfg.AllowMethods[i] = strings.ToUpper(strings.TrimSpace(cfg.AllowMethods[i]))
	}
	for i := range cfg.AllowOrigins {
		cfg.AllowOrigins[i] = strings.TrimSpace(cfg.AllowOrigins[i])
	}
	for i := range cfg.AllowHeaders {
		cfg.AllowHeaders[i] = strings.TrimSpace(cfg.AllowHeaders[i])
	}
	for i := range cfg.ExposeHeaders {
		cfg.ExposeHeaders[i] = strings.TrimSpace(cfg.ExposeHeaders[i])
	}
	for i := range cfg.CustomSchemas {
		cfg.CustomSchemas[i] = strings.ToLower(strings.TrimSpace(cfg.CustomSchemas[i]))
	}

	// Per spec: AllowAllOrigins and credentials are mutually exclusive.
	if cfg.AllowAllOrigins {
		cfg.AllowCredentials = false
	}

	return cfg
}

func isPreflight(req *http.Request) bool {
	return req.Method == http.MethodOptions && req.Header.Get("Access-Control-Request-Method") != ""
}

func (cfg Config) isOriginAllowed(c router.Context, origin string) bool {
	if cfg.AllowOriginWithContextFunc != nil {
		return cfg.AllowOriginWithContextFunc(c, origin)
	}
	if cfg.AllowOriginFunc != nil {
		return cfg.AllowOriginFunc(origin)
	}
	if cfg.AllowAllOrigins {
		return cfg.isSchemaAllowed(origin)
	}

	if len(cfg.AllowOrigins) == 0 {
		return false
	}
	if !cfg.isSchemaAllowed(origin) {
		return false
	}

	for _, allowed := range cfg.AllowOrigins {
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, origin) {
			return true
		}
		if cfg.AllowWildcard && wildcardMatch(allowed, origin) {
			return true
		}
	}
	return false
}

// AllowsOrigin evaluates origin policy without request context.
// This is useful for protocols like WebSocket handshakes.
func (cfg Config) AllowsOrigin(origin string) bool {
	cfg = normalize(cfg)
	if cfg.AllowOriginWithContextFunc != nil {
		if cfg.AllowOriginFunc != nil {
			return cfg.AllowOriginFunc(origin)
		}
		// No router context is available in this execution mode.
		return false
	}
	return cfg.isOriginAllowed(nil, origin)
}

func (cfg Config) isSchemaAllowed(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme == "" {
		return false
	}

	allowed := map[string]struct{}{
		"http":  {},
		"https": {},
	}

	if cfg.AllowWebSockets {
		allowed["ws"] = struct{}{}
		allowed["wss"] = struct{}{}
	}
	if cfg.AllowFiles {
		allowed["file"] = struct{}{}
	}
	if cfg.AllowBrowserExtensions {
		allowed["chrome-extension"] = struct{}{}
		allowed["moz-extension"] = struct{}{}
		allowed["safari-web-extension"] = struct{}{}
		allowed["ms-browser-extension"] = struct{}{}
	}
	for _, s := range cfg.CustomSchemas {
		if s == "" {
			continue
		}
		allowed[s] = struct{}{}
	}

	_, ok := allowed[scheme]
	return ok
}

func wildcardMatch(pattern, value string) bool {
	if pattern == "" || !strings.Contains(pattern, "*") {
		return false
	}
	// Per spec, wildcard patterns are valid only with a single "*".
	if strings.Count(pattern, "*") != 1 {
		return false
	}
	parts := strings.Split(pattern, "*")
	prefix := parts[0]
	suffix := parts[1]

	return strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix)
}

func (cfg Config) setOriginHeaders(h http.Header, origin string) {
	if cfg.AllowCredentials {
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		return
	}
	if cfg.AllowAllOrigins || cfg.isAllowAllOriginsList() {
		h.Set("Access-Control-Allow-Origin", "*")
		return
	}
	h.Set("Access-Control-Allow-Origin", origin)
}

func (cfg Config) isAllowAllOriginsList() bool {
	for _, allowed := range cfg.AllowOrigins {
		if strings.TrimSpace(allowed) == "*" {
			return true
		}
	}
	return false
}

func applyVary(h http.Header) {
	appendVary(h, "Origin")
	appendVary(h, "Access-Control-Request-Method")
	appendVary(h, "Access-Control-Request-Headers")
}

func appendVary(h http.Header, value string) {
	current := h.Get("Vary")
	if current == "" {
		h.Set("Vary", value)
		return
	}

	parts := strings.Split(current, ",")
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}

	h.Set("Vary", current+", "+value)
}

func formatMaxAge(duration time.Duration) string {
	seconds := int(duration / time.Second)
	if seconds < 0 {
		return "0"
	}
	return strconv.Itoa(seconds)
}
