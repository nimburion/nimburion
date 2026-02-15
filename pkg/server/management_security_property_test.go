package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestProperty_ManagementJWTRequiredWhenEnabled(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	endpointGen := gen.OneConstOf("/ready", "/metrics", "/swagger")
	authCaseGen := gen.OneConstOf("missing", "invalid", "valid")

	props.Property("management endpoints require valid JWT when auth is enabled", prop.ForAll(
		func(endpoint string, authCase string) bool {
			validator := &testJWTValidator{
				validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
					switch token {
					case "ready":
						return &auth.Claims{Subject: "u", Scopes: []string{"management:read"}}, nil
					case "metrics":
						return &auth.Claims{Subject: "u", Scopes: []string{"management:metrics"}}, nil
					case "swagger":
						return &auth.Claims{Subject: "u", Scopes: []string{"management:swagger"}}, nil
					default:
						return nil, errors.New("invalid token")
					}
				},
			}
			server := newSecurityPropertyServer(t, true, validator)

			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			if authCase == "invalid" {
				req.Header.Set("Authorization", "Bearer bad")
			}
			if authCase == "valid" {
				req.Header.Set("Authorization", "Bearer "+strings.TrimPrefix(endpoint, "/"))
			}

			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)

			expected := http.StatusUnauthorized
			if authCase == "valid" {
				expected = http.StatusOK
			}
			return rec.Code == expected
		},
		endpointGen,
		authCaseGen,
	))

	props.TestingRun(t)
}

func TestProperty_ManagementNoJWTWhenDisabled(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	endpointGen := gen.OneConstOf("/health", "/ready", "/metrics", "/swagger")

	props.Property("management endpoints are accessible without JWT when auth is disabled", prop.ForAll(
		func(endpoint string) bool {
			server := newSecurityPropertyServer(t, false, nil)
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)
			return rec.Code == http.StatusOK
		},
		endpointGen,
	))

	props.TestingRun(t)
}

func TestProperty_ManagementScopeAuthorization(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	endpointGen := gen.OneConstOf("/ready", "/metrics", "/swagger")
	scopeGen := gen.OneConstOf("", "management:read", "management:metrics", "management:swagger")

	props.Property("insufficient scope returns 403 while required scope returns 200", prop.ForAll(
		func(endpoint, scope string) bool {
			validator := &testJWTValidator{
				validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
					if token == "ok" {
						if scope == "" {
							return &auth.Claims{Subject: "u", Scopes: nil}, nil
						}
						return &auth.Claims{Subject: "u", Scopes: []string{scope}}, nil
					}
					return nil, errors.New("invalid token")
				},
			}
			server := newSecurityPropertyServer(t, true, validator)

			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			req.Header.Set("Authorization", "Bearer ok")
			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)

			required := requiredScopeForEndpoint(endpoint)
			if scope == required {
				return rec.Code == http.StatusOK
			}
			return rec.Code == http.StatusForbidden
		},
		endpointGen,
		scopeGen,
	))

	props.TestingRun(t)
}

func TestProperty_MTLSRejectsMissingOrInvalidClientCert(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	certs := newTLSPropertyMaterial(t)
	clientModeGen := gen.OneConstOf("missing", "invalid")

	props.Property("mTLS rejects missing or invalid client certificates", prop.ForAll(
		func(clientMode string) bool {
			serverTLS, err := LoadTLSConfig(certs.serverCertPath, certs.serverKeyPath, certs.caPath)
			if err != nil {
				t.Logf("failed to load server TLS config: %v", err)
				return false
			}
			clientTLS := &tls.Config{
				RootCAs:    certs.serverCAPool,
				ServerName: "localhost",
			}
			if clientMode == "invalid" {
				clientTLS.Certificates = []tls.Certificate{certs.invalidClientCert}
			}
			var clientCert *tls.Certificate
			if len(clientTLS.Certificates) > 0 {
				clientCert = &clientTLS.Certificates[0]
			}
			return verifyClientCertificate(serverTLS, clientCert) != nil
		},
		clientModeGen,
	))

	props.TestingRun(t)
}

