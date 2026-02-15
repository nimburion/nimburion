# Authentication Package

This package provides OAuth2/OIDC authentication support for the Go microservices framework.

## Components

### JWKS Client

The JWKS (JSON Web Key Set) client fetches and caches public keys used for JWT signature verification.

**Features:**
- Fetches JWKS from configured URL
- Thread-safe caching with configurable TTL
- Automatic cache expiration and refresh
- Support for RSA keys (RS256, RS384, RS512)
- Context-aware HTTP requests with timeout

**Usage:**

```go
import (
    "context"
    "time"
    "github.com/nimburion/nimburion/pkg/auth"
)

// Create JWKS client
jwksURL := "https://your-auth-server.com/.well-known/jwks.json"
cacheTTL := 1 * time.Hour
client := auth.NewJWKSClient(jwksURL, cacheTTL, logger)

// Get public key by key ID
ctx := context.Background()
publicKey, err := client.GetKey(ctx, "key-id")
if err != nil {
    // Handle error
}

// Use publicKey to verify JWT signature
```

**Caching Behavior:**
- Keys are cached for the configured TTL duration
- First request for a key fetches from the JWKS endpoint
- Subsequent requests within TTL use cached keys
- After TTL expiration, next request refreshes the cache

**Thread Safety:**
- All cache operations are protected by RWMutex
- Safe for concurrent access from multiple goroutines

### JWT Validator

The JWT validator validates JWT tokens and extracts claims. It uses JWKS for signature verification and validates standard JWT claims.

**Features:**
- RSA signature verification using JWKS
- Issuer validation
- Audience validation
- Expiration validation
- Scope extraction (OAuth2)
- Custom claims extraction
- Context-aware validation

**Usage:**

```go
import (
    "context"
    "time"
    "github.com/nimburion/nimburion/pkg/auth"
)

// Create JWKS client
jwksClient := auth.NewJWKSClient(
    "https://your-auth-server.com/.well-known/jwks.json",
    1 * time.Hour,
    logger,
)

// Create JWT validator
validator := auth.NewJWKSValidator(
    jwksClient,
    "https://your-auth-server.com",  // Expected issuer
    "your-api-audience",              // Expected audience
    logger,
)

// Validate token
ctx := context.Background()
claims, err := validator.Validate(ctx, tokenString)
if err != nil {
    // Handle validation error (invalid signature, expired, wrong issuer, etc.)
}

// Use claims
fmt.Println("Subject:", claims.Subject)
fmt.Println("Scopes:", claims.Scopes)
fmt.Println("Custom claims:", claims.Custom)
```

**Validation Process:**
1. Parse JWT and extract kid from header
2. Fetch public key from JWKS using kid
3. Verify signature using RSA public key
4. Validate expiration time
5. Validate issuer matches expected issuer
6. Validate audience contains expected audience
7. Extract standard and custom claims

**Claims Structure:**
- `Subject`: User identifier (sub claim)
- `Issuer`: Token issuer (iss claim)
- `Audience`: Intended recipients (aud claim)
- `ExpiresAt`: Expiration time (exp claim)
- `IssuedAt`: Issued at time (iat claim)
- `Scopes`: OAuth2 scopes (scope or scopes claim)
- `Custom`: All other claims as key-value pairs

**Scope Handling:**
- Supports space-separated scope string: `"read write delete"`
- Supports scope array: `["read", "write", "delete"]`
- Both formats are normalized to string slice

## Requirements

Implements:
- Requirement 5.2: Validate JWT tokens using issuer and JWKS
- Requirement 5.3: Cache JWKS responses with configurable TTL
- Requirement 5.4: Extract claims from validated tokens
- Requirement 5.6: Return errors for invalid tokens

## Testing

Run tests with:
```bash
go test -v ./pkg/auth/...
```

Test coverage includes:
- Successful key retrieval
- Key not found scenarios
- Cache hit/miss behavior
- Cache expiration
- Server errors
- Invalid JSON responses
- Context cancellation
- Multiple keys in JWKS
- Unsupported key types
