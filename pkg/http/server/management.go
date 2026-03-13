// Package server provides HTTP server implementations with graceful startup and shutdown.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/featureflag"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/http/authentication"
	"github.com/nimburion/nimburion/pkg/http/authorization"
	"github.com/nimburion/nimburion/pkg/http/middleware/logging"
	"github.com/nimburion/nimburion/pkg/http/middleware/recovery"
	"github.com/nimburion/nimburion/pkg/http/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/http/router"
	serverconfig "github.com/nimburion/nimburion/pkg/http/server/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
)

// ManagementServer wraps Server for management and admin traffic.
// It provides a separate HTTP server for health checks, metrics, and admin endpoints
// on a different port from the public API server.
//
// Requirements: 2.3, 2.4, 2.5, 2.6
type ManagementServer struct {
	*Server
	healthRegistry  *health.Registry
	metricsRegistry *metrics.Registry
}

// NewManagementServer creates a new ManagementServer instance.
// It configures the server with the management configuration and sets up
// standard management endpoints:
// - /health: Liveness check (always returns 200)
// - /ready: Readiness check (checks dependencies)
// - /metrics: Prometheus metrics endpoint
//
// The middleware stack includes:
// 1. Request ID - generates/extracts request IDs for correlation
// 2. Logging - logs HTTP requests with structured data
// 3. Recovery - catches panics and returns 500 errors
//
// Requirements: 2.3, 2.4, 2.5, 30.1, 30.2, 30.3, 13.1, 13.7
func NewManagementServer(
	cfg serverconfig.ManagementConfig,
	r router.Router,
	log logger.Logger,
	healthRegistry *health.Registry,
	metricsRegistry *metrics.Registry,
	validator auth.JWTValidator,
) (*ManagementServer, error) {
	if healthRegistry == nil {
		healthRegistry = health.NewRegistry()
	}
	if metricsRegistry == nil {
		metricsRegistry = metrics.NewRegistry()
	}

	// Apply standard middleware stack (lighter than public API)
	r.Use(
		requestid.RequestID(),
		logging.WithConfig(log, logging.DefaultConfig()),
		recovery.Recovery(log),
	)
	if cfg.AuthEnabled && validator == nil {
		return nil, fmt.Errorf("management auth is enabled but JWT validator is nil")
	}

	if !cfg.AuthEnabled {
		log.Warn("management server is running without authentication: /metrics, /swagger, and /ready are publicly accessible")
	}

	// Create server config from management config
	serverCfg := Config{
		Port:         cfg.Port,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second, // Default idle timeout for management server
		RequireTLS:   cfg.RequireTLS,
	}
	if cfg.MTLSEnabled {
		tlsConfig, err := LoadTLSConfig(cfg.TLSCertFile, cfg.TLSKeyFile, cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load management mTLS config: %w", err)
		}
		serverCfg.TLSConfig = tlsConfig
		log.Info("management mTLS enabled")
	}

	// Create base server
	baseServer := NewServer(serverCfg, r, log)

	mgmtServer := &ManagementServer{
		Server:          baseServer,
		healthRegistry:  healthRegistry,
		metricsRegistry: metricsRegistry,
	}

	// Register management endpoints
	mgmtServer.registerEndpoints(r, cfg.AuthEnabled, cfg.AllowlistCIDRs, validator)

	return mgmtServer, nil
}

// registerEndpoints registers the standard management endpoints.
// Requirements: 30.1, 30.2, 30.3, 13.1, 13.7
func (s *ManagementServer) registerEndpoints(r router.Router, authEnabled bool, allowlistCIDRs []string, validator auth.JWTValidator) {
	// Health endpoint - liveness check (always returns 200)
	// Requirements: 30.1, 30.3
	r.GET("/health", s.handleHealth)

	// Ready endpoint - readiness check (checks dependencies)
	// Requirements: 30.2, 30.4, 30.5, 30.6
	readyMiddleware := append(managementIPAllowlist(allowlistCIDRs), managementSecurityMiddleware(authEnabled, validator, "management:read")...)
	r.GET("/ready", s.handleReady, readyMiddleware...)

	// Metrics endpoint - Prometheus metrics
	// Requirements: 13.1, 13.7
	metricsMiddleware := append(managementIPAllowlist(allowlistCIDRs), managementSecurityMiddleware(authEnabled, validator, "management:metrics")...)
	r.GET("/metrics", s.handleMetrics, metricsMiddleware...)

	// Swagger endpoint - secured when management auth is enabled.
	swaggerMiddleware := append(managementIPAllowlist(allowlistCIDRs), managementSecurityMiddleware(authEnabled, validator, "management:swagger")...)
	r.GET("/swagger", s.handleSwagger, swaggerMiddleware...)
	r.GET("/swagger/", s.handleSwagger, swaggerMiddleware...)
}

