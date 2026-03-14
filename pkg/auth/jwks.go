package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// JWKSClient fetches and caches JSON Web Key Sets for JWT signature verification.
// It implements thread-safe caching with configurable TTL to reduce external calls.
type JWKSClient struct {
	jwksURL           string
	cache             *jwksCache
	httpClient        *http.Client
	logger            logger.Logger
	allowPrivateHosts bool
	resolver          hostResolver
}

type hostResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

// JWKSClientOption configures optional JWKSClient behavior.
type JWKSClientOption func(*JWKSClient)

// WithAllowPrivateHosts allows private and loopback hosts for JWKS URLs.
func WithAllowPrivateHosts(allow bool) JWKSClientOption {
	return func(c *JWKSClient) {
		c.allowPrivateHosts = allow
	}
}

// jwksCache provides thread-safe caching of JWKS keys with TTL support.
type jwksCache struct {
	keys      map[string]interface{} // kid -> public key
	expiresAt time.Time
	mu        sync.RWMutex
	ttl       time.Duration
}

// JWK represents a JSON Web Key from the JWKS endpoint.
type JWK struct {
	Kid string `json:"kid"` // Key ID
	Kty string `json:"kty"` // Key Type (e.g., "RSA")
	Use string `json:"use"` // Public Key Use (e.g., "sig")
	Alg string `json:"alg"` // Algorithm (e.g., "RS256")
	N   string `json:"n"`   // RSA modulus
	E   string `json:"e"`   // RSA exponent
}

// JWKSResponse represents the response from a JWKS endpoint.
type JWKSResponse struct {
	Keys []JWK `json:"keys"`
}

// NewJWKSClient creates a new JWKS client with the specified configuration.
// The client will fetch keys from jwksURL and cache them for cacheTTL duration.
func NewJWKSClient(jwksURL string, cacheTTL time.Duration, logger logger.Logger, opts ...JWKSClientOption) *JWKSClient {
	client := &JWKSClient{
		jwksURL: jwksURL,
		cache: &jwksCache{
			keys: make(map[string]interface{}),
			ttl:  cacheTTL,
		},
		logger:   logger,
		resolver: net.DefaultResolver,
	}
	client.httpClient = newJWKSHTTPClient(client)
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

// GetKey retrieves a public key by its key ID (kid).
// It first checks the cache, and if not found or expired, fetches fresh keys from the JWKS endpoint.
func (c *JWKSClient) GetKey(ctx context.Context, kid string) (interface{}, error) {
	// Check cache first
	if key := c.cache.get(kid); key != nil {
		c.logger.Debug("JWKS key found in cache", "kid", kid)
		return key, nil
	}

	c.logger.Debug("JWKS key not in cache, fetching from endpoint", "kid", kid)

	// Fetch JWKS
	if err := c.refreshJWKS(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh JWKS: %w", err)
	}

	// Try cache again
	key := c.cache.get(kid)
	if key == nil {
		return nil, fmt.Errorf("key not found: %s", kid)
	}

	return key, nil
}

// refreshJWKS fetches the JWKS from the configured URL and updates the cache.
func (c *JWKSClient) refreshJWKS(ctx context.Context) error {
	if err := validateHTTPURLWithOptions(c.jwksURL, c.allowPrivateHosts); err != nil {
		return fmt.Errorf("invalid jwks url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.logger.Info("fetching JWKS", "url", c.jwksURL)

	// #nosec G704 -- jwksURL is validated as an absolute HTTP(S) URL before the request is sent.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			ignoreCloseError(closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwksResp JWKSResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwksResp); err != nil {
		return fmt.Errorf("failed to decode JWKS response: %w", err)
	}

	keys := make(map[string]interface{})
	for _, jwk := range jwksResp.Keys {
		key, err := parseJWK(jwk)
		if err != nil {
			c.logger.Warn("failed to parse JWK", "kid", jwk.Kid, "error", err)
			continue
		}
		keys[jwk.Kid] = key
		c.logger.Debug("parsed JWK", "kid", jwk.Kid, "kty", jwk.Kty, "alg", jwk.Alg)
	}

	if len(keys) == 0 {
		return fmt.Errorf("no valid keys found in JWKS response")
	}

	c.cache.set(keys)
	c.logger.Info("JWKS cache updated", "key_count", len(keys))

	return nil
}

func newJWKSHTTPClient(client *JWKSClient) *http.Client {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok || baseTransport == nil {
		baseTransport = &http.Transport{}
	}
	transport := baseTransport.Clone()
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if client.allowPrivateHosts {
			return dialer.DialContext(ctx, network, address)
		}

		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		allowedAddrs, err := client.resolveAllowedIPs(ctx, host)
		if err != nil {
			return nil, err
		}

		var dialErrs []error
		for _, addr := range allowedAddrs {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			dialErrs = append(dialErrs, err)
		}
		return nil, errors.Join(dialErrs...)
	}

	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}
}

func (c *JWKSClient) resolveAllowedIPs(ctx context.Context, host string) ([]net.IPAddr, error) {
	normalizedHost := normalizeHostname(host)
	if isLocalHostname(normalizedHost) {
		return nil, fmt.Errorf("url host %q is not allowed: private or loopback addresses are forbidden", normalizedHost)
	}
	if ip := net.ParseIP(normalizedHost); ip != nil {
		if isDisallowedIP(ip) {
			return nil, fmt.Errorf("url host %q is not allowed: private or loopback addresses are forbidden", normalizedHost)
		}
		return []net.IPAddr{{IP: ip}}, nil
	}

	lookupCtx, cancel := withLookupTimeout(ctx)
	defer cancel()

	addrs, err := c.resolver.LookupIPAddr(lookupCtx, normalizedHost)
	if err != nil {
		return nil, fmt.Errorf("resolve jwks host %q: %w", normalizedHost, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve jwks host %q: no addresses found", normalizedHost)
	}

	allowedAddrs := make([]net.IPAddr, 0, len(addrs))
	for _, addr := range addrs {
		if isDisallowedIP(addr.IP) {
			return nil, fmt.Errorf("url host %q is not allowed: private or loopback addresses are forbidden", normalizedHost)
		}
		allowedAddrs = append(allowedAddrs, addr)
	}

	return allowedAddrs, nil
}

func withLookupTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		lookupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		return lookupCtx, cancel
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	return lookupCtx, cancel
}

// parseJWK converts a JWK to a public key.
// Currently supports RSA keys only.
func parseJWK(jwk JWK) (interface{}, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}

	// Decode base64url-encoded modulus
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode base64url-encoded exponent
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert bytes to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	publicKey := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	return publicKey, nil
}

// get retrieves a key from the cache if it exists and hasn't expired.
// Returns nil if the key is not found or the cache has expired.
func (c *jwksCache) get(kid string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Now().After(c.expiresAt) {
		return nil
	}

	return c.keys[kid]
}

// set updates the cache with new keys and resets the expiration time.
func (c *jwksCache) set(keys map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.keys = keys
	c.expiresAt = time.Now().Add(c.ttl)
}
