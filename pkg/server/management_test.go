package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

type testJWTValidator struct {
	validateFunc func(ctx context.Context, token string) (*auth.Claims, error)
}

func (v *testJWTValidator) Validate(ctx context.Context, token string) (*auth.Claims, error) {
	if v.validateFunc == nil {
		return nil, errors.New("validateFunc not configured")
	}
	return v.validateFunc(ctx, token)
}

func TestNewManagementServer(t *testing.T) {
	// Given: Management server configuration
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	// When: Creating a new management server
	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// Then: Server should be created with correct configuration
	if mgmtServer == nil {
		t.Fatal("Expected management server to be created, got nil")
	}

	if mgmtServer.Server == nil {
		t.Fatal("Expected base server to be initialized")
	}

	if mgmtServer.healthRegistry == nil {
		t.Fatal("Expected health registry to be set")
	}

	if mgmtServer.metricsRegistry == nil {
		t.Fatal("Expected metrics registry to be set")
	}

	if mgmtServer.config.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", mgmtServer.config.Port)
	}
}

// TestManagementServer_HealthEndpoint tests the /health endpoint
// Requirements: 30.1, 30.3
func TestManagementServer_HealthEndpoint(t *testing.T) {
	// Given: A management server
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// When: Making a request to /health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mgmtServer.router.ServeHTTP(rec, req)

	// Then: Should return 200 OK
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// And: Response should contain healthy status
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}
}

// TestManagementServer_ReadyEndpoint_AllHealthy tests /ready when all dependencies are healthy
// Requirements: 30.2, 30.4
func TestManagementServer_ReadyEndpoint_AllHealthy(t *testing.T) {
	// Given: A management server with healthy dependencies
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	// Register a healthy checker
	healthRegistry.Register(health.NewPingChecker("test"))

	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// When: Making a request to /ready
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	mgmtServer.router.ServeHTTP(rec, req)

	// Then: Should return 200 OK
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// And: Response should contain healthy status
	var response health.AggregatedResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != health.StatusHealthy {
		t.Errorf("Expected status 'healthy', got %v", response.Status)
	}
}

// TestManagementServer_ReadyEndpoint_Unhealthy tests /ready when dependencies are unhealthy
// Requirements: 30.5, 30.6
func TestManagementServer_ReadyEndpoint_Unhealthy(t *testing.T) {
	// Given: A management server with an unhealthy dependency
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	// Register an unhealthy checker
	healthRegistry.Register(health.NewCustomChecker("unhealthy-db", func(ctx context.Context) (health.Status, string, error) {
		return health.StatusUnhealthy, "Database connection failed", nil
	}))

	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// When: Making a request to /ready
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	mgmtServer.router.ServeHTTP(rec, req)

	// Then: Should return 503 Service Unavailable
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rec.Code)
	}

	// And: Response should contain unhealthy status
	var response health.AggregatedResult
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != health.StatusUnhealthy {
		t.Errorf("Expected status 'unhealthy', got %v", response.Status)
	}
}

// TestManagementServer_MetricsEndpoint tests the /metrics endpoint
// Requirements: 13.1, 13.7
func TestManagementServer_MetricsEndpoint(t *testing.T) {
	// Given: A management server
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// When: Making a request to /metrics
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	mgmtServer.router.ServeHTTP(rec, req)

	// Then: Should return 200 OK
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// And: Response should contain Prometheus metrics
	body := rec.Body.String()

	// Check that we got a valid Prometheus metrics response
	if body == "" {
		t.Fatal("Expected non-empty metrics response")
	}

	// Check for Go runtime metrics (always present)
	if !contains(body, "go_goroutines") {
		t.Error("Expected go_goroutines metric in response")
	}

	if !contains(body, "go_memstats") {
		t.Error("Expected go_memstats metrics in response")
	}

	// Check for HTTP in-flight metric (registered but may be 0)
	if !contains(body, "http_requests_in_flight") {
		t.Error("Expected http_requests_in_flight metric in response")
	}

	// Check for process metrics
	if !contains(body, "process_") {
		t.Error("Expected process metrics in response")
	}
}

// TestManagementServer_MiddlewareStack tests that middleware is applied correctly
func TestManagementServer_MiddlewareStack(t *testing.T) {
	// Given: A management server
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// When: Making a request to /health
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mgmtServer.router.ServeHTTP(rec, req)

	// Then: Request ID should be added to response headers
	requestID := rec.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header to be set by middleware")
	}
}

