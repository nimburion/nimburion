package auth_test

import (
	"context"
	"fmt"
	"time"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// This example demonstrates how to create and use a JWKS client
// to fetch and cache public keys for JWT validation.
func ExampleNewJWKSClient() {
	// Create a logger (use your actual logger implementation)
	var log logger.Logger // = logger.NewZapLogger(...)

	// Create JWKS client with 1 hour cache TTL
	jwksURL := "https://your-auth-server.com/.well-known/jwks.json"
	cacheTTL := 1 * time.Hour
	client := auth.NewJWKSClient(jwksURL, cacheTTL, log)

	// Get a public key by its key ID (kid)
	ctx := context.Background()
	kid := "key-id-from-jwt-header"
	publicKey, err := client.GetKey(ctx, kid)
	if err != nil {
		fmt.Printf("Failed to get key: %v\n", err)
		return
	}

	// Use the public key to verify JWT signature
	_ = publicKey // Use with jwt.Parse or similar
}
