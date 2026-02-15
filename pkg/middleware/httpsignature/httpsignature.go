// Package httpsignature provides request-signing middleware for machine-to-machine APIs.
package httpsignature

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
)

const (
	// AuthKeyIDContextKey is the context key where authenticated key ID is stored.
	AuthKeyIDContextKey = "http_signature_key_id"
)

// KeyProvider resolves shared secrets by key ID.
type KeyProvider interface {
	SecretForKey(ctx context.Context, keyID string) ([]byte, error)
}

// StaticKeyProvider resolves secrets from a static map.
type StaticKeyProvider map[string]string

// SecretForKey returns the configured secret for a key ID.
func (p StaticKeyProvider) SecretForKey(_ context.Context, keyID string) ([]byte, error) {
	secret, ok := p[keyID]
	if !ok || strings.TrimSpace(secret) == "" {
		return nil, errors.New("unknown key id")
	}
	return []byte(secret), nil
}

// NonceStore tracks seen nonces to prevent replay attacks.
type NonceStore interface {
	Use(key string, ttl time.Duration) bool
}

// InMemoryNonceStore stores nonce keys with expiration in memory.
type InMemoryNonceStore struct {
	mu    sync.Mutex
	items map[string]time.Time
}

// NewInMemoryNonceStore creates a new in-memory nonce store.
func NewInMemoryNonceStore() *InMemoryNonceStore {
	return &InMemoryNonceStore{
		items: make(map[string]time.Time),
	}
}

// Use marks a nonce key as used and returns false when already seen and not expired.
func (s *InMemoryNonceStore) Use(key string, ttl time.Duration) bool {
	if strings.TrimSpace(key) == "" {
		return false
	}
	now := time.Now().UTC()
	expiration := now.Add(ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	for itemKey, itemExp := range s.items {
		if now.After(itemExp) {
			delete(s.items, itemKey)
		}
	}
	if currentExp, ok := s.items[key]; ok && now.Before(currentExp) {
		return false
	}
	s.items[key] = expiration
	return true
}

// Config controls HMAC request signature validation.
type Config struct {
	Enabled              bool
	KeyProvider          KeyProvider
	NonceStore           NonceStore
	KeyIDHeader          string
	TimestampHeader      string
	NonceHeader          string
	SignatureHeader      string
	MaxClockSkew         time.Duration
	NonceTTL             time.Duration
	RequireNonce         bool
	ExcludedPathPrefixes []string
}

// DefaultConfig returns explicit middleware defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		KeyIDHeader:      "X-Key-Id",
		TimestampHeader:  "X-Timestamp",
		NonceHeader:      "X-Nonce",
		SignatureHeader:  "X-Signature",
		MaxClockSkew:     5 * time.Minute,
		NonceTTL:         10 * time.Minute,
		RequireNonce:     true,
		NonceStore:       NewInMemoryNonceStore(),
		ExcludedPathPrefixes: []string{
			"/health",
			"/metrics",
			"/ready",
			"/version",
		},
	}
}