func TestProperty_MTLSAcceptsValidClientCert(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	certs := newTLSPropertyMaterial(t)

	props.Property("mTLS accepts valid client certificates", prop.ForAll(
		func(_ bool) bool {
			serverTLS, err := LoadTLSConfig(certs.serverCertPath, certs.serverKeyPath, certs.caPath)
			if err != nil {
				t.Logf("failed to load server TLS config: %v", err)
				return false
			}
			clientTLS := &tls.Config{
				RootCAs:      certs.serverCAPool,
				ServerName:   "localhost",
				Certificates: []tls.Certificate{certs.validClientCert},
			}
			return verifyClientCertificate(serverTLS, &clientTLS.Certificates[0]) == nil
		},
		gen.Bool(),
	))

	props.TestingRun(t)
}

func TestProperty_MTLSOptionalWhenDisabled(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	certs := newTLSPropertyMaterial(t)

	props.Property("TLS without mTLS accepts clients with or without client certificates", prop.ForAll(
		func(withClientCert bool) bool {
			serverTLS := &tls.Config{
				Certificates: []tls.Certificate{certs.serverCert},
				MinVersion:   tls.VersionTLS12,
				ClientAuth:   tls.NoClientCert,
			}
			clientTLS := &tls.Config{
				RootCAs:    certs.serverCAPool,
				ServerName: "localhost",
			}
			if withClientCert {
				clientTLS.Certificates = []tls.Certificate{certs.validClientCert}
			}
			var clientCert *tls.Certificate
			if len(clientTLS.Certificates) > 0 {
				clientCert = &clientTLS.Certificates[0]
			}
			return verifyClientCertificate(serverTLS, clientCert) == nil
		},
		gen.Bool(),
	))

	props.TestingRun(t)
}

func TestProperty_CombinedJWTAndMTLS(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	certs := newTLSPropertyMaterial(t)

	props.Property("request is accepted only when both client cert and JWT are valid", prop.ForAll(
		func(withClientCert, withJWT bool) bool {
			// Transport-level gate (mTLS)
			serverTLS, err := LoadTLSConfig(certs.serverCertPath, certs.serverKeyPath, certs.caPath)
			if err != nil {
				t.Logf("failed to load server TLS config: %v", err)
				return false
			}
			clientTLS := &tls.Config{
				RootCAs:    certs.serverCAPool,
				ServerName: "localhost",
			}
			if withClientCert {
				clientTLS.Certificates = []tls.Certificate{certs.validClientCert}
			}
			var clientCert *tls.Certificate
			if len(clientTLS.Certificates) > 0 {
				clientCert = &clientTLS.Certificates[0]
			}
			handshakeOK := verifyClientCertificate(serverTLS, clientCert) == nil
			if !withClientCert {
				return !handshakeOK
			}
			if !handshakeOK {
				return false
			}

			// Application-level gate (JWT)
			validator := &testJWTValidator{
				validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
					if token == "ok" {
						return &auth.Claims{Subject: "u", Scopes: []string{"management:read"}}, nil
					}
					return nil, errors.New("invalid token")
				},
			}
			server := newSecurityPropertyServer(t, true, validator)
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			if withJWT {
				req.Header.Set("Authorization", "Bearer ok")
			}
			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)

			if withJWT {
				return rec.Code == http.StatusOK
			}
			return rec.Code == http.StatusUnauthorized
		},
		gen.Bool(),
		gen.Bool(),
	))

	props.TestingRun(t)
}

