package ratelimit

import (
	"net/http"
	"strings"
	"sync"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/middleware/authz"
	"github.com/nimburion/nimburion/pkg/server/router"
	"golang.org/x/time/rate"
)

// RateLimiter defines the interface for rate limiting implementations.
// Implementations must be thread-safe and support per-key rate limiting.
type RateLimiter interface {
	// Allow checks if a request for the given key should be allowed.
	// Returns true if the request is within rate limits, false otherwise.
	Allow(key string) bool
}

// TokenBucketLimiter implements the token bucket algorithm for rate limiting.
// It provides per-key rate limiting with configurable requests per second and burst capacity.
// The implementation is thread-safe using sync.Map for concurrent access.
//
// The token bucket algorithm allows for burst traffic up to the burst limit,
// while maintaining an average rate of requests per second over time.
type TokenBucketLimiter struct {
	limiters sync.Map   // map[string]*rate.Limiter - per-key rate limiters
	rate     rate.Limit // requests per second
	burst    int        // maximum burst size
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
//
// Parameters:
//   - requestsPerSecond: the maximum average number of requests allowed per second
//   - burst: the maximum number of requests allowed in a burst
//
// The burst parameter allows for temporary spikes in traffic. For example,
// with requestsPerSecond=10 and burst=20, a client can make 20 requests
// immediately, but subsequent requests will be limited to 10 per second.
//
// Example:
//
//	limiter := NewTokenBucketLimiter(100, 200) // 100 req/s, burst of 200
//	if limiter.Allow("user-123") {
//	    // Process request
//	} else {
//	    // Reject with 429 Too Many Requests
//	}
func NewTokenBucketLimiter(requestsPerSecond int, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		rate:  rate.Limit(requestsPerSecond),
		burst: burst,
	}
}

// Allow checks if a request for the given key should be allowed based on rate limits.
// Each key maintains its own independent rate limiter, enabling per-IP or per-user
// rate limiting.
//
// The method is thread-safe and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - key: the identifier for rate limiting (e.g., IP address, user ID)
//
// Returns:
//   - true if the request is allowed (within rate limits)
//   - false if the request should be rejected (rate limit exceeded)
func (l *TokenBucketLimiter) Allow(key string) bool {
	limiter := l.getLimiter(key)
	return limiter.Allow()
}

// getLimiter retrieves or creates a rate limiter for the given key.
// This method is thread-safe and uses sync.Map for concurrent access.
//
// If a limiter for the key already exists, it is returned.
// Otherwise, a new limiter is created with the configured rate and burst settings.
func (l *TokenBucketLimiter) getLimiter(key string) *rate.Limiter {
	// Try to load existing limiter
	if limiter, ok := l.limiters.Load(key); ok {
		return limiter.(*rate.Limiter)
	}

	// Create new limiter for this key
	limiter := rate.NewLimiter(l.rate, l.burst)
	l.limiters.Store(key, limiter)
	return limiter
}

// Config defines the configuration for rate limiting middleware.
type Config struct {
	// RequestsPerSecond is the maximum average number of requests allowed per second.
	RequestsPerSecond int
	// Burst is the maximum number of requests allowed in a burst.
	Burst int
	// KeyFunc extracts the rate limiting key from the request context.
	// Common implementations:
	//   - Per-IP: func(c router.Context) string { return c.Request().RemoteAddr }
	//   - Per-User: func(c router.Context) string { return getUserID(c) }
	KeyFunc func(router.Context) string
}

// RateLimit creates middleware that enforces rate limiting based on the provided configuration.
// It uses the configured KeyFunc to extract a rate limiting key (e.g., IP address or user ID),
// checks if the request is within rate limits, and returns HTTP 429 with Retry-After header
// when the rate limit is exceeded.
//
// The middleware supports both per-IP and per-user rate limiting through the KeyFunc.
//
// Example usage:
//
//	// Per-IP rate limiting
//	limiter := NewTokenBucketLimiter(100, 200)
//	rateLimitMiddleware := RateLimit(limiter, Config{
//	    RequestsPerSecond: 100,
//	    Burst:             200,
//	    KeyFunc: func(c router.Context) string {
//	        return extractIP(c.Request())
//	    },
//	})
//
//	// Per-user rate limiting
//	rateLimitMiddleware := RateLimit(limiter, Config{
//	    RequestsPerSecond: 100,
//	    Burst:             200,
//	    KeyFunc: func(c router.Context) string {
//	        claims := c.Get("claims").(*auth.Claims)
//	        return claims.Subject
//	    },
//	})
//
// Requirements: 34.1, 34.4, 34.5, 34.6, 34.7
func RateLimit(limiter RateLimiter, cfg Config) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Extract rate limiting key using the configured KeyFunc
			key := cfg.KeyFunc(c)

			// Check if request is within rate limits
			if !limiter.Allow(key) {
				// Set Retry-After header (1 second as a reasonable default)
				c.Response().Header().Set("Retry-After", "1")

				// Return 429 Too Many Requests
				return c.JSON(http.StatusTooManyRequests, map[string]interface{}{
					"error": "rate limit exceeded",
				})
			}

			// Request is within rate limits, proceed to next handler
			return next(c)
		}
	}
}

// ExtractIPFromRequest extracts the client IP address from the HTTP request.
// It checks X-Forwarded-For and X-Real-IP headers first (for proxied requests),
// then falls back to RemoteAddr.
//
// This is a helper function for implementing per-IP rate limiting.
//
// Example:
//
//	cfg := Config{
//	    KeyFunc: func(c router.Context) string {
//	        return ExtractIPFromRequest(c.Request())
//	    },
//	}
//
// Requirements: 34.3
func ExtractIPFromRequest(r *http.Request) string {
	// Check X-Forwarded-For header (set by proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Check X-Real-IP header (alternative proxy header)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	// RemoteAddr is in format "IP:port", extract just the IP
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}

	return addr
}

// ExtractUserIDFromContext extracts the user ID from JWT claims in the context.
// This is a helper function for implementing per-user rate limiting.
//
// Returns an empty string if no claims are found or if the subject is empty.
//
// Example:
//
//	cfg := Config{
//	    KeyFunc: func(c router.Context) string {
//	        return ExtractUserIDFromContext(c)
//	    },
//	}
//
// Requirements: 34.4
func ExtractUserIDFromContext(c router.Context) string {
	claimsValue := c.Get(authz.ClaimsKey)
	if claimsValue == nil {
		return ""
	}

	claims, ok := claimsValue.(*auth.Claims)
	if !ok || claims == nil {
		return ""
	}

	return claims.Subject
}
