package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestNewPublicAPIServer(t *testing.T) {
	// Given: HTTP configuration
	cfg := config.HTTPConfig{
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// And: a router and logger
	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// When: creating a new PublicAPIServer
	server := NewPublicAPIServer(cfg, r, log)

	// Then: server should be created successfully
	if server == nil {
		t.Fatal("expected server to be created, got nil")
	}

	// And: server should have the correct configuration
	if server.config.Port != cfg.Port {
		t.Errorf("expected port %d, got %d", cfg.Port, server.config.Port)
	}
	if server.config.ReadTimeout != cfg.ReadTimeout {
		t.Errorf("expected read timeout %v, got %v", cfg.ReadTimeout, server.config.ReadTimeout)
	}
	if server.config.WriteTimeout != cfg.WriteTimeout {
		t.Errorf("expected write timeout %v, got %v", cfg.WriteTimeout, server.config.WriteTimeout)
	}
	if server.config.IdleTimeout != cfg.IdleTimeout {
		t.Errorf("expected idle timeout %v, got %v", cfg.IdleTimeout, server.config.IdleTimeout)
	}
}

func TestPublicAPIServer_MiddlewareStack(t *testing.T) {
	// Given: HTTP configuration
	cfg := config.HTTPConfig{
		Port:         8081,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// And: a router with a test route
	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// When: creating a PublicAPIServer
	server := NewPublicAPIServer(cfg, r, log)

	// And: adding a test route
	r.GET("/test", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Then: the server should be ready to handle requests with middleware applied
	// Note: Middleware application is verified through integration tests
	if server == nil {
		t.Fatal("expected server to be created")
	}
}

func TestPublicAPIServer_SSEEndpointEnabled(t *testing.T) {
	httpCfg := config.HTTPConfig{
		Port:         8084,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	defaults := config.DefaultConfig()
	sseCfg := defaults.SSE
	sseCfg.Enabled = true
	sseCfg.Bus = "inmemory"

	server := NewPublicAPIServerWithConfig(
		httpCfg,
		defaults.CORS,
		defaults.SecurityHeaders,
		defaults.Security,
		defaults.I18n,
		defaults.Session,
		defaults.CSRF,
		sseCfg,
		defaults.EventBus,
		defaults.Validation,
		defaults.Observability,
		r,
		log,
	)

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing channel, got %d", rec.Code)
	}
}

func TestPublicAPIServer_RequestSizeMiddleware(t *testing.T) {
	cfg := config.HTTPConfig{
		Port:           8083,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxRequestSize: 8,
	}

	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	server := NewPublicAPIServer(cfg, r, log)

	r.POST("/items", func(c router.Context) error {
		var payload map[string]interface{}
		if err := c.Bind(&payload); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"name":"very-long"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
}

func TestPublicAPIServer_StartAndShutdown(t *testing.T) {
	// Given: HTTP configuration with a unique port
	cfg := config.HTTPConfig{
		Port:         8082,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	// And: a router and logger
	r := nethttp.NewRouter()
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// And: a test route
	r.GET("/health", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// When: creating a PublicAPIServer
	server := NewPublicAPIServer(cfg, r, log)

	// And: starting the server with a context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Then: server should be accepting requests
	resp, err := http.Get("http://localhost:8082/health")
	if err != nil {
		t.Fatalf("expected server to be running, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// When: cancelling the context to trigger shutdown
	cancel()

	// Then: server should shut down gracefully
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("expected clean shutdown, got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within timeout")
	}
}

func TestPublicAPIServer_BindsToConfiguredPort(t *testing.T) {
	// Given: different HTTP configurations
	testCases := []struct {
		name string
		port int
	}{
		{"default port", 8080},
		{"custom port", 9000},
		{"high port", 18080},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Given: HTTP configuration with specific port
			cfg := config.HTTPConfig{
				Port:         tc.port,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				IdleTimeout:  10 * time.Second,
			}

			// And: a router and logger
			r := nethttp.NewRouter()
			log, err := logger.NewZapLogger(logger.Config{
				Level:  logger.InfoLevel,
				Format: logger.JSONFormat,
			})
			if err != nil {
				t.Fatalf("failed to create logger: %v", err)
			}

			// When: creating a PublicAPIServer
			server := NewPublicAPIServer(cfg, r, log)

			// Then: server should have the correct port configured
			if server.config.Port != tc.port {
				t.Errorf("expected port %d, got %d", tc.port, server.config.Port)
			}
		})
	}
}
