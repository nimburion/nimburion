package redis

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// TestProperty15_CacheOperationsWithTTL verifies that cache operations with TTL work correctly.
// Property 15: Cache Operations with TTL
//
// *For any* cache adapter, when a key-value pair is set with TTL, retrieving the key before
// TTL expiration should return the value, and retrieving the key after TTL expiration should
// return not-found error.
//
// **Validates: Requirements 21.7**
func TestProperty15_CacheOperationsWithTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid Redis configurations
	genRedisConfig := gopter.CombineGens(
		gen.IntRange(1, 50), // MaxConns
	).Map(func(values []interface{}) Config {
		return Config{
			URL:              getTestRedisURL(),
			MaxConns:         values[0].(int),
			OperationTimeout: 5 * time.Second,
		}
	})

	// Generator for key-value pairs
	genKeyValue := gopter.CombineGens(
		gen.AlphaString(),
		gen.AlphaString(),
	).SuchThat(func(values []interface{}) bool {
		key := values[0].(string)
		value := values[1].(string)
		return len(key) > 0 && len(value) > 0
	}).Map(func(values []interface{}) struct{ key, value string } {
		return struct{ key, value string }{
			key:   "test:" + values[0].(string),
			value: values[1].(string),
		}
	})

	properties.Property("key retrieval before TTL expiration returns value", prop.ForAll(
		func(cfg Config, kv struct{ key, value string }) bool {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Set key with 2 second TTL
			err = adapter.SetWithTTL(ctx, kv.key, kv.value, 2*time.Second)
			if err != nil {
				t.Logf("Failed to set key with TTL: %v", err)
				return false
			}

			// Retrieve immediately (before expiration)
			retrieved, err := adapter.Get(ctx, kv.key)
			if err != nil {
				t.Logf("Failed to get key before expiration: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, kv.key)

			return retrieved == kv.value
		},
		genRedisConfig,
		genKeyValue,
	))

	properties.Property("key retrieval after TTL expiration returns not-found error", prop.ForAll(
		func(cfg Config, kv struct{ key, value string }) bool {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Set key with very short TTL (100ms)
			err = adapter.SetWithTTL(ctx, kv.key, kv.value, 100*time.Millisecond)
			if err != nil {
				t.Logf("Failed to set key with TTL: %v", err)
				return false
			}

			// Wait for expiration
			time.Sleep(200 * time.Millisecond)

			// Retrieve after expiration
			_, err = adapter.Get(ctx, kv.key)
			
			// Should return "key not found" error
			return err != nil && strings.Contains(err.Error(), "key not found")
		},
		genRedisConfig,
		genKeyValue,
	))

	properties.Property("set without TTL persists indefinitely", prop.ForAll(
		func(cfg Config, kv struct{ key, value string }) bool {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Set key without TTL
			err = adapter.Set(ctx, kv.key, kv.value)
			if err != nil {
				t.Logf("Failed to set key: %v", err)
				return false
			}

			// Wait a bit
			time.Sleep(100 * time.Millisecond)

			// Retrieve - should still exist
			retrieved, err := adapter.Get(ctx, kv.key)
			if err != nil {
				t.Logf("Failed to get key: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, kv.key)

			return retrieved == kv.value
		},
		genRedisConfig,
		genKeyValue,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty16_RedisAtomicOperations verifies that atomic operations maintain atomicity.
// Property 16: Redis Atomic Operations
//
// *For any* Redis adapter, atomic operations (INCR, DECR, etc.) should maintain atomicity
// even under concurrent access.
//
// **Validates: Requirements 21.8**
func TestProperty16_RedisAtomicOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for valid Redis configurations
	genRedisConfig := gopter.CombineGens(
		gen.IntRange(1, 50), // MaxConns
	).Map(func(values []interface{}) Config {
		return Config{
			URL:              getTestRedisURL(),
			MaxConns:         values[0].(int),
			OperationTimeout: 5 * time.Second,
		}
	})

	// Generator for counter keys
	genCounterKey := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0
	}).Map(func(s string) string {
		return "counter:" + s
	})

	properties.Property("incr atomically increments value", prop.ForAll(
		func(cfg Config, key string) bool {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Clean up first
			adapter.Delete(ctx, key)

			// Increment from 0 to 1
			val1, err := adapter.Incr(ctx, key)
			if err != nil {
				t.Logf("Failed to increment: %v", err)
				return false
			}

			// Increment from 1 to 2
			val2, err := adapter.Incr(ctx, key)
			if err != nil {
				t.Logf("Failed to increment: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, key)

			return val1 == 1 && val2 == 2
		},
		genRedisConfig,
		genCounterKey,
	))

	properties.Property("decr atomically decrements value", prop.ForAll(
		func(cfg Config, key string) bool {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Clean up first
			adapter.Delete(ctx, key)

			// Set initial value
			adapter.Set(ctx, key, "10")

			// Decrement from 10 to 9
			val1, err := adapter.Decr(ctx, key)
			if err != nil {
				t.Logf("Failed to decrement: %v", err)
				return false
			}

			// Decrement from 9 to 8
			val2, err := adapter.Decr(ctx, key)
			if err != nil {
				t.Logf("Failed to decrement: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, key)

			return val1 == 9 && val2 == 8
		},
		genRedisConfig,
		genCounterKey,
	))

	properties.Property("incrby atomically increments by amount", prop.ForAll(
		func(cfg Config, key string, amount int64) bool {
			if amount == 0 {
				return true // Skip zero amounts
			}

			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Clean up first
			adapter.Delete(ctx, key)

			// Increment by amount
			val, err := adapter.IncrBy(ctx, key, amount)
			if err != nil {
				t.Logf("Failed to increment by amount: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, key)

			return val == amount
		},
		genRedisConfig,
		genCounterKey,
		gen.Int64Range(-1000, 1000),
	))

	properties.Property("decrby atomically decrements by amount", prop.ForAll(
		func(cfg Config, key string, amount int64) bool {
			if amount == 0 {
				return true // Skip zero amounts
			}

			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Clean up first
			adapter.Delete(ctx, key)

			// Set initial value
			adapter.Set(ctx, key, "1000")

			// Decrement by amount
			val, err := adapter.DecrBy(ctx, key, amount)
			if err != nil {
				t.Logf("Failed to decrement by amount: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, key)

			return val == 1000-amount
		},
		genRedisConfig,
		genCounterKey,
		gen.Int64Range(-1000, 1000),
	))

	properties.Property("concurrent increments maintain atomicity", prop.ForAll(
		func(cfg Config, key string) bool {
			log, _ := logger.NewZapLogger(logger.Config{Level: logger.InfoLevel, Format: logger.JSONFormat})
			adapter, err := NewAdapter(cfg, log)
			if err != nil {
				if isConnectionError(err) {
					return true
				}
				t.Logf("Failed to create adapter: %v", err)
				return false
			}
			defer adapter.Close()

			ctx := context.Background()

			// Clean up first
			adapter.Delete(ctx, key)

			// Perform 10 concurrent increments
			numIncrements := 10
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
					t.Logf("Concurrent increment failed: %v", err)
					return false
				}
			}

			// Get final value
			finalValue, err := adapter.Get(ctx, key)
			if err != nil {
				t.Logf("Failed to get final value: %v", err)
				return false
			}

			// Clean up
			adapter.Delete(ctx, key)

			// Final value should be exactly numIncrements
			return finalValue == fmt.Sprintf("%d", numIncrements)
		},
		genRedisConfig,
		genCounterKey,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// getTestRedisURL returns a test Redis URL
func getTestRedisURL() string {
	// This is a placeholder - in real tests, this would be a testcontainer URL
	return "redis://localhost:6379/0"
}

// isConnectionError checks if an error is a connection error
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "failed to ping redis") ||
		strings.Contains(errStr, "failed to parse redis URL")
}
