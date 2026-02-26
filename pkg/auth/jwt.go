package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

// JWTValidator validates JWT tokens and extracts claims.
type JWTValidator interface {
	Validate(ctx context.Context, token string) (*Claims, error)
}

// Claims represents the extracted claims from a validated JWT token.
type Claims struct {
	Subject   string                 // Subject (sub) - typically user ID
	Issuer    string                 // Issuer (iss) - token issuer
	Audience  []string               // Audience (aud) - intended recipients
	ExpiresAt time.Time              // Expiration time (exp)
	IssuedAt  time.Time              // Issued at (iat)
	TenantID  string                 // Tenant identifier (tenant_id)
	Scopes    []string               // OAuth2 scopes
	Roles     []string               // Roles extracted from role/roles claims
	Custom    map[string]interface{} // Custom claims
}

// JWKSValidator validates JWT tokens using JWKS for signature verification.
// It validates the signature, issuer, audience, and expiration.
type JWKSValidator struct {
	jwksClient *JWKSClient
	issuer     string
	audience   string
	logger     logger.Logger
	mappings   map[string][]string
}

// JWKSValidatorOption configures optional behavior for JWKS validator instances.
type JWKSValidatorOption func(*JWKSValidator)

// WithClaimMappings overrides claim alias mappings used during claim extraction.
// Example: map["tenant_id"] = []string{"tenant_id","tenantId","https://example.com/tenant_id"}.
func WithClaimMappings(mappings map[string][]string) JWKSValidatorOption {
	return func(v *JWKSValidator) {
		if len(mappings) == 0 {
			return
		}
		for claimName, aliases := range mappings {
			normalizedClaim := strings.TrimSpace(claimName)
			if normalizedClaim == "" {
				continue
			}
			cleanedAliases := make([]string, 0, len(aliases))
			seen := make(map[string]struct{}, len(aliases))
			for _, alias := range aliases {
				normalizedAlias := strings.TrimSpace(alias)
				if normalizedAlias == "" {
					continue
				}
				key := strings.ToLower(normalizedAlias)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				cleanedAliases = append(cleanedAliases, normalizedAlias)
			}
			v.mappings[normalizedClaim] = cleanedAliases
		}
	}
}

// NewJWKSValidator creates a new JWT validator that uses JWKS for signature verification.
func NewJWKSValidator(jwksClient *JWKSClient, issuer, audience string, logger logger.Logger, opts ...JWKSValidatorOption) *JWKSValidator {
	validator := &JWKSValidator{
		jwksClient: jwksClient,
		issuer:     issuer,
		audience:   audience,
		logger:     logger,
		mappings: map[string][]string{
			"subject":   {"sub"},
			"issuer":    {"iss"},
			"audience":  {"aud"},
			"scopes":    {"scope", "scopes"},
			"roles":     {"role", "roles"},
			"tenant_id": {"tenant_id", "tenantId"},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(validator)
		}
	}
	return validator
}

// Validate validates a JWT token and extracts its claims.
// It performs the following validations:
// - Signature verification using JWKS public key
// - Issuer validation
// - Audience validation
// - Expiration validation
func (v *JWKSValidator) Validate(ctx context.Context, tokenString string) (*Claims, error) {
	// Parse token with validation
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method is RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get kid from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in token header")
		}

		v.logger.Debug("validating token", "kid", kid)

		// Get public key from JWKS
		publicKey, err := v.jwksClient.GetKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	claims, err := v.extractClaims(token)
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	// Validate issuer
	if claims.Issuer != v.issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", v.issuer, claims.Issuer)
	}

	// Validate audience
	if !containsAudience(claims.Audience, v.audience) {
		return nil, fmt.Errorf("invalid audience: token not intended for %s", v.audience)
	}

	v.logger.Debug("token validated successfully", "subject", claims.Subject, "issuer", claims.Issuer)

	return claims, nil
}