func TestProperty_HealthAlwaysAccessibleWithoutJWT(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	authEnabledGen := gen.Bool()
	tokenCaseGen := gen.OneConstOf("missing", "invalid", "valid")

	props.Property("/health does not require JWT regardless of management auth flag", prop.ForAll(
		func(authEnabled bool, tokenCase string) bool {
			validator := &testJWTValidator{
				validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
					if token == "ok" {
						return &auth.Claims{Subject: "u", Scopes: []string{"management:read"}}, nil
					}
					return nil, errors.New("invalid token")
				},
			}
			var v auth.JWTValidator
			if authEnabled {
				v = validator
			}
			server := newSecurityPropertyServer(t, authEnabled, v)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			if tokenCase == "invalid" {
				req.Header.Set("Authorization", "Bearer bad")
			}
			if tokenCase == "valid" {
				req.Header.Set("Authorization", "Bearer ok")
			}
			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)
			return rec.Code == http.StatusOK
		},
		authEnabledGen,
		tokenCaseGen,
	))

	props.TestingRun(t)
}

func TestProperty_HealthRequiresMTLSWhenEnabled(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 100
	props := gopter.NewProperties(params)

	certs := newTLSPropertyMaterial(t)

	props.Property("when mTLS is enabled, handshake blocks health access without client cert", prop.ForAll(
		func(withClientCert bool) bool {
			serverTLS, err := LoadTLSConfig(certs.serverCertPath, certs.serverKeyPath, certs.caPath)
			if err != nil {
				t.Logf("failed to load server TLS config: %v", err)
				return false
			}
			clientTLS := &tls.Config{
				RootCAs:    certs.serverCAPool,
				ServerName: "localhost",
			}
			if withClientCert {
				clientTLS.Certificates = []tls.Certificate{certs.validClientCert}
			}
			var clientCert *tls.Certificate
			if len(clientTLS.Certificates) > 0 {
				clientCert = &clientTLS.Certificates[0]
			}
			handshakeOK := verifyClientCertificate(serverTLS, clientCert) == nil
			if withClientCert {
				return handshakeOK
			}
			return !handshakeOK
		},
		gen.Bool(),
	))

	props.TestingRun(t)
}

func TestIntegration_ManagementSecurityE2E_ConcurrentAndBypass(t *testing.T) {
	validator := &testJWTValidator{
		validateFunc: func(_ context.Context, token string) (*auth.Claims, error) {
			switch token {
			case "metrics":
				return &auth.Claims{Subject: "u", Scopes: []string{"management:metrics"}}, nil
			case "read":
				return &auth.Claims{Subject: "u", Scopes: []string{"management:read"}}, nil
			default:
				return nil, errors.New("invalid token")
			}
		},
	}
	server := newSecurityPropertyServer(t, true, validator)

	// Concurrent requests with mixed auth levels.
	var wg sync.WaitGroup
	results := make(chan int, 30)
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			results <- performStatus(server, "/metrics", "")
		}()
		go func() {
			defer wg.Done()
			results <- performStatus(server, "/metrics", "read")
		}()
		go func() {
			defer wg.Done()
			results <- performStatus(server, "/metrics", "metrics")
		}()
	}
	wg.Wait()
	close(results)

	var unauthorized, forbidden, ok int
	for code := range results {
		switch code {
		case http.StatusUnauthorized:
			unauthorized++
		case http.StatusForbidden:
			forbidden++
		case http.StatusOK:
			ok++
		default:
			t.Fatalf("unexpected status in concurrent run: %d", code)
		}
	}
	if unauthorized == 0 || forbidden == 0 || ok == 0 {
		t.Fatalf("expected mixed outcomes (401/403/200), got 401=%d 403=%d 200=%d", unauthorized, forbidden, ok)
	}

	// Bypass attempts.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for non-bearer bypass attempt, got %d", rec.Code)
	}
}

func TestTLSHandshakeHelper_Smoke(t *testing.T) {
	certs := newTLSPropertyMaterial(t)
	serverTLS, err := LoadTLSConfig(certs.serverCertPath, certs.serverKeyPath, certs.caPath)
	if err != nil {
		t.Fatalf("failed to load tls config: %v", err)
	}
	clientTLS := &tls.Config{
		RootCAs:      certs.serverCAPool,
		ServerName:   "localhost",
		Certificates: []tls.Certificate{certs.validClientCert},
	}
	if err := verifyClientCertificate(serverTLS, &clientTLS.Certificates[0]); err != nil {
		t.Fatalf("expected client cert verification success, got %v", err)
	}
}

