package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/middleware/authz"
	"github.com/nimburion/nimburion/pkg/server/router"
)

var (
	// ErrCacheMiss indicates that a cache key was not found.
	ErrCacheMiss = errors.New("cache key not found")
)

// Store defines a pluggable backend for HTTP response cache.
type Store interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(key string) error
	Close() error
}

// KeySource defines where a cache key fragment is extracted from.
type KeySource string

const (
	KeySourceStatic          KeySource = "static"
	KeySourceRoute           KeySource = "route"
	KeySourceHeader          KeySource = "header"
	KeySourceQuery           KeySource = "query"
	KeySourceClaim           KeySource = "claim"
	KeySourceTenant          KeySource = "tenant"
	KeySourceSubject         KeySource = "subject"
	KeySourceScopeHash       KeySource = "scope_hash"
	KeySourceAuthFingerprint KeySource = "auth_fingerprint"
)

// CacheKeyRule defines one dynamic fragment for cache key composition.
type CacheKeyRule struct {
	Source   KeySource
	Key      string
	Value    string
	Name     string
	Optional bool
}

// Config controls middleware behavior.
type Config struct {
	Enabled bool
	Store   Store

	TTL                  time.Duration
	StaleWhileRevalidate time.Duration
	Public               bool

	// VaryHeaders is appended to Vary response header on cache hits.
	VaryHeaders []string

	// BypassQueryParam allows explicit bypass (example: ?__cache_bypass=1).
	BypassQueryParam string

	KeyPrefix string
	KeyRules  []CacheKeyRule

	// Sensitive triggers stronger key validation at startup.
	Sensitive bool
	// RequireTenant enforces tenant dimension in key rules.
	RequireTenant bool
	// RequireAuthFingerprint enforces auth dimension in key rules.
	RequireAuthFingerprint bool
}

// DefaultConfig returns safe defaults with caching disabled.
func DefaultConfig() Config {
	return Config{
		Enabled:                false,
		TTL:                    30 * time.Second,
		StaleWhileRevalidate:   0,
		Public:                 true,
		VaryHeaders:            []string{},
		BypassQueryParam:       "__cache_bypass",
		KeyPrefix:              "http-cache",
		KeyRules:               []CacheKeyRule{},
		Sensitive:              false,
		RequireTenant:          false,
		RequireAuthFingerprint: false,
	}
}

// New validates config and returns a middleware instance.
func New(cfg Config) (router.MiddlewareFunc, error) {
	cfg = normalizeConfig(cfg)
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	m := &middleware{
		cfg:   cfg,
		group: newRequestGroup(),
	}
	return m.handle, nil
}

// Middleware creates cache middleware. Invalid configs degrade to pass-through.
func Middleware(cfg Config) router.MiddlewareFunc {
	mw, err := New(cfg)
	if err != nil {
		return passthroughMiddleware()
	}
	return mw
}

// ValidateConfig checks startup invariants, especially on sensitive routes.
func ValidateConfig(cfg Config) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Store == nil {
		return errors.New("cache store is required when cache middleware is enabled")
	}
	if cfg.TTL <= 0 {
		return errors.New("cache ttl must be greater than zero")
	}
	if cfg.StaleWhileRevalidate < 0 {
		return errors.New("cache stale_while_revalidate cannot be negative")
	}

	validSources := map[KeySource]struct{}{
		KeySourceStatic:          {},
		KeySourceRoute:           {},
		KeySourceHeader:          {},
		KeySourceQuery:           {},
		KeySourceClaim:           {},
		KeySourceTenant:          {},
		KeySourceSubject:         {},
		KeySourceScopeHash:       {},
		KeySourceAuthFingerprint: {},
	}

	hasTenant := false
	hasAuthDim := false
	for idx, rule := range cfg.KeyRules {
		if _, ok := validSources[rule.Source]; !ok {
			return fmt.Errorf("cache key_rules[%d].source is invalid: %s", idx, rule.Source)
		}
		if rule.Source == KeySourceStatic && strings.TrimSpace(rule.Value) == "" {
			return fmt.Errorf("cache key_rules[%d].value is required for source=static", idx)
		}
		if (rule.Source == KeySourceRoute || rule.Source == KeySourceHeader || rule.Source == KeySourceQuery || rule.Source == KeySourceClaim) && strings.TrimSpace(rule.Key) == "" {
			return fmt.Errorf("cache key_rules[%d].key is required for source=%s", idx, rule.Source)
		}
		if rule.Source == KeySourceTenant {
			hasTenant = true
		}
		if rule.Source == KeySourceSubject || rule.Source == KeySourceScopeHash || rule.Source == KeySourceAuthFingerprint || rule.Source == KeySourceClaim {
			hasAuthDim = true
		}
	}

	if cfg.Sensitive || cfg.RequireTenant {
		if !hasTenant {
			return errors.New("cache sensitive route requires tenant key rule")
		}
	}
	if cfg.Sensitive || cfg.RequireAuthFingerprint {
		if !hasAuthDim {
			return errors.New("cache sensitive route requires auth fingerprint key rule")
		}
	}
	return nil
}

