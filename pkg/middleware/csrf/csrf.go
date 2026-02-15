package csrf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/middleware/session"
	"github.com/nimburion/nimburion/pkg/server/router"
)

const (
	// SessionTokenKey stores the CSRF token in the server-side session.
	SessionTokenKey = "csrf_token"
)

// Config controls CSRF middleware behavior.
type Config struct {
	Enabled        bool
	HeaderName     string
	CookieName     string
	CookiePath     string
	CookieDomain   string
	CookieSecure   bool
	CookieSameSite string // lax, strict, none
	CookieTTL      time.Duration
	ExemptMethods  []string
	ExemptPaths    []string
}

// DefaultConfig returns explicit CSRF defaults for session-cookie auth.
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		HeaderName:     "X-CSRF-Token",
		CookieName:     "XSRF-TOKEN",
		CookiePath:     "/",
		CookieDomain:   "",
		CookieSecure:   true,
		CookieSameSite: "lax",
		CookieTTL:      12 * time.Hour,
		ExemptMethods:  []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace},
		ExemptPaths:    []string{},
	}
}

// Middleware validates CSRF tokens using session-backed double-submit strategy.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalizeConfig(cfg)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled {
				return next(c)
			}

			s, ok := session.FromContext(c)
			if !ok || s == nil {
				return errors.New("csrf middleware requires session middleware")
			}

			token, exists := s.Get(SessionTokenKey)
			if !exists || strings.TrimSpace(token) == "" {
				generated, err := generateToken()
				if err != nil {
					return err
				}
				token = generated
				s.Set(SessionTokenKey, token)
			}

			writeCSRFCookie(c.Response(), cfg, token)

			if isExempt(c.Request(), cfg) {
				return next(c)
			}

			sent := strings.TrimSpace(c.Request().Header.Get(cfg.HeaderName))
			if sent == "" {
				sent = strings.TrimSpace(c.Request().Header.Get("X-XSRF-Token"))
			}
			if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "invalid csrf token",
				})
			}

			return next(c)
		}
	}
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if strings.TrimSpace(cfg.HeaderName) == "" {
		cfg.HeaderName = def.HeaderName
	}
	if strings.TrimSpace(cfg.CookieName) == "" {
		cfg.CookieName = def.CookieName
	}
	if strings.TrimSpace(cfg.CookiePath) == "" {
		cfg.CookiePath = def.CookiePath
	}
	if strings.TrimSpace(cfg.CookieSameSite) == "" {
		cfg.CookieSameSite = def.CookieSameSite
	}
	if cfg.CookieTTL <= 0 {
		cfg.CookieTTL = def.CookieTTL
	}
	if len(cfg.ExemptMethods) == 0 {
		cfg.ExemptMethods = def.ExemptMethods
	}
	return cfg
}

func isExempt(req *http.Request, cfg Config) bool {
	if req == nil {
		return true
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	for _, exemptMethod := range cfg.ExemptMethods {
		if method == strings.ToUpper(strings.TrimSpace(exemptMethod)) {
			return true
		}
	}
	path := req.URL.Path
	for _, exemptPath := range cfg.ExemptPaths {
		prefix := strings.TrimSpace(exemptPath)
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeCSRFCookie(w http.ResponseWriter, cfg Config, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    token,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		Secure:   cfg.CookieSecure,
		HttpOnly: false,
		SameSite: parseSameSite(cfg.CookieSameSite),
		MaxAge:   int(cfg.CookieTTL.Seconds()),
		Expires:  time.Now().Add(cfg.CookieTTL),
	})
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