// extractClaims extracts standard and custom claims from a JWT token.
func (v *JWKSValidator) extractClaims(token *jwt.Token) (*Claims, error) {
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("failed to parse claims")
	}

	claims := &Claims{
		Custom: make(map[string]interface{}),
	}

	// Extract canonical claims through configured alias mappings.
	claims.Subject = extractClaimByMapping(mapClaims, "subject", v.mappings)
	claims.Issuer = extractClaimByMapping(mapClaims, "issuer", v.mappings)
	claims.Audience = extractStringSliceByMapping(mapClaims, "audience", v.mappings, false)

	// Extract expiration time
	if exp, err := mapClaims.GetExpirationTime(); err == nil && exp != nil {
		claims.ExpiresAt = exp.Time
	}

	// Extract issued at time
	if iat, err := mapClaims.GetIssuedAt(); err == nil && iat != nil {
		claims.IssuedAt = iat.Time
	}

	// Extract scopes from mapped aliases.
	claims.Scopes = extractStringSliceByMapping(mapClaims, "scopes", v.mappings, true)
	claims.Scopes = appendPermissionsAsScopes(claims.Scopes, mapClaims)

	// Extract tenant identifier from configured aliases.
	claims.TenantID = extractClaimByMapping(mapClaims, "tenant_id", v.mappings)

	// Extract roles from mapped aliases.
	claims.Roles = extractStringSliceByMapping(mapClaims, "roles", v.mappings, true)

	// Extract custom claims (everything except standard claims)
	standardClaims := map[string]bool{
		"exp": true, "iat": true, "nbf": true, "jti": true,
	}
	addMappedClaimsToStandardSet(standardClaims, v.mappings)

	for key, value := range mapClaims {
		if !standardClaims[key] && !isNamespacedMappedClaimKey(key, v.mappings) {
			claims.Custom[key] = value
		}
	}

	return claims, nil
}

func extractClaimByMapping(mapClaims jwt.MapClaims, claimName string, mappings map[string][]string) string {
	normalizedClaim := strings.ToLower(strings.TrimSpace(claimName))
	if normalizedClaim == "" {
		return ""
	}

	aliases := mappings[normalizedClaim]
	candidates := append([]string{normalizedClaim}, aliases...)
	for _, candidate := range candidates {
		trimmedCandidate := strings.TrimSpace(candidate)
		if trimmedCandidate == "" {
			continue
		}
		if value, ok := mapClaims[trimmedCandidate].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}

	normalizedSuffixes := claimSuffixes(normalizedClaim, aliases)
	if len(normalizedSuffixes) == 0 {
		return ""
	}
	for key, value := range mapClaims {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		for _, suffix := range normalizedSuffixes {
			if strings.HasSuffix(normalizedKey, suffix) {
				if typed, ok := value.(string); ok {
					if trimmed := strings.TrimSpace(typed); trimmed != "" {
						return trimmed
					}
				}
				break
			}
		}
	}

	return ""
}

func extractStringSliceByMapping(mapClaims jwt.MapClaims, claimName string, mappings map[string][]string, splitOnSpaces bool) []string {
	normalizedClaim := strings.ToLower(strings.TrimSpace(claimName))
	if normalizedClaim == "" {
		return nil
	}

	candidates := claimCandidates(normalizedClaim, mappings[normalizedClaim])
	values := make([]string, 0)

	tryAppend := func(raw interface{}) bool {
		switch typed := raw.(type) {
		case string:
			if splitOnSpaces {
				values = append(values, parseScopes(typed)...)
			} else if trimmed := strings.TrimSpace(typed); trimmed != "" {
				values = append(values, trimmed)
			}
			return true
		case []string:
			for _, item := range typed {
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					values = append(values, trimmed)
				}
			}
			return true
		case []interface{}:
			for _, item := range typed {
				str, ok := item.(string)
				if !ok {
					continue
				}
				trimmed := strings.TrimSpace(str)
				if trimmed != "" {
					values = append(values, trimmed)
				}
			}
			return true
		default:
			return false
		}
	}

	for _, candidate := range candidates {
		if raw, ok := mapClaims[candidate]; ok {
			tryAppend(raw)
		}
	}

	suffixes := claimSuffixes(normalizedClaim, mappings[normalizedClaim])
	for key, raw := range mapClaims {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		for _, suffix := range suffixes {
			if strings.HasSuffix(normalizedKey, suffix) {
				tryAppend(raw)
				break
			}
		}
	}

	return dedupeStrings(values)
}