type middleware struct {
	cfg   Config
	group *requestGroup
}

func (m *middleware) handle(next router.HandlerFunc) router.HandlerFunc {
	return func(c router.Context) error {
		if !m.cfg.Enabled || m.cfg.Store == nil {
			return next(c)
		}

		method := strings.ToUpper(strings.TrimSpace(c.Request().Method))
		if method != http.MethodGet && method != http.MethodHead {
			return next(c)
		}

		start := time.Now()
		defer observeCacheLatency("total", time.Since(start))

		if shouldBypass(c.Request(), m.cfg) {
			incCacheResult("bypass")
			c.Response().Header().Set("X-Cache", "BYPASS")
			return next(c)
		}

		cacheKey, err := buildCacheKey(c, m.cfg)
		if err != nil {
			incCacheResult("error")
			c.Response().Header().Set("X-Cache", "MISS")
			return next(c)
		}

		now := time.Now()
		entry, cached, entryState := m.load(cacheKey, now)
		if cached {
			switch entryState {
			case "fresh":
				incCacheResult("hit")
				return serveCached(c, entry, m.cfg, "HIT")
			case "stale":
				if m.cfg.StaleWhileRevalidate > 0 && m.group.InFlight(cacheKey) {
					incCacheResult("stale")
					return serveCached(c, entry, m.cfg, "STALE")
				}
			}
		}

		// Stampede protection: one request computes and stores value for a key,
		// concurrent requests wait and receive the same cached payload.
		sharedEntry, sharedErr, shared := m.group.Do(cacheKey, func() (*cacheEntry, error) {
			c.Response().Header().Set("X-Cache", "MISS")
			incCacheResult("miss")
			return m.computeAndStore(c, next, cacheKey)
		})
		if shared {
			if sharedErr == nil && sharedEntry != nil {
				incCacheResult("hit_shared")
				return serveCached(c, sharedEntry, m.cfg, "HIT")
			}
			// If leader did not generate cacheable response, execute handler normally.
			c.Response().Header().Set("X-Cache", "MISS")
			return next(c)
		}
		return sharedErr
	}
}

func (m *middleware) load(key string, now time.Time) (*cacheEntry, bool, string) {
	lookupStart := time.Now()
	defer observeCacheLatency("lookup", time.Since(lookupStart))

	raw, err := m.cfg.Store.Get(key)
	if err != nil {
		if errors.Is(err, ErrCacheMiss) {
			return nil, false, ""
		}
		incCacheResult("error")
		return nil, false, ""
	}

	entry := &cacheEntry{}
	if err := json.Unmarshal(raw, entry); err != nil {
		incCacheResult("error")
		return nil, false, ""
	}

	if now.Before(entry.FreshUntil) || now.Equal(entry.FreshUntil) {
		return entry, true, "fresh"
	}
	if m.cfg.StaleWhileRevalidate > 0 && now.Before(entry.StaleUntil) {
		return entry, true, "stale"
	}
	return nil, false, ""
}

