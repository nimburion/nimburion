package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/middleware/authz"
	"github.com/nimburion/nimburion/pkg/server/router"
)

func TestNewTokenBucketLimiter(t *testing.T) {
	tests := []struct {
		name               string
		requestsPerSecond  int
		burst              int
		wantRequestsPerSec int
		wantBurst          int
	}{
		{
			name:               "standard rate limit",
			requestsPerSecond:  100,
			burst:              200,
			wantRequestsPerSec: 100,
			wantBurst:          200,
		},
		{
			name:               "low rate limit",
			requestsPerSecond:  1,
			burst:              5,
			wantRequestsPerSec: 1,
			wantBurst:          5,
		},
		{
			name:               "high rate limit",
			requestsPerSecond:  1000,
			burst:              2000,
			wantRequestsPerSec: 1000,
			wantBurst:          2000,
		},
		{
			name:               "burst equals rate",
			requestsPerSecond:  50,
			burst:              50,
			wantRequestsPerSec: 50,
			wantBurst:          50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewTokenBucketLimiter(tt.requestsPerSecond, tt.burst)

			if limiter == nil {
				t.Fatal("NewTokenBucketLimiter returned nil")
			}

			if float64(limiter.rate) != float64(tt.wantRequestsPerSec) {
				t.Errorf("rate = %v, want %v", limiter.rate, tt.wantRequestsPerSec)
			}

			if limiter.burst != tt.wantBurst {
				t.Errorf("burst = %v, want %v", limiter.burst, tt.wantBurst)
			}
		})
	}
}

func TestTokenBucketLimiter_Allow(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerSecond int
		burst             int
		key               string
		numRequests       int
		wantAllowed       int
	}{
		{
			name:              "all requests within burst",
			requestsPerSecond: 10,
			burst:             20,
			key:               "user-1",
			numRequests:       20,
			wantAllowed:       20,
		},
		{
			name:              "requests exceed burst",
			requestsPerSecond: 10,
			burst:             5,
			key:               "user-2",
			numRequests:       10,
			wantAllowed:       5,
		},
		{
			name:              "single request always allowed",
			requestsPerSecond: 1,
			burst:             1,
			key:               "user-3",
			numRequests:       1,
			wantAllowed:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewTokenBucketLimiter(tt.requestsPerSecond, tt.burst)

			allowed := 0
			for i := 0; i < tt.numRequests; i++ {
				if limiter.Allow(tt.key) {
					allowed++
				}
			}

			if allowed != tt.wantAllowed {
				t.Errorf("allowed = %v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestTokenBucketLimiter_PerKeyIsolation(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// Exhaust rate limit for key1
	for i := 0; i < 5; i++ {
		if !limiter.Allow("key1") {
			t.Errorf("request %d for key1 should be allowed", i)
		}
	}

	// key1 should now be rate limited
	if limiter.Allow("key1") {
		t.Error("key1 should be rate limited")
	}

	// key2 should still have full capacity
	for i := 0; i < 5; i++ {
		if !limiter.Allow("key2") {
			t.Errorf("request %d for key2 should be allowed", i)
		}
	}

	// key2 should now be rate limited
	if limiter.Allow("key2") {
		t.Error("key2 should be rate limited")
	}
}

func TestTokenBucketLimiter_TokenRefill(t *testing.T) {
	// Use a very low rate for predictable timing
	limiter := NewTokenBucketLimiter(10, 2)

	// Exhaust the burst
	if !limiter.Allow("user") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow("user") {
		t.Fatal("second request should be allowed")
	}

	// Should be rate limited now
	if limiter.Allow("user") {
		t.Error("third request should be rate limited")
	}

	// Wait for tokens to refill (100ms = 1 token at 10 req/s)
	time.Sleep(150 * time.Millisecond)

	// Should allow one more request after refill
	if !limiter.Allow("user") {
		t.Error("request after refill should be allowed")
	}
}

func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(100, 50)
	numGoroutines := 10
	requestsPerGoroutine := 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines making concurrent requests
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := "concurrent-user"

			for j := 0; j < requestsPerGoroutine; j++ {
				// Just call Allow - we're testing thread safety, not the result
				limiter.Allow(key)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	// If there's a race condition, this will be caught by -race flag
	wg.Wait()
}

func TestTokenBucketLimiter_MultipleKeys(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)
	keys := []string{"user1", "user2", "user3", "192.168.1.1", "192.168.1.2"}

	// Each key should have independent rate limits
	for _, key := range keys {
		allowed := 0
		for i := 0; i < 10; i++ {
			if limiter.Allow(key) {
				allowed++
			}
		}

		if allowed != 5 {
			t.Errorf("key %s: allowed = %v, want 5", key, allowed)
		}
	}
}

func TestTokenBucketLimiter_ZeroRate(t *testing.T) {
	// Edge case: zero rate should still allow burst
	limiter := NewTokenBucketLimiter(0, 5)

	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow("user") {
			allowed++
		}
	}

	// With zero rate, only burst tokens are available
	if allowed != 5 {
		t.Errorf("allowed = %v, want 5", allowed)
	}

	// Wait and verify no refill happens with zero rate
	time.Sleep(100 * time.Millisecond)
	if limiter.Allow("user") {
		t.Error("no requests should be allowed after burst with zero rate")
	}
}

func TestTokenBucketLimiter_getLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)

	// Get limiter for a new key
	limiter1 := limiter.getLimiter("key1")
	if limiter1 == nil {
		t.Fatal("getLimiter returned nil")
	}

	// Get limiter for the same key again
	limiter2 := limiter.getLimiter("key1")
	if limiter2 == nil {
		t.Fatal("getLimiter returned nil on second call")
	}

	// Should return the same limiter instance
	if limiter1 != limiter2 {
		t.Error("getLimiter should return the same instance for the same key")
	}

	// Get limiter for a different key
	limiter3 := limiter.getLimiter("key2")
	if limiter3 == nil {
		t.Fatal("getLimiter returned nil for different key")
	}

	// Should return a different limiter instance
	if limiter1 == limiter3 {
		t.Error("getLimiter should return different instances for different keys")
	}
}

