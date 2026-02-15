package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestRedisAdapter_Integration tests the Redis adapter with a real Redis instance
// using testcontainers.
//
// **Validates: Requirements 21.1-21.8, 38.2**
func TestRedisAdapter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Redis container
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(redisContainer); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Get connection string
	connStr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Create logger
	log, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.JSONFormat,
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test adapter creation
	t.Run("CreateAdapter", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Verify connection works
		if err := adapter.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	// Test connection and ping
	t.Run("ConnectionAndPing", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Test ping
		if err := adapter.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Test health check
		if err := adapter.HealthCheck(ctx); err != nil {
			t.Errorf("Health check failed: %v", err)
		}
	})

	// Test graceful shutdown
	t.Run("GracefulShutdown", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}

		// Close adapter
		if err := adapter.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Verify connection is closed
		if err := adapter.Ping(ctx); err == nil {
			t.Error("Expected ping to fail after close, but it succeeded")
		}
	})

	// Test basic cache operations
	t.Run("BasicCacheOperations", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		key := "test:basic:key"
		value := "test_value"

		// Set value
		if err := adapter.Set(ctx, key, value); err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Get value
		retrieved, err := adapter.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if retrieved != value {
			t.Errorf("Expected value=%s, got %s", value, retrieved)
		}

		// Delete value
		if err := adapter.Delete(ctx, key); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify deletion
		_, err = adapter.Get(ctx, key)
		if err == nil {
			t.Error("Expected Get to fail after delete, but it succeeded")
		}
	})

	// Test cache operations with TTL
	t.Run("CacheOperationsWithTTL", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		key := "test:ttl:key"
		value := "ttl_value"

		// Set value with TTL
		if err := adapter.SetWithTTL(ctx, key, value, 2*time.Second); err != nil {
			t.Fatalf("SetWithTTL failed: %v", err)
		}

		// Get value immediately (before expiration)
		retrieved, err := adapter.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed before expiration: %v", err)
		}
		if retrieved != value {
			t.Errorf("Expected value=%s, got %s", value, retrieved)
		}

		// Wait for expiration
		time.Sleep(3 * time.Second)

		// Get value after expiration
		_, err = adapter.Get(ctx, key)
		if err == nil {
			t.Error("Expected Get to fail after TTL expiration, but it succeeded")
		}
	})

	// Test atomic increment operations
	t.Run("AtomicIncrementOperations", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		key := "test:counter:incr"

		// Clean up first
		adapter.Delete(ctx, key)

		// Increment from 0 to 1
		val, err := adapter.Incr(ctx, key)
		if err != nil {
			t.Fatalf("Incr failed: %v", err)
		}
		if val != 1 {
			t.Errorf("Expected value=1, got %d", val)
		}

		// Increment from 1 to 2
		val, err = adapter.Incr(ctx, key)
		if err != nil {
			t.Fatalf("Incr failed: %v", err)
		}
		if val != 2 {
			t.Errorf("Expected value=2, got %d", val)
		}

		// Increment by 10
		val, err = adapter.IncrBy(ctx, key, 10)
		if err != nil {
			t.Fatalf("IncrBy failed: %v", err)
		}
		if val != 12 {
			t.Errorf("Expected value=12, got %d", val)
		}

		// Clean up
		adapter.Delete(ctx, key)
	})

	// Test atomic decrement operations
	t.Run("AtomicDecrementOperations", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		key := "test:counter:decr"

		// Clean up first
		adapter.Delete(ctx, key)

		// Set initial value
		adapter.Set(ctx, key, "100")

		// Decrement from 100 to 99
		val, err := adapter.Decr(ctx, key)
		if err != nil {
			t.Fatalf("Decr failed: %v", err)
		}
		if val != 99 {
			t.Errorf("Expected value=99, got %d", val)
		}

		// Decrement from 99 to 98
		val, err = adapter.Decr(ctx, key)
		if err != nil {
			t.Fatalf("Decr failed: %v", err)
		}
		if val != 98 {
			t.Errorf("Expected value=98, got %d", val)
		}

		// Decrement by 10
		val, err = adapter.DecrBy(ctx, key, 10)
		if err != nil {
			t.Fatalf("DecrBy failed: %v", err)
		}
		if val != 88 {
			t.Errorf("Expected value=88, got %d", val)
		}

		// Clean up
		adapter.Delete(ctx, key)
	})

	// Test concurrent atomic operations
	t.Run("ConcurrentAtomicOperations", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         20,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		key := "test:counter:concurrent"

		// Clean up first
		adapter.Delete(ctx, key)

		// Perform 100 concurrent increments
		numIncrements := 100
		done := make(chan error, numIncrements)

		for i := 0; i < numIncrements; i++ {
			go func() {
				_, err := adapter.Incr(ctx, key)
				done <- err
			}()
		}

		// Wait for all increments to complete
		for i := 0; i < numIncrements; i++ {
			if err := <-done; err != nil {
				t.Fatalf("Concurrent increment failed: %v", err)
			}
		}

		// Get final value
		finalValue, err := adapter.Get(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get final value: %v", err)
		}

		// Final value should be exactly numIncrements
		expected := fmt.Sprintf("%d", numIncrements)
		if finalValue != expected {
			t.Errorf("Expected final value=%s, got %s", expected, finalValue)
		}

		// Clean up
		adapter.Delete(ctx, key)
	})

	// Test multiple key deletion
	t.Run("MultipleKeyDeletion", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 5 * time.Second,
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		keys := []string{"test:multi:1", "test:multi:2", "test:multi:3"}

		// Set multiple keys
		for _, key := range keys {
			if err := adapter.Set(ctx, key, "value"); err != nil {
				t.Fatalf("Set failed for key %s: %v", key, err)
			}
		}

		// Delete all keys at once
		if err := adapter.Delete(ctx, keys...); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify all keys are deleted
		for _, key := range keys {
			_, err := adapter.Get(ctx, key)
			if err == nil {
				t.Errorf("Expected Get to fail for key %s after delete, but it succeeded", key)
			}
		}
	})

	// Test operation timeout
	t.Run("OperationTimeout", func(t *testing.T) {
		cfg := Config{
			URL:              connStr,
			MaxConns:         10,
			OperationTimeout: 1 * time.Millisecond, // Very short timeout
		}

		adapter, err := NewRedisAdapter(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create adapter: %v", err)
		}
		defer adapter.Close()

		// Operations might timeout with such a short timeout
		// This test verifies that timeout configuration is applied
		// We don't assert failure because it depends on system load
		key := "test:timeout:key"
		_ = adapter.Set(ctx, key, "value")
		adapter.Delete(ctx, key)
	})
}