func claimCandidates(claimName string, aliases []string) []string {
	candidates := make([]string, 0, 1+len(aliases))
	candidates = append(candidates, claimName)
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed != "" {
			candidates = append(candidates, trimmed)
		}
	}
	return dedupeStrings(candidates)
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func addMappedClaimsToStandardSet(standardClaims map[string]bool, mappings map[string][]string) {
	for claimName, aliases := range mappings {
		normalizedClaim := strings.ToLower(strings.TrimSpace(claimName))
		if normalizedClaim != "" {
			standardClaims[normalizedClaim] = true
		}
		for _, alias := range aliases {
			normalizedAlias := strings.TrimSpace(alias)
			if normalizedAlias != "" {
				standardClaims[normalizedAlias] = true
			}
		}
	}
}

func isNamespacedMappedClaimKey(key string, mappings map[string][]string) bool {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	if normalizedKey == "" {
		return false
	}
	for claimName, aliases := range mappings {
		suffixes := claimSuffixes(strings.ToLower(strings.TrimSpace(claimName)), aliases)
		for _, suffix := range suffixes {
			if strings.HasSuffix(normalizedKey, suffix) {
				return true
			}
		}
	}
	return false
}

func claimSuffixes(claimName string, aliases []string) []string {
	suffixSet := map[string]struct{}{}
	addSuffix := func(raw string) {
		normalized := strings.ToLower(strings.TrimSpace(raw))
		if normalized == "" {
			return
		}
		idx := strings.LastIndex(normalized, "/")
		var lastSegment string
		if idx >= 0 && idx < len(normalized)-1 {
			lastSegment = normalized[idx+1:]
		} else {
			lastSegment = normalized
		}
		if strings.TrimSpace(lastSegment) == "" {
			return
		}
		suffixSet["/"+lastSegment] = struct{}{}
	}

	addSuffix(claimName)
	for _, alias := range aliases {
		addSuffix(alias)
	}

	suffixes := make([]string, 0, len(suffixSet))
	for suffix := range suffixSet {
		suffixes = append(suffixes, suffix)
	}
	slices.Sort(suffixes)
	return suffixes
}

// containsAudience checks if the expected audience is in the token's audience list.
func containsAudience(audiences []string, expected string) bool {
	for _, aud := range audiences {
		if aud == expected {
			return true
		}
	}
	return false
}

// parseScopes parses space-separated scope string into a slice.
func parseScopes(scope string) []string {
	if scope == "" {
		return nil
	}

	scopes := []string{}
	current := ""

	for _, ch := range scope {
		if ch == ' ' {
			if current != "" {
				scopes = append(scopes, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		scopes = append(scopes, current)
	}

	return scopes
}

func appendPermissionsAsScopes(scopes []string, mapClaims jwt.MapClaims) []string {
	addScope := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || slices.Contains(scopes, trimmed) {
			return
		}
		scopes = append(scopes, trimmed)
	}

	parsePermissionClaim := func(value interface{}) {
		switch typed := value.(type) {
		case []interface{}:
			for _, item := range typed {
				if scopeValue, ok := item.(string); ok {
					addScope(scopeValue)
				}
			}
		case []string:
			for _, item := range typed {
				addScope(item)
			}
		case string:
			for _, item := range parseScopes(typed) {
				addScope(item)
			}
		}
	}

	if permissions, ok := mapClaims["permissions"]; ok {
		parsePermissionClaim(permissions)
	}

	for key, value := range mapClaims {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if !strings.HasSuffix(normalized, "/permissions") {
			continue
		}
		parsePermissionClaim(value)
	}

	return scopes
}

// claimsContextKey is the context key for storing claims.
type claimsContextKey struct{}

// WithClaims stores claims in the context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

// GetClaims retrieves claims from the context.
// Returns nil if no claims are found.
func GetClaims(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(claimsContextKey{}).(*Claims); ok {
		return claims
	}
	return nil
}