func (m *middleware) computeAndStore(c router.Context, next router.HandlerFunc, key string) (*cacheEntry, error) {
	base := c.Response()
	writer := newCaptureResponseWriter(base)
	c.SetResponse(writer)
	defer c.SetResponse(base)

	if err := next(c); err != nil {
		return nil, err
	}

	status := writer.Status()
	if status != http.StatusOK {
		return nil, nil
	}

	body := writer.Body()
	filteredHeaders := filterHeaders(base.Header())
	etag := strings.TrimSpace(filteredHeaders.Get("ETag"))
	if etag == "" {
		etag = computeETag(body)
		filteredHeaders.Set("ETag", etag)
	}
	applyVary(filteredHeaders, m.cfg.VaryHeaders)
	if strings.TrimSpace(filteredHeaders.Get("Cache-Control")) == "" {
		filteredHeaders.Set("Cache-Control", buildCacheControl(m.cfg))
	}

	entry := &cacheEntry{
		StatusCode: status,
		Headers:    cloneHeader(filteredHeaders),
		Body:       body,
		ETag:       etag,
		FreshUntil: time.Now().Add(m.cfg.TTL),
		StaleUntil: time.Now().Add(m.cfg.TTL + m.cfg.StaleWhileRevalidate),
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		incCacheResult("error")
		return nil, nil
	}
	storeStart := time.Now()
	ttl := m.cfg.TTL + m.cfg.StaleWhileRevalidate
	if ttl <= 0 {
		ttl = m.cfg.TTL
	}
	if err := m.cfg.Store.Set(key, encoded, ttl); err != nil {
		incCacheResult("error")
		return nil, nil
	}
	observeCacheLatency("store", time.Since(storeStart))
	incCacheResult("set")
	return entry, nil
}

type cacheEntry struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	ETag       string              `json:"etag"`
	FreshUntil time.Time           `json:"fresh_until"`
	StaleUntil time.Time           `json:"stale_until"`
}

func serveCached(c router.Context, entry *cacheEntry, cfg Config, state string) error {
	if entry == nil {
		return nil
	}

	h := c.Response().Header()
	for k, values := range entry.Headers {
		copied := append([]string{}, values...)
		h[k] = copied
	}
	if strings.TrimSpace(h.Get("Cache-Control")) == "" {
		h.Set("Cache-Control", buildCacheControl(cfg))
	}
	applyVary(h, cfg.VaryHeaders)
	if strings.TrimSpace(entry.ETag) != "" {
		h.Set("ETag", entry.ETag)
	}
	h.Set("X-Cache", state)

	if ifNoneMatchSatisfied(c.Request(), entry.ETag) {
		c.Response().WriteHeader(http.StatusNotModified)
		return nil
	}

	c.Response().WriteHeader(entry.StatusCode)
	if strings.EqualFold(c.Request().Method, http.MethodHead) {
		return nil
	}
	_, err := c.Response().Write(entry.Body)
	return err
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if cfg.TTL <= 0 {
		cfg.TTL = def.TTL
	}
	if cfg.StaleWhileRevalidate < 0 {
		cfg.StaleWhileRevalidate = def.StaleWhileRevalidate
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		cfg.KeyPrefix = def.KeyPrefix
	}
	if strings.TrimSpace(cfg.BypassQueryParam) == "" {
		cfg.BypassQueryParam = def.BypassQueryParam
	}
	if cfg.VaryHeaders == nil {
		cfg.VaryHeaders = def.VaryHeaders
	}
	if cfg.KeyRules == nil {
		cfg.KeyRules = def.KeyRules
	}
	return cfg
}

func passthroughMiddleware() router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			return next(c)
		}
	}
}

func buildCacheKey(c router.Context, cfg Config) (string, error) {
	req := c.Request()
	if req == nil {
		return "", errors.New("request is nil")
	}

	if len(cfg.KeyRules) == 0 {
		return fmt.Sprintf("%s:%s:%s:%s", cfg.KeyPrefix, req.Method, req.URL.Path, canonicalQuery(req.URL.Query())), nil
	}

	parts := []string{
		cfg.KeyPrefix,
		"method=" + strings.ToUpper(req.Method),
		"path=" + req.URL.Path,
	}

	for _, rule := range cfg.KeyRules {
		value, ok := resolveRuleValue(c, rule)
		if !ok {
			if rule.Optional {
				continue
			}
			return "", fmt.Errorf("missing required cache key rule for source=%s key=%s", rule.Source, rule.Key)
		}
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			name = defaultRuleName(rule)
		}
		parts = append(parts, name+"="+url.QueryEscape(value))
	}

	return strings.Join(parts, ":"), nil
}