func BenchmarkTokenBucketLimiter_Allow(b *testing.B) {
	limiter := NewTokenBucketLimiter(1000, 2000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("benchmark-key")
	}
}

func BenchmarkTokenBucketLimiter_AllowMultipleKeys(b *testing.B) {
	limiter := NewTokenBucketLimiter(1000, 2000)
	keys := []string{"key1", "key2", "key3", "key4", "key5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		limiter.Allow(key)
	}
}

func BenchmarkTokenBucketLimiter_ConcurrentAllow(b *testing.B) {
	limiter := NewTokenBucketLimiter(1000, 2000)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow("concurrent-key")
		}
	})
}

// Mock router.Context for testing middleware
type mockContext struct {
	request  *http.Request
	response *mockResponseWriter
	data     map[string]interface{}
}

func newMockContext(r *http.Request) *mockContext {
	return &mockContext{
		request:  r,
		response: &mockResponseWriter{header: make(http.Header)},
		data:     make(map[string]interface{}),
	}
}

func (m *mockContext) Request() *http.Request {
	return m.request
}

func (m *mockContext) SetRequest(r *http.Request) {
	m.request = r
}

func (m *mockContext) Response() router.ResponseWriter {
	return m.response
}

func (m *mockContext) SetResponse(w router.ResponseWriter) {
	if response, ok := w.(*mockResponseWriter); ok {
		m.response = response
	}
}

func (m *mockContext) Param(name string) string {
	return ""
}

func (m *mockContext) Query(name string) string {
	return m.request.URL.Query().Get(name)
}

func (m *mockContext) Bind(v interface{}) error {
	return nil
}

func (m *mockContext) JSON(code int, v interface{}) error {
	m.response.statusCode = code
	m.response.written = true
	return nil
}

func (m *mockContext) String(code int, s string) error {
	m.response.statusCode = code
	m.response.written = true
	return nil
}

func (m *mockContext) Get(key string) interface{} {
	return m.data[key]
}

func (m *mockContext) Set(key string, value interface{}) {
	m.data[key] = value
}

type mockResponseWriter struct {
	header     http.Header
	statusCode int
	written    bool
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.written = true
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	m.written = true
}

func (m *mockResponseWriter) Status() int {
	return m.statusCode
}

func (m *mockResponseWriter) Written() bool {
	return m.written
}

func TestRateLimit_AllowsRequestWithinLimit(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 5)
	cfg := RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             5,
		KeyFunc: func(c router.Context) string {
			return "test-key"
		},
	}

	middleware := RateLimit(limiter, cfg)
	nextCalled := false
	next := func(c router.Context) error {
		nextCalled = true
		return nil
	}

	handler := middleware(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := newMockContext(req)

	err := handler(ctx)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !nextCalled {
		t.Error("expected next handler to be called")
	}

	if ctx.Response().Status() != 0 {
		t.Errorf("expected status 0 (not set), got %d", ctx.Response().Status())
	}
}

func TestRateLimit_RejectsRequestExceedingLimit(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 2)
	cfg := RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             2,
		KeyFunc: func(c router.Context) string {
			return "test-key"
		},
	}

	middleware := RateLimit(limiter, cfg)
	next := func(c router.Context) error {
		return nil
	}

	handler := middleware(next)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := newMockContext(req)
		err := handler(ctx)
		if err != nil {
			t.Errorf("request %d: expected no error, got %v", i, err)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := newMockContext(req)
	err := handler(ctx)

	if err != nil {
		t.Errorf("expected no error (error handled in middleware), got %v", err)
	}

	if ctx.Response().Status() != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, ctx.Response().Status())
	}

	retryAfter := ctx.Response().Header().Get("Retry-After")
	if retryAfter != "1" {
		t.Errorf("expected Retry-After header '1', got '%s'", retryAfter)
	}
}