type tlsPropertyMaterial struct {
	caPath            string
	serverCertPath    string
	serverKeyPath     string
	serverCert        tls.Certificate
	validClientCert   tls.Certificate
	invalidClientCert tls.Certificate
	serverCAPool      *x509.CertPool
}

func newTLSPropertyMaterial(t *testing.T) tlsPropertyMaterial {
	t.Helper()
	dir := t.TempDir()
	caPath, serverCertPath, serverKeyPath, clientCertPath, clientKeyPath := writeTestCertificates(t, dir)

	validClientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		t.Fatalf("failed loading valid client cert/key: %v", err)
	}
	serverCert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		t.Fatalf("failed loading server cert/key: %v", err)
	}

	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatalf("failed reading CA file: %v", err)
	}
	serverCAPool := x509.NewCertPool()
	if !serverCAPool.AppendCertsFromPEM(caPEM) {
		t.Fatalf("failed parsing CA pem")
	}

	// Build an invalid client cert signed by another CA.
	otherDir := t.TempDir()
	_, _, _, invalidClientCertPath, invalidClientKeyPath := writeTestCertificates(t, otherDir)
	invalidClientCert, err := tls.LoadX509KeyPair(invalidClientCertPath, invalidClientKeyPath)
	if err != nil {
		t.Fatalf("failed loading invalid client cert/key: %v", err)
	}

	return tlsPropertyMaterial{
		caPath:            caPath,
		serverCertPath:    serverCertPath,
		serverKeyPath:     serverKeyPath,
		serverCert:        serverCert,
		validClientCert:   validClientCert,
		invalidClientCert: invalidClientCert,
		serverCAPool:      serverCAPool,
	}
}

func verifyClientCertificate(serverTLS *tls.Config, clientCert *tls.Certificate) error {
	if serverTLS == nil {
		return errors.New("server TLS config is nil")
	}
	if serverTLS.ClientAuth == tls.NoClientCert {
		return nil
	}
	if clientCert == nil || len(clientCert.Certificate) == 0 {
		return errors.New("missing client certificate")
	}

	leaf, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		return err
	}
	intermediates := x509.NewCertPool()
	for i := 1; i < len(clientCert.Certificate); i++ {
		parsed, parseErr := x509.ParseCertificate(clientCert.Certificate[i])
		if parseErr != nil {
			return parseErr
		}
		intermediates.AddCert(parsed)
	}

	_, err = leaf.Verify(x509.VerifyOptions{
		Roots:         serverTLS.ClientCAs,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		CurrentTime:   time.Now(),
	})
	return err
}

func newSecurityPropertyServer(t *testing.T, authEnabled bool, validator auth.JWTValidator) *ManagementServer {
	t.Helper()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.ErrorLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}
	r := nethttp.NewRouter()
	healthRegistry := health.NewRegistry()
	healthRegistry.Register(health.NewPingChecker("ready-check"))
	metricsRegistry := metrics.NewRegistry()
	cfg := config.ManagementConfig{
		Port:         9090,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		AuthEnabled:  authEnabled,
	}
	server, err := NewManagementServer(cfg, r, log, healthRegistry, metricsRegistry, validator)
	if err != nil {
		t.Fatalf("failed to create management server: %v", err)
	}
	return server
}

func requiredScopeForEndpoint(endpoint string) string {
	switch endpoint {
	case "/ready":
		return "management:read"
	case "/metrics":
		return "management:metrics"
	case "/swagger":
		return "management:swagger"
	default:
		return ""
	}
}

func performStatus(server *ManagementServer, path, token string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	return rec.Code
}