func resolveRuleValue(c router.Context, rule CacheKeyRule) (string, bool) {
	switch rule.Source {
	case KeySourceStatic:
		value := strings.TrimSpace(rule.Value)
		return value, value != ""
	case KeySourceRoute:
		value := strings.TrimSpace(c.Param(rule.Key))
		return value, value != ""
	case KeySourceHeader:
		value := strings.TrimSpace(c.Request().Header.Get(rule.Key))
		return value, value != ""
	case KeySourceQuery:
		value := strings.TrimSpace(c.Query(rule.Key))
		return value, value != ""
	case KeySourceClaim:
		claims, ok := claimsFromContext(c)
		if !ok {
			return "", false
		}
		value := strings.TrimSpace(claimValue(claims, rule.Key))
		return value, value != ""
	case KeySourceTenant:
		claims, ok := claimsFromContext(c)
		if !ok {
			return "", false
		}
		value := strings.TrimSpace(claims.TenantID)
		return value, value != ""
	case KeySourceSubject:
		claims, ok := claimsFromContext(c)
		if !ok {
			return "", false
		}
		value := strings.TrimSpace(claims.Subject)
		return value, value != ""
	case KeySourceScopeHash:
		claims, ok := claimsFromContext(c)
		if !ok {
			return "", false
		}
		value := scopeHash(claims.Scopes)
		return value, value != ""
	case KeySourceAuthFingerprint:
		claims, ok := claimsFromContext(c)
		if !ok {
			return "", false
		}
		value := authFingerprint(claims)
		return value, value != ""
	default:
		return "", false
	}
}

func defaultRuleName(rule CacheKeyRule) string {
	switch rule.Source {
	case KeySourceRoute:
		return "route." + rule.Key
	case KeySourceHeader:
		return "header." + strings.ToLower(rule.Key)
	case KeySourceQuery:
		return "query." + rule.Key
	case KeySourceClaim:
		return "claim." + rule.Key
	default:
		return string(rule.Source)
	}
}

func claimValue(claims *auth.Claims, key string) string {
	if claims == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "sub", "subject":
		return claims.Subject
	case "tenant", "tenant_id", "tenantid":
		return claims.TenantID
	case "iss", "issuer":
		return claims.Issuer
	case "scope", "scopes":
		return strings.Join(claims.Scopes, " ")
	case "roles", "role":
		return strings.Join(claims.Roles, ",")
	default:
		if claims.Custom == nil {
			return ""
		}
		if v, ok := claims.Custom[key]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func claimsFromContext(c router.Context) (*auth.Claims, bool) {
	raw := c.Get(authz.ClaimsKey)
	if raw == nil {
		return nil, false
	}
	claims, ok := raw.(*auth.Claims)
	return claims, ok && claims != nil
}

func scopeHash(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	cp := append([]string{}, scopes...)
	sort.Strings(cp)
	sum := sha256.Sum256([]byte(strings.Join(cp, ",")))
	return hex.EncodeToString(sum[:8])
}

func authFingerprint(claims *auth.Claims) string {
	if claims == nil {
		return ""
	}
	scopes := scopeHash(claims.Scopes)
	raw := strings.Join([]string{
		strings.TrimSpace(claims.Subject),
		strings.TrimSpace(claims.TenantID),
		scopes,
	}, "|")
	raw = strings.Trim(raw, "|")
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

func canonicalQuery(values url.Values) string {
	if values == nil {
		return ""
	}
	return values.Encode()
}

func shouldBypass(r *http.Request, cfg Config) bool {
	if r == nil {
		return false
	}
	cc := strings.ToLower(strings.TrimSpace(r.Header.Get("Cache-Control")))
	if strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		return true
	}
	param := strings.TrimSpace(cfg.BypassQueryParam)
	if param == "" {
		return false
	}
	val := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(param)))
	if val == "" {
		return false
	}
	if val == "0" || val == "false" || val == "off" {
		return false
	}
	return true
}

func computeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `W/"` + hex.EncodeToString(sum[:]) + `"`
}

func ifNoneMatchSatisfied(r *http.Request, etag string) bool {
	if r == nil || strings.TrimSpace(etag) == "" {
		return false
	}
	header := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	for _, token := range strings.Split(header, ",") {
		if strings.TrimSpace(token) == etag {
			return true
		}
	}
	return false
}

func applyVary(h http.Header, vary []string) {
	if len(vary) == 0 {
		return
	}
	existing := parseCSVHeader(h.Get("Vary"))
	seen := make(map[string]struct{}, len(existing)+len(vary))
	merged := make([]string, 0, len(existing)+len(vary))
	for _, item := range existing {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range vary {
		trimmed := strings.TrimSpace(item)
		normalized := strings.ToLower(trimmed)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		merged = append(merged, trimmed)
	}
	if len(merged) > 0 {
		h.Set("Vary", strings.Join(merged, ", "))
	}
}

func parseCSVHeader(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func buildCacheControl(cfg Config) string {
	scope := "private"
	if cfg.Public {
		scope = "public"
	}
	maxAge := int(cfg.TTL.Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	parts := []string{
		scope,
		"max-age=" + strconv.Itoa(maxAge),
	}
	if cfg.StaleWhileRevalidate > 0 {
		parts = append(parts, "stale-while-revalidate="+strconv.Itoa(int(cfg.StaleWhileRevalidate.Seconds())))
	}
	return strings.Join(parts, ", ")
}

func filterHeaders(h http.Header) http.Header {
	cloned := cloneHeader(h)
	drop := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
		"Set-Cookie",
		"Date",
		"Content-Length",
	}
	for _, key := range drop {
		cloned.Del(key)
	}
	return cloned
}

func cloneHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, values := range h {
		out[k] = append([]string{}, values...)
	}
	return out
}

type captureResponseWriter struct {
	base        router.ResponseWriter
	statusCode  int
	headerWrote bool
	body        []byte
	bodyMu      sync.Mutex
}

func newCaptureResponseWriter(base router.ResponseWriter) *captureResponseWriter {
	return &captureResponseWriter{base: base}
}

func (w *captureResponseWriter) Header() http.Header {
	return w.base.Header()
}

func (w *captureResponseWriter) WriteHeader(code int) {
	if w.headerWrote {
		return
	}
	w.headerWrote = true
	w.statusCode = code
	w.base.WriteHeader(code)
}

func (w *captureResponseWriter) Write(p []byte) (int, error) {
	if !w.headerWrote {
		w.WriteHeader(http.StatusOK)
	}
	w.bodyMu.Lock()
	w.body = append(w.body, p...)
	w.bodyMu.Unlock()
	return w.base.Write(p)
}

func (w *captureResponseWriter) Status() int {
	if w.statusCode == 0 {
		return w.base.Status()
	}
	return w.statusCode
}

func (w *captureResponseWriter) Written() bool {
	return w.base.Written() || w.headerWrote
}

func (w *captureResponseWriter) Body() []byte {
	w.bodyMu.Lock()
	defer w.bodyMu.Unlock()
	return append([]byte{}, w.body...)
}

type requestGroup struct {
	mu sync.Mutex
	m  map[string]*requestCall
}

type requestCall struct {
	wg    sync.WaitGroup
	entry *cacheEntry
	err   error
}

func newRequestGroup() *requestGroup {
	return &requestGroup{
		m: make(map[string]*requestCall),
	}
}

func (g *requestGroup) InFlight(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.m[key]
	return ok
}

func (g *requestGroup) Do(key string, fn func() (*cacheEntry, error)) (*cacheEntry, error, bool) {
	g.mu.Lock()
	if call, ok := g.m[key]; ok {
		g.mu.Unlock()
		call.wg.Wait()
		return call.entry, call.err, true
	}
	call := &requestCall{}
	call.wg.Add(1)
	g.m[key] = call
	g.mu.Unlock()

	call.entry, call.err = fn()
	call.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return call.entry, call.err, false
}