func TestRateLimit_PerIPRateLimiting(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 2)
	cfg := RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             2,
		KeyFunc: func(c router.Context) string {
			return ExtractIPFromRequest(c.Request())
		},
	}

	middleware := RateLimit(limiter, cfg)
	next := func(c router.Context) error {
		return nil
	}

	handler := middleware(next)

	// IP1 exhausts its limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		ctx := newMockContext(req)
		err := handler(ctx)
		if err != nil {
			t.Errorf("IP1 request %d: expected no error, got %v", i, err)
		}
	}

	// IP1 should be rate limited
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	ctx1 := newMockContext(req1)
	err := handler(ctx1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if ctx1.Response().Status() != http.StatusTooManyRequests {
		t.Errorf("IP1: expected status %d, got %d", http.StatusTooManyRequests, ctx1.Response().Status())
	}

	// IP2 should still have full capacity
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.2:54321"
		ctx := newMockContext(req)
		err := handler(ctx)
		if err != nil {
			t.Errorf("IP2 request %d: expected no error, got %v", i, err)
		}
		if ctx.Response().Status() == http.StatusTooManyRequests {
			t.Errorf("IP2 request %d: should not be rate limited", i)
		}
	}
}

func TestRateLimit_PerUserRateLimiting(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 2)
	cfg := RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             2,
		KeyFunc: func(c router.Context) string {
			return ExtractUserIDFromContext(c)
		},
	}

	middleware := RateLimit(limiter, cfg)
	next := func(c router.Context) error {
		return nil
	}

	handler := middleware(next)

	// User1 exhausts their limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := newMockContext(req)
		ctx.Set(authz.ClaimsKey, &auth.Claims{Subject: "user-1"})
		err := handler(ctx)
		if err != nil {
			t.Errorf("User1 request %d: expected no error, got %v", i, err)
		}
	}

	// User1 should be rate limited
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx1 := newMockContext(req1)
	ctx1.Set(authz.ClaimsKey, &auth.Claims{Subject: "user-1"})
	err := handler(ctx1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if ctx1.Response().Status() != http.StatusTooManyRequests {
		t.Errorf("User1: expected status %d, got %d", http.StatusTooManyRequests, ctx1.Response().Status())
	}

	// User2 should still have full capacity
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := newMockContext(req)
		ctx.Set(authz.ClaimsKey, &auth.Claims{Subject: "user-2"})
		err := handler(ctx)
		if err != nil {
			t.Errorf("User2 request %d: expected no error, got %v", i, err)
		}
		if ctx.Response().Status() == http.StatusTooManyRequests {
			t.Errorf("User2 request %d: should not be rate limited", i)
		}
	}
}

func TestExtractIPFromRequest(t *testing.T) {
	tests := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		expectedIP    string
	}{
		{
			name:       "extract from RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "extract from X-Forwarded-For",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1, 198.51.100.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "extract from X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xRealIP:    "203.0.113.2",
			expectedIP: "203.0.113.2",
		},
		{
			name:          "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr:    "10.0.0.1:12345",
			xForwardedFor: "203.0.113.1",
			xRealIP:       "203.0.113.2",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			expectedIP: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := ExtractIPFromRequest(req)

			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestExtractUserIDFromContext(t *testing.T) {
	tests := []struct {
		name           string
		claims         interface{}
		expectedUserID string
	}{
		{
			name:           "extract user ID from valid claims",
			claims:         &auth.Claims{Subject: "user-123"},
			expectedUserID: "user-123",
		},
		{
			name:           "no claims in context",
			claims:         nil,
			expectedUserID: "",
		},
		{
			name:           "invalid claims type",
			claims:         "invalid",
			expectedUserID: "",
		},
		{
			name:           "empty subject",
			claims:         &auth.Claims{Subject: ""},
			expectedUserID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			ctx := newMockContext(req)

			if tt.claims != nil {
				ctx.Set(authz.ClaimsKey, tt.claims)
			}

			userID := ExtractUserIDFromContext(ctx)

			if userID != tt.expectedUserID {
				t.Errorf("expected user ID %s, got %s", tt.expectedUserID, userID)
			}
		})
	}
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 1)
	cfg := RateLimitConfig{
		RequestsPerSecond: 10,
		Burst:             1,
		KeyFunc: func(c router.Context) string {
			return "test-key"
		},
	}

	middleware := RateLimit(limiter, cfg)
	next := func(c router.Context) error {
		return nil
	}

	handler := middleware(next)

	// First request succeeds
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx1 := newMockContext(req1)
	handler(ctx1)

	// Second request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx2 := newMockContext(req2)
	handler(ctx2)

	retryAfter := ctx2.Response().Header().Get("Retry-After")
	if retryAfter != "1" {
		t.Errorf("expected Retry-After header '1', got '%s'", retryAfter)
	}
}
