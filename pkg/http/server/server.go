// Package server provides HTTP server implementations with graceful startup and shutdown.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// Server wraps http.Server with configurable timeouts and graceful lifecycle management.
// It supports graceful startup, shutdown with timeout, and context cancellation.
type Server struct {
	httpServer *http.Server
	router     router.Router
	logger     logger.Logger
	config     Config
}

// Config holds configuration for the HTTP server.
type Config struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	TLSConfig    *tls.Config
	RequireTLS   bool
}

// NewServer creates a new Server instance with the provided configuration.
// The router parameter defines the HTTP routing behavior.
// The logger parameter is used for structured logging of server lifecycle events.
func NewServer(cfg Config, router router.Router, logger logger.Logger) *Server {
	return &Server{
		router: router,
		logger: logger,
		config: cfg,
	}
}

// Start initializes and starts the HTTP server.
// It creates an http.Server with configured timeouts and begins listening for requests.
// The method runs the server in a goroutine and monitors for context cancellation.
//
// If the context is canceled, Start will call Shutdown to gracefully stop the server.
// Returns an error if the server fails to start or if shutdown fails.
func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.config.RequireTLS && s.config.TLSConfig == nil {
		return fmt.Errorf("server RequireTLS is set but TLSConfig is nil: refusing to start in plain HTTP")
	}
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
		TLSConfig:    s.config.TLSConfig,
	}

	if s.config.TLSConfig == nil {
		s.logger.Warn("starting server without TLS", "port", s.config.Port)
	} else {
		s.logger.Info("starting server", "port", s.config.Port, "tls_enabled", true)
	}

	// Channel to capture server startup errors
	errChan := make(chan error, 1)

	// Start server in goroutine
	go func() {
		var err error
		if s.config.TLSConfig != nil {
			err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for either startup error or context cancellation
	select {
	case err := <-errChan:
		return fmt.Errorf("server failed to start: %w", err)
	case <-ctx.Done():
		// Context canceled, initiate graceful shutdown
		shutdownCtx, cancel := shutdownContextFromParent(ctx)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// Shutdown gracefully stops the HTTP server with a timeout.
// It stops accepting new connections and waits for in-flight requests to complete.
// The shutdown process has a 30-second timeout. If the timeout is exceeded,
// the server will be forcefully terminated.
//
// Returns an error if the shutdown process fails.
func (s *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.logger.Info(fmt.Sprintf("shutting down server on %s", s.httpServer.Addr))

	// Create a timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info(fmt.Sprintf("server on %s shutdown complete ", s.httpServer.Addr))

	return nil
}

func shutdownContextFromParent(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.Background(), func() {}
	}

	base := context.WithoutCancel(parent)
	if deadline, ok := parent.Deadline(); ok {
		// #nosec G118 -- the derived cancel function is returned to the shutdown caller.
		return context.WithDeadline(base, deadline)
	}
	return base, func() {}
}
