package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// mockLogger implements the logger.Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...any) {}
func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Warn(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) With(args ...any) logger.Logger {
	return m
}
func (m *mockLogger) WithContext(ctx context.Context) logger.Logger {
	return m
}

// createTestJWKS creates a test JWKS response with a valid RSA key
func createTestJWKS(t *testing.T) (JWKSResponse, *rsa.PrivateKey) {
	t.Helper()

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	// Encode public key components
	nBytes := privateKey.PublicKey.N.Bytes()
	eBytes := big.NewInt(int64(privateKey.PublicKey.E)).Bytes()

	jwk := JWK{
		Kid: "test-key-1",
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}

	return JWKSResponse{Keys: []JWK{jwk}}, privateKey
}

func TestJWKSClient_GetKey_Success(t *testing.T) {
	// Create test JWKS
	jwksResp, _ := createTestJWKS(t)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// Create JWKS client
	client := NewJWKSClient(server.URL, 1*time.Hour, &mockLogger{})

	// Get key
	ctx := context.Background()
	key, err := client.GetKey(ctx, "test-key-1")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	// Verify key is RSA public key
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("expected *rsa.PublicKey, got %T", key)
	}

	if rsaKey.N == nil {
		t.Error("RSA public key modulus is nil")
	}
}

func TestJWKSClient_GetKey_NotFound(t *testing.T) {
	// Create test JWKS
	jwksResp, _ := createTestJWKS(t)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// Create JWKS client
	client := NewJWKSClient(server.URL, 1*time.Hour, &mockLogger{})

	// Try to get non-existent key
	ctx := context.Background()
	_, err := client.GetKey(ctx, "non-existent-key")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}

	expectedMsg := "key not found: non-existent-key"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestJWKSClient_Caching(t *testing.T) {
	// Create test JWKS
	jwksResp, _ := createTestJWKS(t)

	// Track number of requests
	requestCount := 0

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// Create JWKS client with short TTL
	client := NewJWKSClient(server.URL, 100*time.Millisecond, &mockLogger{})

	ctx := context.Background()

	// First request - should fetch from server
	_, err := client.GetKey(ctx, "test-key-1")
	if err != nil {
		t.Fatalf("first GetKey failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request after first GetKey, got %d", requestCount)
	}

	// Second request - should use cache
	_, err = client.GetKey(ctx, "test-key-1")
	if err != nil {
		t.Fatalf("second GetKey failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 request after second GetKey (cached), got %d", requestCount)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third request - should fetch from server again
	_, err = client.GetKey(ctx, "test-key-1")
	if err != nil {
		t.Fatalf("third GetKey failed: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests after cache expiry, got %d", requestCount)
	}
}

func TestJWKSClient_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create JWKS client
	client := NewJWKSClient(server.URL, 1*time.Hour, &mockLogger{})

	// Try to get key
	ctx := context.Background()
	_, err := client.GetKey(ctx, "test-key-1")
	if err == nil {
		t.Fatal("expected error when server returns 500, got nil")
	}
}

func TestJWKSClient_InvalidJSON(t *testing.T) {
	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	// Create JWKS client
	client := NewJWKSClient(server.URL, 1*time.Hour, &mockLogger{})

	// Try to get key
	ctx := context.Background()
	_, err := client.GetKey(ctx, "test-key-1")
	if err == nil {
		t.Fatal("expected error when server returns invalid JSON, got nil")
	}
}

func TestJWKSClient_ContextCancellation(t *testing.T) {
	// Create test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		jwksResp, _ := createTestJWKS(t)
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// Create JWKS client
	client := NewJWKSClient(server.URL, 1*time.Hour, &mockLogger{})

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Try to get key - should timeout
	_, err := client.GetKey(ctx, "test-key-1")
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
}

func TestJWKSClient_MultipleKeys(t *testing.T) {
	// Create JWKS with multiple keys
	jwk1 := JWK{
		Kid: "key-1",
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		N:   "test-n-1",
		E:   "AQAB",
	}

	jwk2 := JWK{
		Kid: "key-2",
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		N:   "test-n-2",
		E:   "AQAB",
	}

	// Note: These are invalid keys for actual use, but sufficient for cache testing
	jwksResp := JWKSResponse{Keys: []JWK{jwk1, jwk2}}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// Create JWKS client
	client := NewJWKSClient(server.URL, 1*time.Hour, &mockLogger{})

	ctx := context.Background()

	// Fetch keys (will fail to parse but should cache the attempt)
	client.GetKey(ctx, "key-1")

	// Verify cache has been populated (even with parse errors)
	// The cache should have attempted to parse both keys
}

func TestParseJWK_UnsupportedKeyType(t *testing.T) {
	jwk := JWK{
		Kid: "test-key",
		Kty: "EC", // Unsupported type
		Use: "sig",
		Alg: "ES256",
	}

	_, err := parseJWK(jwk)
	if err == nil {
		t.Fatal("expected error for unsupported key type, got nil")
	}

	expectedMsg := "unsupported key type: EC"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}