// TestManagementServer_BindsToConfiguredPort tests that the server binds to the configured port
// Requirements: 2.3, 2.7
func TestManagementServer_BindsToConfiguredPort(t *testing.T) {
	// Given: A management server with a specific port
	cfg := config.ManagementConfig{
		Port:         9091, // Different from default
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	healthRegistry := health.NewRegistry()
	metricsRegistry := metrics.NewRegistry()

	mgmtServer, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, nil)
	if err != nil {
		t.Fatalf("expected no error creating management server, got %v", err)
	}

	// Then: Server config should have the correct port
	if mgmtServer.config.Port != 9091 {
		t.Errorf("Expected port 9091, got %d", mgmtServer.config.Port)
	}
}

func TestNewManagementServer_AuthEnabledRequiresValidator(t *testing.T) {
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		AuthEnabled:  true,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	_, err = NewManagementServer(cfg, r, log, health.NewRegistry(), metrics.NewRegistry(), nil)
	if err == nil {
		t.Fatal("expected error when auth is enabled and validator is nil")
	}
}

func TestManagementServer_AuthAndScopes(t *testing.T) {
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		AuthEnabled:  true,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	validator := &testJWTValidator{
		validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
			switch token {
			case "metrics":
				return &auth.Claims{Subject: "u1", Scopes: []string{"management:metrics"}}, nil
			case "read":
				return &auth.Claims{Subject: "u1", Scopes: []string{"management:read"}}, nil
			case "swagger":
				return &auth.Claims{Subject: "u1", Scopes: []string{"management:swagger"}}, nil
			default:
				return nil, errors.New("invalid token")
			}
		},
	}

	mgmtServer, err := NewManagementServer(cfg, r, log, health.NewRegistry(), metrics.NewRegistry(), validator)
	if err != nil {
		t.Fatalf("unexpected server creation error: %v", err)
	}

	t.Run("health remains public without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		mgmtServer.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected /health 200, got %d", rec.Code)
		}
	})

	t.Run("ready requires token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		mgmtServer.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected /ready 401 without token, got %d", rec.Code)
		}
	})

	t.Run("metrics enforces metrics scope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer read")
		rec := httptest.NewRecorder()
		mgmtServer.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected /metrics 403 with only management:read scope, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer metrics")
		rec = httptest.NewRecorder()
		mgmtServer.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected /metrics 200 with management:metrics scope, got %d", rec.Code)
		}
	})

	t.Run("swagger enforces swagger scope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
		req.Header.Set("Authorization", "Bearer read")
		rec := httptest.NewRecorder()
		mgmtServer.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected /swagger 403 with read scope only, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/swagger", nil)
		req.Header.Set("Authorization", "Bearer swagger")
		rec = httptest.NewRecorder()
		mgmtServer.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected /swagger 200 with management:swagger scope, got %d", rec.Code)
		}
	})
}

func TestNewManagementServer_MTLSEnabledInvalidFiles(t *testing.T) {
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		MTLSEnabled:  true,
		TLSCertFile:  "/missing/server.crt",
		TLSKeyFile:   "/missing/server.key",
		TLSCAFile:    "/missing/ca.crt",
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	_, err = NewManagementServer(cfg, r, log, health.NewRegistry(), metrics.NewRegistry(), nil)
	if err == nil {
		t.Fatal("expected mTLS initialization error")
	}
	if !strings.Contains(err.Error(), "failed to load management mTLS config") {
		t.Fatalf("expected wrapped mTLS initialization error, got %v", err)
	}
}

func TestManagementSecurity_CombinedScenarios(t *testing.T) {
	certs := newTLSPropertyMaterial(t)
	serverTLS, err := LoadTLSConfig(certs.serverCertPath, certs.serverKeyPath, certs.caPath)
	if err != nil {
		t.Fatalf("failed to load mTLS config: %v", err)
	}

	validator := &testJWTValidator{
		validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
			if token == "ok" {
				return &auth.Claims{Subject: "u", Scopes: []string{"management:read"}}, nil
			}
			return nil, errors.New("invalid token")
		},
	}
	mgmt := newSecurityPropertyServer(t, true, validator)

	t.Run("valid cert without JWT returns 401", func(t *testing.T) {
		if err := verifyClientCertificate(serverTLS, &certs.validClientCert); err != nil {
			t.Fatalf("expected valid client cert, got %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		mgmt.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid JWT without cert fails mTLS gate", func(t *testing.T) {
		if err := verifyClientCertificate(serverTLS, nil); err == nil {
			t.Fatal("expected mTLS verification failure without client cert")
		}
	})

	t.Run("valid cert and JWT returns 200", func(t *testing.T) {
		if err := verifyClientCertificate(serverTLS, &certs.validClientCert); err != nil {
			t.Fatalf("expected valid client cert, got %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		req.Header.Set("Authorization", "Bearer ok")
		rec := httptest.NewRecorder()
		mgmt.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
