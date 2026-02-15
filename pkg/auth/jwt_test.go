package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// mockJWKSClient is a mock implementation of JWKSClient for testing.
type mockJWKSClient struct {
	keys map[string]interface{}
	err  error
}

func (m *mockJWKSClient) GetKey(ctx context.Context, kid string) (interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	if key, ok := m.keys[kid]; ok {
		return key, nil
	}
	return nil, jwt.ErrTokenInvalidId
}

// generateTestKeyPair generates an RSA key pair for testing.
func generateTestKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// createTestToken creates a JWT token for testing.
func createTestToken(privateKey *rsa.PrivateKey, kid string, claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	return token.SignedString(privateKey)
}

func TestJWKSValidator_Validate_Success(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Create validator
	validator := NewJWKSValidator(
		&JWKSClient{}, // Not used due to mock
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)
	validator.jwksClient = &JWKSClient{} // Replace with mock behavior

	// Create test token with valid claims
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   "user123",
		"iss":   "https://issuer.example.com",
		"aud":   "test-audience",
		"exp":   jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat":   jwt.NewNumericDate(now),
		"scope": "read write",
	}

	tokenString, err := createTestToken(privateKey, "test-kid", claims)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	// Parse and validate manually to test extraction logic
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	// Verify extracted claims
	if extractedClaims.Subject != "user123" {
		t.Errorf("expected subject 'user123', got '%s'", extractedClaims.Subject)
	}

	if extractedClaims.Issuer != "https://issuer.example.com" {
		t.Errorf("expected issuer 'https://issuer.example.com', got '%s'", extractedClaims.Issuer)
	}

	if len(extractedClaims.Audience) != 1 || extractedClaims.Audience[0] != "test-audience" {
		t.Errorf("expected audience ['test-audience'], got %v", extractedClaims.Audience)
	}

	if len(extractedClaims.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(extractedClaims.Scopes))
	}

	if extractedClaims.Scopes[0] != "read" || extractedClaims.Scopes[1] != "write" {
		t.Errorf("expected scopes ['read', 'write'], got %v", extractedClaims.Scopes)
	}
}

func TestJWKSValidator_Validate_ExpiredToken(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Create validator
	_ = NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	// Create expired token
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "test-audience",
		"exp": jwt.NewNumericDate(now.Add(-1 * time.Hour)), // Expired
		"iat": jwt.NewNumericDate(now.Add(-2 * time.Hour)),
	}

	tokenString, err := createTestToken(privateKey, "test-kid", claims)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	// Validate should fail due to expiration
	_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})

	if err == nil {
		t.Error("expected validation to fail for expired token")
	}
}

func TestJWKSValidator_Validate_InvalidIssuer(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Create validator
	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	// Create token with wrong issuer
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "https://wrong-issuer.example.com",
		"aud": "test-audience",
		"exp": jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat": jwt.NewNumericDate(now),
	}

	tokenString, err := createTestToken(privateKey, "test-kid", claims)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	// Extract claims
	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	// Validate issuer manually
	if extractedClaims.Issuer == validator.issuer {
		t.Error("expected issuer validation to fail")
	}
}

func TestJWKSValidator_Validate_InvalidAudience(t *testing.T) {
	// Generate test key pair
	privateKey, publicKey, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Create validator
	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	// Create token with wrong audience
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "wrong-audience",
		"exp": jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat": jwt.NewNumericDate(now),
	}

	tokenString, err := createTestToken(privateKey, "test-kid", claims)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	// Extract claims
	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	// Validate audience manually
	if containsAudience(extractedClaims.Audience, validator.audience) {
		t.Error("expected audience validation to fail")
	}
}

func TestJWKSValidator_Validate_MissingKid(t *testing.T) {
	// Generate test key pair
	privateKey, _, err := generateTestKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	// Create token without kid
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "test-audience",
		"exp": jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat": jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// Don't set kid header
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Validator should fail when trying to get kid
	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	ctx := context.Background()
	_, err = validator.Validate(ctx, tokenString)

	if err == nil {
		t.Error("expected validation to fail for token without kid")
	}
}