// Middleware validates HMAC signatures and rejects invalid requests with 401.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalizeConfig(cfg)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled || c.Request() == nil {
				return next(c)
			}
			if isExcluded(c.Request().URL.Path, cfg.ExcludedPathPrefixes) {
				return next(c)
			}
			if cfg.KeyProvider == nil {
				return unauthorized(c, "missing_key_provider")
			}

			req := c.Request()
			keyID := strings.TrimSpace(req.Header.Get(cfg.KeyIDHeader))
			if keyID == "" {
				return unauthorized(c, "missing_key_id")
			}
			timestampRaw := strings.TrimSpace(req.Header.Get(cfg.TimestampHeader))
			if timestampRaw == "" {
				return unauthorized(c, "missing_timestamp")
			}
			signatureRaw := strings.TrimSpace(req.Header.Get(cfg.SignatureHeader))
			if signatureRaw == "" {
				return unauthorized(c, "missing_signature")
			}
			nonce := strings.TrimSpace(req.Header.Get(cfg.NonceHeader))
			if cfg.RequireNonce && nonce == "" {
				return unauthorized(c, "missing_nonce")
			}

			timestamp, err := parseTimestamp(timestampRaw)
			if err != nil {
				return unauthorized(c, "invalid_timestamp")
			}
			now := time.Now().UTC()
			if skew := now.Sub(timestamp); skew > cfg.MaxClockSkew || skew < -cfg.MaxClockSkew {
				return unauthorized(c, "timestamp_out_of_window")
			}

			secret, err := cfg.KeyProvider.SecretForKey(req.Context(), keyID)
			if err != nil || len(secret) == 0 {
				return unauthorized(c, "unknown_key_id")
			}

			bodyBytes, err := readAndRestoreBody(req)
			if err != nil {
				return unauthorized(c, "invalid_body")
			}

			payload := buildPayload(req, timestampRaw, nonce, bodyBytes)
			expected := computeHMAC(secret, payload)
			received, ok := decodeSignature(signatureRaw)
			if !ok || !hmac.Equal(expected, received) {
				return unauthorized(c, "invalid_signature")
			}

			if cfg.RequireNonce && cfg.NonceStore != nil {
				nonceKey := keyID + ":" + nonce
				if !cfg.NonceStore.Use(nonceKey, cfg.NonceTTL) {
					return unauthorized(c, "replayed_request")
				}
			}

			c.Set(AuthKeyIDContextKey, keyID)
			return next(c)
		}
	}
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if cfg.KeyProvider == nil && cfg.NonceStore == nil && cfg.KeyIDHeader == "" && cfg.TimestampHeader == "" && cfg.NonceHeader == "" && cfg.SignatureHeader == "" && cfg.MaxClockSkew == 0 && cfg.NonceTTL == 0 && len(cfg.ExcludedPathPrefixes) == 0 {
		return def
	}
	if strings.TrimSpace(cfg.KeyIDHeader) == "" {
		cfg.KeyIDHeader = def.KeyIDHeader
	}
	if strings.TrimSpace(cfg.TimestampHeader) == "" {
		cfg.TimestampHeader = def.TimestampHeader
	}
	if strings.TrimSpace(cfg.NonceHeader) == "" {
		cfg.NonceHeader = def.NonceHeader
	}
	if strings.TrimSpace(cfg.SignatureHeader) == "" {
		cfg.SignatureHeader = def.SignatureHeader
	}
	if cfg.MaxClockSkew <= 0 {
		cfg.MaxClockSkew = def.MaxClockSkew
	}
	if cfg.NonceTTL <= 0 {
		cfg.NonceTTL = def.NonceTTL
	}
	if cfg.NonceStore == nil && cfg.RequireNonce {
		cfg.NonceStore = def.NonceStore
	}
	if len(cfg.ExcludedPathPrefixes) == 0 {
		cfg.ExcludedPathPrefixes = def.ExcludedPathPrefixes
	}
	return cfg
}

func isExcluded(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func parseTimestamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, errors.New("empty")
	}
	if unixSec, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unixSec, 0).UTC(), nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, errors.New("invalid format")
}

func readAndRestoreBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func buildPayload(req *http.Request, timestamp string, nonce string, body []byte) string {
	requestURI := req.URL.EscapedPath()
	if query := req.URL.RawQuery; query != "" {
		requestURI += "?" + query
	}
	sum := sha256.Sum256(body)
	return strings.Join([]string{
		strings.ToUpper(req.Method),
		requestURI,
		timestamp,
		nonce,
		hex.EncodeToString(sum[:]),
	}, "\n")
}

func computeHMAC(secret []byte, payload string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func decodeSignature(raw string) ([]byte, bool) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "sha256=")
	if value == "" {
		return nil, false
	}
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, false
	}
	return b, true
}

func unauthorized(c router.Context, reason string) error {
	return c.JSON(http.StatusUnauthorized, map[string]interface{}{
		"error":   "invalid_request_signature",
		"reason":  reason,
		"message": fmt.Sprintf("request rejected: %s", reason),
	})
}
