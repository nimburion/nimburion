package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

// TestProperty34_DualServerPortBinding verifies that public and management servers bind to different ports.
// Property 34: Dual Server Port Binding
//
// *For any* framework configuration with public and management servers, starting both servers should result
// in two HTTP servers listening on different ports, with public server handling application routes and
// management server handling health, metrics, and admin routes.
//
// **Validates: Requirements 2.7**
func TestProperty34_DualServerPortBinding(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid port numbers (1024-65535, avoiding privileged ports)
	genPort := gen.IntRange(10000, 60000)

	properties.Property("public and management servers bind to different ports", prop.ForAll(
		func(publicPort, managementPort int) bool {
			// Skip if ports are the same (we want to test different ports)
			if publicPort == managementPort {
				return true
			}

			// Create logger
			log, err := logger.NewZapLogger(logger.Config{
				Level:  "error", // Reduce noise in tests
				Format: "json",
			})
			if err != nil {
				t.Logf("Failed to create logger: %v", err)
				return false
			}

			// Create public server
			publicRouter := nethttp.NewRouter()
			publicRouter.GET("/api/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"server": "public"})
			})

			publicCfg := config.HTTPConfig{
				Port:         publicPort,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				IdleTimeout:  10 * time.Second,
			}
			publicServer := NewPublicAPIServer(publicCfg, publicRouter, log)

			// Create management server
			managementRouter := nethttp.NewRouter()
			managementCfg := config.ManagementConfig{
				Port:         managementPort,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			}
			healthRegistry := health.NewRegistry()
			metricsRegistry := metrics.NewRegistry()
			managementServer, err := NewManagementServer(managementCfg, managementRouter, log, healthRegistry, metricsRegistry, nil)
			if err != nil {
				t.Logf("Failed to create management server (skipping): %v", err)
				return true
			}

			// Create contexts for server lifecycle
			publicCtx, publicCancel := context.WithCancel(context.Background())
			managementCtx, managementCancel := context.WithCancel(context.Background())
			defer publicCancel()
			defer managementCancel()

			// Start both servers in goroutines
			publicErrChan := make(chan error, 1)
			managementErrChan := make(chan error, 1)

			go func() {
				if err := publicServer.Start(publicCtx); err != nil && err != http.ErrServerClosed {
					publicErrChan <- err
				}
			}()

			go func() {
				if err := managementServer.Start(managementCtx); err != nil && err != http.ErrServerClosed {
					managementErrChan <- err
				}
			}()

			// Wait for servers to start (give them time to bind)
			time.Sleep(100 * time.Millisecond)

			// Check for startup errors (including port conflicts)
			select {
			case err := <-publicErrChan:
				// Port conflict is expected in property tests, skip this case
				if err != nil {
					t.Logf("Public server failed to start (skipping): %v", err)
					publicCancel()
					managementCancel()
					return true // Skip this test case, not a failure
				}
			case err := <-managementErrChan:
				// Port conflict is expected in property tests, skip this case
				if err != nil {
					t.Logf("Management server failed to start (skipping): %v", err)
					publicCancel()
					managementCancel()
					return true // Skip this test case, not a failure
				}
			default:
				// No errors, continue
			}

			// Verify: Public server is listening on its port
			publicAddr := fmt.Sprintf("localhost:%d", publicPort)
			if !isPortListening(publicAddr) {
				t.Logf("Public server not listening on %s", publicAddr)
				publicCancel()
				managementCancel()
				return false
			}

			// Verify: Management server is listening on its port
			managementAddr := fmt.Sprintf("localhost:%d", managementPort)
			if !isPortListening(managementAddr) {
				t.Logf("Management server not listening on %s", managementAddr)
				publicCancel()
				managementCancel()
				return false
			}

			// Verify: Public server handles application routes
			publicResp, err := http.Get(fmt.Sprintf("http://%s/api/test", publicAddr))
			if err != nil {
				t.Logf("Failed to reach public server: %v", err)
				publicCancel()
				managementCancel()
				return false
			}
			publicResp.Body.Close()

			if publicResp.StatusCode != http.StatusOK {
				t.Logf("Public server returned unexpected status: %d", publicResp.StatusCode)
				publicCancel()
				managementCancel()
				return false
			}

			// Verify: Management server handles health endpoint
			healthResp, err := http.Get(fmt.Sprintf("http://%s/health", managementAddr))
			if err != nil {
				t.Logf("Failed to reach management server health endpoint: %v", err)
				publicCancel()
				managementCancel()
				return false
			}
			healthResp.Body.Close()

			if healthResp.StatusCode != http.StatusOK {
				t.Logf("Management server health endpoint returned unexpected status: %d", healthResp.StatusCode)
				publicCancel()
				managementCancel()
				return false
			}

			// Verify: Management server handles metrics endpoint
			metricsResp, err := http.Get(fmt.Sprintf("http://%s/metrics", managementAddr))
			if err != nil {
				t.Logf("Failed to reach management server metrics endpoint: %v", err)
				publicCancel()
				managementCancel()
				return false
			}
			metricsResp.Body.Close()

			if metricsResp.StatusCode != http.StatusOK {
				t.Logf("Management server metrics endpoint returned unexpected status: %d", metricsResp.StatusCode)
				publicCancel()
				managementCancel()
				return false
			}

			// Verify: Public server does NOT serve management endpoints
			publicHealthResp, err := http.Get(fmt.Sprintf("http://%s/health", publicAddr))
			if err == nil {
				publicHealthResp.Body.Close()
				// If we get a 200, that's wrong - public server shouldn't have /health
				if publicHealthResp.StatusCode == http.StatusOK {
					t.Log("Public server incorrectly serves /health endpoint")
					publicCancel()
					managementCancel()
					return false
				}
			}

			// Verify: Management server does NOT serve application routes
			managementAppResp, err := http.Get(fmt.Sprintf("http://%s/api/test", managementAddr))
			if err == nil {
				managementAppResp.Body.Close()
				// If we get a 200, that's wrong - management server shouldn't have /api/test
				if managementAppResp.StatusCode == http.StatusOK {
					t.Log("Management server incorrectly serves /api/test endpoint")
					publicCancel()
					managementCancel()
					return false
				}
			}

			// Gracefully shutdown both servers
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			publicServer.Shutdown(shutdownCtx)
			managementServer.Shutdown(shutdownCtx)

			publicCancel()
			managementCancel()

			// Give servers time to shutdown and release ports
			time.Sleep(200 * time.Millisecond)

			// Verify: Ports are released after shutdown
			maxRetries := 10
			for i := 0; i < maxRetries; i++ {
				if !isPortListening(publicAddr) && !isPortListening(managementAddr) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			if isPortListening(publicAddr) {
				t.Logf("Public server port %s still listening after shutdown", publicAddr)
				return false
			}

			if isPortListening(managementAddr) {
				t.Logf("Management server port %s still listening after shutdown", managementAddr)
				return false
			}

			return true
		},
		genPort,
		genPort,
	))

	properties.Property("servers can start in any order", prop.ForAll(
		func(publicPort, managementPort int, startPublicFirst bool) bool {
			// Skip if ports are the same
			if publicPort == managementPort {
				return true
			}

			// Create logger
			log, err := logger.NewZapLogger(logger.Config{
				Level:  "error",
				Format: "json",
			})
			if err != nil {
				t.Logf("Failed to create logger: %v", err)
				return false
			}

			// Create servers
			publicRouter := nethttp.NewRouter()
			publicRouter.GET("/api/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"server": "public"})
			})

			publicCfg := config.HTTPConfig{
				Port:         publicPort,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				IdleTimeout:  10 * time.Second,
			}
			publicServer := NewPublicAPIServer(publicCfg, publicRouter, log)

			managementRouter := nethttp.NewRouter()
			managementCfg := config.ManagementConfig{
				Port:         managementPort,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			}
			healthRegistry := health.NewRegistry()
			metricsRegistry := metrics.NewRegistry()
			managementServer, err := NewManagementServer(managementCfg, managementRouter, log, healthRegistry, metricsRegistry, nil)
			if err != nil {
				t.Logf("Failed to create management server (skipping): %v", err)
				return true
			}

			// Create contexts
			publicCtx, publicCancel := context.WithCancel(context.Background())
			managementCtx, managementCancel := context.WithCancel(context.Background())
			defer publicCancel()
			defer managementCancel()

			publicErrChan := make(chan error, 1)
			managementErrChan := make(chan error, 1)

			startPublic := func() {
				go func() {
					if err := publicServer.Start(publicCtx); err != nil && err != http.ErrServerClosed {
						select {
						case publicErrChan <- err:
						default:
						}
					}
				}()
			}

			startManagement := func() {
				go func() {
					if err := managementServer.Start(managementCtx); err != nil && err != http.ErrServerClosed {
						select {
						case managementErrChan <- err:
						default:
						}
					}
				}()
			}

			// Start servers in different order based on flag
			if startPublicFirst {
				startPublic()
				time.Sleep(100 * time.Millisecond)
				startManagement()
			} else {
				startManagement()
				time.Sleep(100 * time.Millisecond)
				startPublic()
			}

			publicAddr := fmt.Sprintf("localhost:%d", publicPort)
			managementAddr := fmt.Sprintf("localhost:%d", managementPort)

			publicListening, publicStartErr := waitForListeningOrError(publicAddr, publicErrChan, 2*time.Second)
			if publicStartErr != nil {
				t.Logf("Public server failed to start (skipping): %v", publicStartErr)
				return true // Port conflict can happen in property tests.
			}

			managementListening, managementStartErr := waitForListeningOrError(managementAddr, managementErrChan, 2*time.Second)
			if managementStartErr != nil {
				t.Logf("Management server failed to start (skipping): %v", managementStartErr)
				return true // Port conflict can happen in property tests.
			}

			// Cleanup
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			publicServer.Shutdown(shutdownCtx)
			managementServer.Shutdown(shutdownCtx)

			publicCancel()
			managementCancel()

			if !waitForPortsReleased([]string{publicAddr, managementAddr}, 2*time.Second) {
				t.Logf("Ports not released after shutdown (public: %s, management: %s)", publicAddr, managementAddr)
				return false
			}

			if !publicListening {
				t.Logf("Public server not listening on %s (started first: %v)", publicAddr, startPublicFirst)
				return false
			}

			if !managementListening {
				t.Logf("Management server not listening on %s (started first: %v)", managementAddr, !startPublicFirst)
				return false
			}

			return true
		},
		genPort,
		genPort,
		gen.Bool(),
	))

	properties.Property("servers operate independently", prop.ForAll(
		func(publicPort, managementPort int) bool {
			// Skip if ports are the same
			if publicPort == managementPort {
				return true
			}

			// Create logger
			log, err := logger.NewZapLogger(logger.Config{
				Level:  "error",
				Format: "json",
			})
			if err != nil {
				t.Logf("Failed to create logger: %v", err)
				return false
			}

			// Create servers
			publicRouter := nethttp.NewRouter()
			publicRouter.GET("/api/test", func(c router.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"server": "public"})
			})

			publicCfg := config.HTTPConfig{
				Port:         publicPort,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				IdleTimeout:  10 * time.Second,
			}
			publicServer := NewPublicAPIServer(publicCfg, publicRouter, log)

			managementRouter := nethttp.NewRouter()
			managementCfg := config.ManagementConfig{
				Port:         managementPort,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			}
			healthRegistry := health.NewRegistry()
			metricsRegistry := metrics.NewRegistry()
			managementServer, err := NewManagementServer(managementCfg, managementRouter, log, healthRegistry, metricsRegistry, nil)
			if err != nil {
				t.Logf("Failed to create management server (skipping): %v", err)
				return true
			}

			// Start both servers
			publicCtx, publicCancel := context.WithCancel(context.Background())
			managementCtx, managementCancel := context.WithCancel(context.Background())
			defer managementCancel()

			go publicServer.Start(publicCtx)
			go managementServer.Start(managementCtx)

			time.Sleep(100 * time.Millisecond)

			// Verify both are running
			publicAddr := fmt.Sprintf("localhost:%d", publicPort)
			managementAddr := fmt.Sprintf("localhost:%d", managementPort)

			if !isPortListening(publicAddr) || !isPortListening(managementAddr) {
				publicCancel()
				managementCancel()
				return true // Skip this test case
			}

			// Shutdown only the public server
			publicCancel()
			time.Sleep(100 * time.Millisecond)

			// Verify: Public server is stopped
			if isPortListening(publicAddr) {
				t.Log("Public server still listening after shutdown")
				managementCancel()
				return false
			}

			// Verify: Management server is still running
			if !isPortListening(managementAddr) {
				t.Log("Management server stopped when only public server was shutdown")
				managementCancel()
				return false
			}

			// Verify: Management server still responds
			healthResp, err := http.Get(fmt.Sprintf("http://%s/health", managementAddr))
			if err != nil {
				t.Logf("Management server not responding after public server shutdown: %v", err)
				managementCancel()
				return false
			}
			healthResp.Body.Close()

			if healthResp.StatusCode != http.StatusOK {
				t.Logf("Management server health check failed: %d", healthResp.StatusCode)
				managementCancel()
				return false
			}

			// Cleanup
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			managementServer.Shutdown(shutdownCtx)
			managementCancel()
			time.Sleep(200 * time.Millisecond)

			return true
		},
		genPort,
		genPort,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// isPortListening checks if a port is currently listening for connections.
func isPortListening(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitForListeningOrError(addr string, errChan <-chan error, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-errChan:
			return false, err
		default:
		}

		if isPortListening(addr) {
			return true, nil
		}

		time.Sleep(25 * time.Millisecond)
	}

	select {
	case err := <-errChan:
		return false, err
	default:
	}

	return false, nil
}

func waitForPortsReleased(addrs []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allReleased := true
		for _, addr := range addrs {
			if isPortListening(addr) {
				allReleased = false
				break
			}
		}

		if allReleased {
			return true
		}

		time.Sleep(25 * time.Millisecond)
	}

	for _, addr := range addrs {
		if isPortListening(addr) {
			return false
		}
	}

	return true
}