func managementIPAllowlist(cidrs []string) []router.MiddlewareFunc {
	if len(cidrs) == 0 {
		return nil
	}

	allowed := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		allowed = append(allowed, network)
	}
	if len(allowed) == 0 {
		return nil
	}

	return []router.MiddlewareFunc{func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
			if err != nil {
				host = c.Request().RemoteAddr
			}
			ip := net.ParseIP(strings.TrimSpace(host))
			if ip == nil {
				return c.JSON(http.StatusForbidden, map[string]interface{}{"error": "access denied"})
			}
			for _, network := range allowed {
				if network.Contains(ip) {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, map[string]interface{}{"error": "access denied"})
		}
	}}
}

func managementSecurityMiddleware(authEnabled bool, validator auth.JWTValidator, scopes ...string) []router.MiddlewareFunc {
	if !authEnabled {
		return nil
	}

	return []router.MiddlewareFunc{
		authentication.Authenticate(validator),
		authorization.RequireScopes(scopes...),
	}
}

// handleHealth handles the /health endpoint.
// This is a liveness check that always returns HTTP 200 to indicate the service is alive.
// It does not check dependencies.
//
// Requirements: 30.1, 30.3
func (s *ManagementServer) handleHealth(c router.Context) error {
	if s.healthRegistry != nil {
		result, err := s.healthRegistry.CheckOne(c.Request().Context(), featureflag.CheckNameRuntimeLiveness)
		if err == nil {
			statusCode := http.StatusOK
			if result.Status == health.StatusUnhealthy {
				statusCode = http.StatusServiceUnavailable
			}
			return c.JSON(statusCode, result)
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": health.StatusHealthy,
	})
}

// handleReady handles the /ready endpoint.
// This is a readiness check that verifies all critical dependencies are healthy.
// Returns HTTP 200 if ready to handle traffic, HTTP 503 if not ready.
//
// Requirements: 30.2, 30.4, 30.5, 30.6
func (s *ManagementServer) handleReady(c router.Context) error {
	ctx := c.Request().Context()

	// Run all registered health checks
	result := s.healthRegistry.Check(ctx)

	// Return 503 if any dependency is unhealthy
	// Requirements: 30.6
	if !result.IsReady() {
		return c.JSON(http.StatusServiceUnavailable, result)
	}

	// Return 200 if all dependencies are healthy
	// Requirements: 30.4
	return c.JSON(http.StatusOK, result)
}

// handleMetrics handles the /metrics endpoint.
// Exposes Prometheus metrics in the standard Prometheus text format.
//
// Requirements: 13.1, 13.7
func (s *ManagementServer) handleMetrics(c router.Context) error {
	// Use the Prometheus handler to serve metrics
	s.metricsRegistry.Handler().ServeHTTP(c.Response(), c.Request())
	return nil
}

func (s *ManagementServer) handleSwagger(c router.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "available",
		"message": "swagger route is reserved for the OpenAPI/Swagger handler",
	})
}

// Start starts the management server.
// It delegates to the underlying Server's Start method.
func (s *ManagementServer) Start(ctx context.Context) error {
	return s.Server.Start(ctx)
}

// Shutdown gracefully shuts down the management server.
// It delegates to the underlying Server's Shutdown method.
func (s *ManagementServer) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

// Router returns the management server's router instance
func (s *ManagementServer) Router() router.Router {
	return s.router
}