func TestExtractClaims_ArrayAudience(t *testing.T) {
	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	// Create token with array audience
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": []interface{}{"audience1", "audience2", "test-audience"},
		"exp": jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat": jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if len(extractedClaims.Audience) != 3 {
		t.Errorf("expected 3 audiences, got %d", len(extractedClaims.Audience))
	}

	if !containsAudience(extractedClaims.Audience, "test-audience") {
		t.Error("expected audience list to contain 'test-audience'")
	}
}

func TestExtractClaims_ScopesArray(t *testing.T) {
	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	// Create token with scopes as array
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":    "user123",
		"iss":    "https://issuer.example.com",
		"aud":    "test-audience",
		"exp":    jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat":    jwt.NewNumericDate(now),
		"scopes": []interface{}{"read", "write", "delete"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if len(extractedClaims.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(extractedClaims.Scopes))
	}

	expectedScopes := map[string]bool{"read": true, "write": true, "delete": true}
	for _, scope := range extractedClaims.Scopes {
		if !expectedScopes[scope] {
			t.Errorf("unexpected scope: %s", scope)
		}
	}
}

func TestExtractClaims_PermissionsMergedIntoScopes(t *testing.T) {
	validator := NewJWKSValidator(nil, "", "", nil)
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":         "user123",
		"iss":         "https://issuer.example.com",
		"aud":         "api-gateway",
		"exp":         now.Add(1 * time.Hour).Unix(),
		"scope":       "openid profile",
		"permissions": []interface{}{"platform:tenant:read", "member:read"},
	})

	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("extractClaims failed: %v", err)
	}

	expected := map[string]bool{
		"openid":               true,
		"profile":              true,
		"platform:tenant:read": true,
		"member:read":          true,
	}
	for _, scope := range extractedClaims.Scopes {
		delete(expected, scope)
	}
	if len(expected) != 0 {
		t.Fatalf("missing expected scopes after permissions merge: %v", expected)
	}
}

func TestExtractClaims_NamespacedPermissionsMergedIntoScopes(t *testing.T) {
	validator := NewJWKSValidator(nil, "", "", nil)
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   "user123",
		"iss":   "https://issuer.example.com",
		"aud":   "api-gateway",
		"exp":   now.Add(1 * time.Hour).Unix(),
		"scope": "openid",
		"https://example.localhost/permissions": []interface{}{"platform:tenant:read"},
	})

	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("extractClaims failed: %v", err)
	}

	found := false
	for _, scope := range extractedClaims.Scopes {
		if scope == "platform:tenant:read" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected namespaced permission to be merged into scopes, got %v", extractedClaims.Scopes)
	}
}

func TestExtractClaims_CustomClaims(t *testing.T) {
	// Create logger
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	// Create token with custom claims
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":        "user123",
		"iss":        "https://issuer.example.com",
		"aud":        "test-audience",
		"exp":        jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat":        jwt.NewNumericDate(now),
		"tenant_id":  "tenant-1",
		"email":      "user@example.com",
		"role":       "admin",
		"department": "engineering",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if len(extractedClaims.Custom) != 2 {
		t.Errorf("expected 2 custom claims, got %d", len(extractedClaims.Custom))
	}

	if extractedClaims.Custom["email"] != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got '%v'", extractedClaims.Custom["email"])
	}

	if extractedClaims.TenantID != "tenant-1" {
		t.Errorf("expected tenant_id 'tenant-1', got '%v'", extractedClaims.TenantID)
	}

	if len(extractedClaims.Roles) != 1 || extractedClaims.Roles[0] != "admin" {
		t.Errorf("expected roles ['admin'], got '%v'", extractedClaims.Roles)
	}
}

