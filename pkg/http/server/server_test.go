package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/http/router/nethttp"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// TestServerStartAndShutdown tests that the server starts and shuts down gracefully
func TestServerStartAndShutdown(t *testing.T) {
	// Create a test router
	r := nethttp.NewRouter()
	r.GET("/health", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Create a test logger
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Create server config
	cfg := Config{
		Port:         8081,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	// Create server
	srv := NewServer(cfg, r, log)

	// Start server in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to verify server is running
	resp, err := http.Get("http://localhost:8081/health")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Cancel context to trigger shutdown
	cancel()

	// Wait for shutdown to complete
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("server shutdown failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server shutdown timed out")
	}
}

// TestConfigurableTimeouts tests that server respects configured timeouts
func TestConfigurableTimeouts(t *testing.T) {
	r := nethttp.NewRouter()
	r.GET("/test", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	cfg := Config{
		Port:         8082,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		IdleTimeout:  2 * time.Second,
	}

	srv := NewServer(cfg, r, log)

	// Verify server config is set correctly
	if srv.config.Port != 8082 {
		t.Errorf("expected port 8082, got %d", srv.config.Port)
	}
	if srv.config.ReadTimeout != 1*time.Second {
		t.Errorf("expected read timeout 1s, got %v", srv.config.ReadTimeout)
	}
	if srv.config.WriteTimeout != 1*time.Second {
		t.Errorf("expected write timeout 1s, got %v", srv.config.WriteTimeout)
	}
	if srv.config.IdleTimeout != 2*time.Second {
		t.Errorf("expected idle timeout 2s, got %v", srv.config.IdleTimeout)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify http.Server has correct timeouts
	if srv.httpServer.ReadTimeout != 1*time.Second {
		t.Errorf("expected http.Server read timeout 1s, got %v", srv.httpServer.ReadTimeout)
	}
	if srv.httpServer.WriteTimeout != 1*time.Second {
		t.Errorf("expected http.Server write timeout 1s, got %v", srv.httpServer.WriteTimeout)
	}
	if srv.httpServer.IdleTimeout != 2*time.Second {
		t.Errorf("expected http.Server idle timeout 2s, got %v", srv.httpServer.IdleTimeout)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestServerContextCancellation tests that server responds to context cancellation
func TestServerContextCancellation(t *testing.T) {
	r := nethttp.NewRouter()
	r.GET("/test", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	cfg := Config{
		Port:         8083,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	srv := NewServer(cfg, r, log)

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Verify server shuts down
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("expected nil error on graceful shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server did not shut down after context cancellation")
	}
}

func TestShutdownContextFromParent_PreservesDeadlineWithoutCancellation(t *testing.T) {
	parent, cancelParent := context.WithTimeout(context.Background(), time.Second)
	cancelParent()

	shutdownCtx, cancel := shutdownContextFromParent(parent)
	defer cancel()

	if shutdownCtx.Err() != nil {
		t.Fatalf("expected shutdown context to ignore parent cancellation, got %v", shutdownCtx.Err())
	}
	parentDeadline, ok := parent.Deadline()
	if !ok {
		t.Fatal("expected parent deadline")
	}
	gotDeadline, ok := shutdownCtx.Deadline()
	if !ok {
		t.Fatal("expected shutdown deadline to be preserved")
	}
	if !gotDeadline.Equal(parentDeadline) {
		t.Fatalf("expected preserved deadline %v, got %v", parentDeadline, gotDeadline)
	}
}

// TestServerShutdownTimeout tests that shutdown respects timeout
func TestServerShutdownTimeout(t *testing.T) {
	r := nethttp.NewRouter()

	// Handler that takes longer than shutdown timeout
	r.GET("/slow", func(c router.Context) error {
		time.Sleep(35 * time.Second) // Longer than 30s shutdown timeout
		return c.JSON(http.StatusOK, map[string]string{"message": "done"})
	})

	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	cfg := Config{
		Port:         8084,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	srv := NewServer(cfg, r, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Start a slow request
	go func() {
		_, _ = http.Get("http://localhost:8084/slow")
	}()

	// Give the request time to start
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	start := time.Now()
	if shutdownErr := srv.Shutdown(context.Background()); shutdownErr != nil {
		t.Logf("shutdown returned error (expected during timeout path): %v", shutdownErr)
	}
	duration := time.Since(start)

	// Shutdown should complete within ~30 seconds (the timeout)
	// We allow some buffer for processing
	if duration > 35*time.Second {
		t.Errorf("shutdown took too long: %v", duration)
	}

	// Note: We don't check for error here because the shutdown timeout
	// may or may not produce an error depending on timing
}

// TestServerMultipleRequests tests that server handles multiple concurrent requests
func TestServerMultipleRequests(t *testing.T) {
	r := nethttp.NewRouter()
	r.GET("/test", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	cfg := Config{
		Port:         8085,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	srv := NewServer(cfg, r, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = srv.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make multiple concurrent requests
	const numRequests = 10
	errChan := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			resp, err := http.Get("http://localhost:8085/test")
			if err != nil {
				errChan <- fmt.Errorf("request %d failed: %w", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("request %d got status %d", id, resp.StatusCode)
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				errChan <- fmt.Errorf("request %d failed to read body: %w", id, err)
				return
			}

			if len(body) == 0 {
				errChan <- fmt.Errorf("request %d got empty body", id)
				return
			}

			errChan <- nil
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("request timed out")
		}
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestServerStart_WithTLSConfigUsesHTTPS(t *testing.T) {
	r := nethttp.NewRouter()
	r.GET("/health", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	dir := t.TempDir()
	_, certPath, keyPath, _, _ := writeTestCertificates(t, dir)
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("failed to load certificate pair: %v", err)
	}

	cfg := Config{
		Port:         8086,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	srv := NewServer(cfg, r, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	time.Sleep(150 * time.Millisecond)

	// Plain HTTP must not successfully reach the TLS endpoint.
	if resp, getErr := http.Get("http://localhost:8086/health"); getErr == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected plain HTTP request to fail or be rejected when TLS is enabled")
		}
	}

	httpsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := httpsClient.Get("https://localhost:8086/health")
	if err != nil {
		t.Fatalf("expected HTTPS request to succeed, got: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("expected graceful shutdown, got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server shutdown timed out")
	}
}

// TestServerStartError tests handling of server startup errors
func TestServerStartError(t *testing.T) {
	r := nethttp.NewRouter()
	r.GET("/test", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Start first server on port 8086
	cfg1 := Config{
		Port:         8086,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	srv1 := NewServer(cfg1, r, log)

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	go func() {
		_ = srv1.Start(ctx1)
	}()

	// Wait for first server to start
	time.Sleep(100 * time.Millisecond)

	// Try to start second server on same port (should fail)
	cfg2 := Config{
		Port:         8086, // Same port
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	srv2 := NewServer(cfg2, r, log)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	err = srv2.Start(ctx2)

	// We expect an error because the port is already in use
	// The error could be either a startup error or a context timeout
	if err == nil {
		t.Error("expected error when starting server on occupied port, got nil")
	}

	cancel1()
	time.Sleep(100 * time.Millisecond)
}

type captureLogger struct {
	warns []string
}

func (l *captureLogger) Debug(string, ...any) {}
func (l *captureLogger) Info(string, ...any)  {}
func (l *captureLogger) Warn(msg string, _ ...any) {
	l.warns = append(l.warns, msg)
}
func (l *captureLogger) Error(string, ...any)                      {}
func (l *captureLogger) With(...any) logger.Logger                 { return l }
func (l *captureLogger) WithContext(context.Context) logger.Logger { return l }

func TestServerStart_RequireTLSWithoutConfigFails(t *testing.T) {
	r := nethttp.NewRouter()
	log := &captureLogger{}
	srv := NewServer(Config{Port: 0, RequireTLS: true}, r, log)

	err := srv.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when RequireTLS is true and TLSConfig is nil")
	}
}

func TestServerStart_WithoutRequireTLSStarts(t *testing.T) {
	r := nethttp.NewRouter()
	r.GET("/health", func(c router.Context) error { return c.JSON(http.StatusOK, map[string]string{"ok": "1"}) })
	log := &captureLogger{}
	srv := NewServer(Config{Port: 8087, RequireTLS: false}, r, log)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("expected start/shutdown without error, got %v", err)
	}
	if len(log.warns) == 0 {
		t.Fatal("expected warning log when starting without TLS")
	}
}
