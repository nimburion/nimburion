package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
)

const (
	// ContextKey stores the active request session.
	ContextKey = "session"
)

var (
	// ErrNotFound indicates that a session does not exist in the backend store.
	ErrNotFound = errors.New("session not found")
)

// Store defines a pluggable session backend.
type Store interface {
	Load(ctx context.Context, id string) (map[string]string, error)
	Save(ctx context.Context, id string, data map[string]string, ttl time.Duration) error
	Delete(ctx context.Context, id string) error
	Touch(ctx context.Context, id string, ttl time.Duration) error
	Close() error
}

// Config controls session middleware behavior.
type Config struct {
	Enabled        bool
	Store          Store
	CookieName     string
	CookiePath     string
	CookieDomain   string
	CookieSecure   bool
	CookieHTTPOnly bool
	CookieSameSite string // lax, strict, none
	TTL            time.Duration
	IdleTimeout    time.Duration
	AutoCreate     bool
}

// Session is the per-request mutable session view.
type Session struct {
	id        string
	data      map[string]string
	dirty     bool
	destroyed bool
	renewed   bool
}

// DefaultConfig returns safe defaults for server-side sessions.
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		Store:          nil,
		CookieName:     "sid",
		CookiePath:     "/",
		CookieDomain:   "",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: "lax",
		TTL:            12 * time.Hour,
		IdleTimeout:    30 * time.Minute,
		AutoCreate:     true,
	}
}

// Middleware loads/persists request sessions in a pluggable backend store.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalizeConfig(cfg)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled || cfg.Store == nil {
				return next(c)
			}

			sessionID := readSessionID(c.Request(), cfg.CookieName)
			s := &Session{
				id:   sessionID,
				data: map[string]string{},
			}

			if sessionID != "" {
				storedData, err := cfg.Store.Load(c.Request().Context(), sessionID)
				if err == nil {
					s.data = cloneMap(storedData)
				} else if !errors.Is(err, ErrNotFound) {
					return err
				}
			}

			if s.id == "" && cfg.AutoCreate {
				newID, err := generateSessionID()
				if err != nil {
					return err
				}
				s.id = newID
				s.dirty = true
			}

			c.Set(ContextKey, s)

			if s.id != "" && cfg.IdleTimeout > 0 {
				_ = cfg.Store.Touch(c.Request().Context(), s.id, cfg.IdleTimeout)
			}
			if s.id != "" {
				writeSessionCookie(c.Response(), cfg, s.id)
			}

			handlerErr := next(c)
			commitErr := commit(c, cfg, sessionID, s)
			if handlerErr != nil {
				return handlerErr
			}
			return commitErr
		}
	}
}

// FromContext returns the active request session if present.
func FromContext(c router.Context) (*Session, bool) {
	if c == nil {
		return nil, false
	}
	v := c.Get(ContextKey)
	if v == nil {
		return nil, false
	}
	s, ok := v.(*Session)
	return s, ok
}

// ID returns the current session identifier.
func (s *Session) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// Get reads a session value.
func (s *Session) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	val, ok := s.data[key]
	return val, ok
}

// Set writes a session value.
func (s *Session) Set(key, value string) {
	if s == nil || strings.TrimSpace(key) == "" {
		return
	}
	s.data[key] = value
	s.dirty = true
}

// Delete removes a session value.
func (s *Session) Delete(key string) {
	if s == nil {
		return
	}
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.dirty = true
	}
}

// Values returns a copy of current session values.
func (s *Session) Values() map[string]string {
	if s == nil {
		return map[string]string{}
	}
	return cloneMap(s.data)
}

// Renew rotates the session id at the end of the current request.
func (s *Session) Renew() {
	if s == nil {
		return
	}
	s.renewed = true
	s.dirty = true
}

// Destroy marks the session for deletion and cookie removal.
func (s *Session) Destroy() {
	if s == nil {
		return
	}
	s.destroyed = true
	s.dirty = true
	s.data = map[string]string{}
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if strings.TrimSpace(cfg.CookieName) == "" {
		cfg.CookieName = def.CookieName
	}
	if strings.TrimSpace(cfg.CookiePath) == "" {
		cfg.CookiePath = def.CookiePath
	}
	if strings.TrimSpace(cfg.CookieSameSite) == "" {
		cfg.CookieSameSite = def.CookieSameSite
	}
	if cfg.TTL <= 0 {
		cfg.TTL = def.TTL
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = def.IdleTimeout
	}
	if !cfg.AutoCreate {
		cfg.AutoCreate = def.AutoCreate
	}
	return cfg
}

func commit(c router.Context, cfg Config, previousID string, s *Session) error {
	if s == nil || s.id == "" {
		return nil
	}

	if s.destroyed {
		if previousID != "" {
			_ = cfg.Store.Delete(c.Request().Context(), previousID)
		}
		if !c.Response().Written() {
			clearSessionCookie(c.Response(), cfg)
		}
		return nil
	}

	targetID := s.id
	if s.renewed {
		newID, err := generateSessionID()
		if err != nil {
			return err
		}
		targetID = newID
	}

	if s.dirty || s.renewed {
		if err := cfg.Store.Save(c.Request().Context(), targetID, cloneMap(s.data), cfg.TTL); err != nil {
			return err
		}
	}

	if s.renewed && previousID != "" && previousID != targetID {
		_ = cfg.Store.Delete(c.Request().Context(), previousID)
	}

	if !c.Response().Written() {
		writeSessionCookie(c.Response(), cfg, targetID)
	}
	s.id = targetID
	return nil
}

func readSessionID(req *http.Request, cookieName string) string {
	if req == nil {
		return ""
	}
	cookie, err := req.Cookie(cookieName)
	if err != nil || cookie == nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func writeSessionCookie(w http.ResponseWriter, cfg Config, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    sessionID,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		Secure:   cfg.CookieSecure,
		HttpOnly: cfg.CookieHTTPOnly,
		SameSite: parseSameSite(cfg.CookieSameSite),
		MaxAge:   int(cfg.TTL.Seconds()),
		Expires:  time.Now().Add(cfg.TTL),
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    "",
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		Secure:   cfg.CookieSecure,
		HttpOnly: cfg.CookieHTTPOnly,
		SameSite: parseSameSite(cfg.CookieSameSite),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
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

func generateSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
