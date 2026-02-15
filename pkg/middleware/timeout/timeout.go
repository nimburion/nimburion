package timeout

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// Mode defines timeout behavior for matching request paths.
type Mode string

const (
	ModeOff Mode = "off"
	ModeOn  Mode = "on"
)

// Config configures request timeout middleware behavior.
type Config struct {
	Enabled              bool
	Default              time.Duration
	ExcludedPathPrefixes []string
	PathPolicies         []PathPolicy
}

// PathPolicy configures timeout mode for a path prefix.
type PathPolicy struct {
	Prefix string
	Mode   Mode
}

// DefaultConfig returns default timeout middleware behavior.
func DefaultConfig() Config {
	return Config{
		Enabled:              false,
		Default:              15 * time.Second,
		ExcludedPathPrefixes: []string{},
		PathPolicies:         []PathPolicy{},
	}
}

// Middleware applies request context deadlines with path-based policies.
func Middleware(cfg Config) router.MiddlewareFunc {
	normalized := normalize(cfg)
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			path := c.Request().URL.Path
			if normalized.modeForPath(path) == ModeOff {
				return next(c)
			}

			reqCtx, cancel := context.WithTimeout(c.Request().Context(), normalized.Default)
			defer cancel()

			c.SetRequest(c.Request().WithContext(reqCtx))
			err := next(c)
			if !isDeadlineExceeded(err, reqCtx.Err()) {
				return err
			}
			if c.Response().Written() {
				return nil
			}
			return c.JSON(504, map[string]string{"error": "request timeout"})
		}
	}
}

func normalize(cfg Config) Config {
	normalized := cfg
	for index := range normalized.PathPolicies {
		normalized.PathPolicies[index].Mode = parseMode(normalized.PathPolicies[index].Mode)
	}
	if normalized.Default <= 0 {
		normalized.Default = DefaultConfig().Default
	}
	return normalized
}

func (cfg Config) modeForPath(path string) Mode {
	if !cfg.Enabled {
		return ModeOff
	}

	for _, prefix := range cfg.ExcludedPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return ModeOff
		}
	}

	bestLen := -1
	bestMode := ModeOn
	for _, policy := range cfg.PathPolicies {
		if strings.TrimSpace(policy.Prefix) == "" {
			continue
		}
		if strings.HasPrefix(path, policy.Prefix) && len(policy.Prefix) > bestLen {
			bestLen = len(policy.Prefix)
			bestMode = policy.Mode
		}
	}
	return bestMode
}

func parseMode(mode Mode) Mode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(ModeOff):
		return ModeOff
	case string(ModeOn):
		return ModeOn
	default:
		return ModeOn
	}
}

func isDeadlineExceeded(err error, reqErr error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(reqErr, context.DeadlineExceeded)
}