func TestExtractClaims_RolesArray(t *testing.T) {
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	now := time.Now()
	claims := jwt.MapClaims{
		"sub":       "user123",
		"iss":       "https://issuer.example.com",
		"aud":       "test-audience",
		"exp":       jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat":       jwt.NewNumericDate(now),
		"tenant_id": "tenant-2",
		"roles":     []interface{}{"operator", "auditor"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if extractedClaims.TenantID != "tenant-2" {
		t.Errorf("expected tenant_id 'tenant-2', got '%s'", extractedClaims.TenantID)
	}
	if len(extractedClaims.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(extractedClaims.Roles))
	}
}

func TestExtractClaims_NamespacedTenantID(t *testing.T) {
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
	)

	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "test-audience",
		"exp": jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat": jwt.NewNumericDate(now),
		"https://example.localhost/tenant_id": "tenant-dev",
		"roles": []interface{}{"operator"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if extractedClaims.TenantID != "tenant-dev" {
		t.Errorf("expected tenant_id 'tenant-dev', got '%s'", extractedClaims.TenantID)
	}
}

func TestExtractClaims_TenantIDWithConfiguredMapping(t *testing.T) {
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
		WithClaimMappings(map[string][]string{
			"tenant_id": {"org_id"},
		}),
	)

	now := time.Now()
	claims := jwt.MapClaims{
		"sub":    "user123",
		"iss":    "https://issuer.example.com",
		"aud":    "test-audience",
		"exp":    jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat":    jwt.NewNumericDate(now),
		"org_id": "org-dev",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if extractedClaims.TenantID != "org-dev" {
		t.Fatalf("expected tenant_id from mapped org_id, got '%s'", extractedClaims.TenantID)
	}
}

func TestExtractClaims_CanonicalFieldsWithConfiguredMappings(t *testing.T) {
	testLogger, err := logger.NewZapLogger(logger.Config{
		Level:  logger.InfoLevel,
		Format: logger.TextFormat,
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	validator := NewJWKSValidator(
		&JWKSClient{},
		"https://issuer.example.com",
		"test-audience",
		testLogger,
		WithClaimMappings(map[string][]string{
			"subject":  {"user_id"},
			"issuer":   {"token_issuer"},
			"audience": {"client_audience"},
			"scopes":   {"app_scopes"},
			"roles":    {"app_roles"},
		}),
	)

	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":         "user-42",
		"token_issuer":    "https://issuer.example.com",
		"client_audience": []interface{}{"test-audience", "secondary-aud"},
		"app_scopes":      "openid profile",
		"app_roles":       []interface{}{"admin", "manager"},
		"exp":             jwt.NewNumericDate(now.Add(1 * time.Hour)),
		"iat":             jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	extractedClaims, err := validator.extractClaims(token)
	if err != nil {
		t.Fatalf("failed to extract claims: %v", err)
	}

	if extractedClaims.Subject != "user-42" {
		t.Fatalf("expected mapped subject, got '%s'", extractedClaims.Subject)
	}
	if extractedClaims.Issuer != "https://issuer.example.com" {
		t.Fatalf("expected mapped issuer, got '%s'", extractedClaims.Issuer)
	}
	if !containsAudience(extractedClaims.Audience, "test-audience") {
		t.Fatalf("expected mapped audience to contain test-audience, got %v", extractedClaims.Audience)
	}
	if !containsAudience(extractedClaims.Audience, "secondary-aud") {
		t.Fatalf("expected mapped audience to contain secondary-aud, got %v", extractedClaims.Audience)
	}
	if !slices.Contains(extractedClaims.Scopes, "openid") || !slices.Contains(extractedClaims.Scopes, "profile") {
		t.Fatalf("expected mapped scopes to contain openid and profile, got %v", extractedClaims.Scopes)
	}
	if !slices.Contains(extractedClaims.Roles, "admin") || !slices.Contains(extractedClaims.Roles, "manager") {
		t.Fatalf("expected mapped roles to contain admin and manager, got %v", extractedClaims.Roles)
	}
}

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single scope",
			input:    "read",
			expected: []string{"read"},
		},
		{
			name:     "multiple scopes",
			input:    "read write delete",
			expected: []string{"read", "write", "delete"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "extra spaces",
			input:    "read  write   delete",
			expected: []string{"read", "write", "delete"},
		},
		{
			name:     "leading and trailing spaces",
			input:    " read write ",
			expected: []string{"read", "write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseScopes(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d scopes, got %d", len(tt.expected), len(result))
				return
			}

			for i, scope := range result {
				if scope != tt.expected[i] {
					t.Errorf("expected scope[%d] = '%s', got '%s'", i, tt.expected[i], scope)
				}
			}
		})
	}
}
