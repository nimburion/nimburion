// Package authz provides authentication and authorization middleware components.
package authz

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/nimburion/nimburion/pkg/auth"
	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// ClaimsKey is the context key for storing JWT claims.
const ClaimsKey = "claims"

const (
	DefaultTenantHeader  = "X-Tenant-ID"
	DefaultSubjectHeader = "X-Subject-ID"
	DefaultScopesHeader  = "X-Scopes"
	DefaultRolesHeader   = "X-Roles"
)

// ClaimValueSource defines where guard values are extracted from.
type ClaimValueSource string

const (
	ClaimValueSourceRoute  ClaimValueSource = "route"
	ClaimValueSourceHeader ClaimValueSource = "header"
	ClaimValueSourceQuery  ClaimValueSource = "query"
)

// ClaimOperator defines the rule evaluation operator.
type ClaimOperator string

const (
	ClaimOperatorRequired ClaimOperator = "required"
	ClaimOperatorEquals   ClaimOperator = "equals"
	ClaimOperatorOneOf    ClaimOperator = "one_of"
	ClaimOperatorRegex    ClaimOperator = "regex"
)

// ClaimRule defines a generic claim guard rule.
type ClaimRule struct {
	Claim    string
	Aliases  []string
	Source   ClaimValueSource
	Key      string
	Operator ClaimOperator
	Values   []string
	Optional bool
}

// Authenticate creates middleware that validates JWT tokens from the Authorization header.
// It extracts the Bearer token, validates it using the provided JWT validator,
// stores the claims in the context, and returns 401 for invalid tokens.
//
// Requirements: 3.4, 5.5, 5.6
func Authenticate(validator auth.JWTValidator) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Extract Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "missing authorization header",
				})
			}

			// Parse Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid authorization header format",
				})
			}

			token := parts[1]

			// Validate token using JWT validator
			claims, err := validator.Validate(c.Request().Context(), token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid token",
				})
			}

			// Store claims in context for use by handlers and other middleware
			c.Set(ClaimsKey, claims)

			// Also store in request context for propagation
			ctx := c.Request().Context()
			ctx = auth.WithClaims(ctx, claims)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// ScopeLogic defines the logic for combining multiple scopes.
type ScopeLogic int

const (
	// ScopeLogicAND requires all scopes to be present (default).
	ScopeLogicAND ScopeLogic = iota
	// ScopeLogicOR requires at least one scope to be present.
	ScopeLogicOR
)

// ScopeRequirement defines the scope requirements for authorization.
type ScopeRequirement struct {
	Scopes []string
	Logic  ScopeLogic
}

// RequireScopes creates middleware that checks if the authenticated user has the required scopes.
// It extracts claims from the context and verifies that all required scopes are present (AND logic).
// Returns 401 if no claims are found (not authenticated) and 403 if scopes are insufficient.
//
// Requirements: 6.1, 6.2, 6.3, 6.4, 6.5
func RequireScopes(requiredScopes ...string) router.MiddlewareFunc {
	return RequireScopesWithLogic(ScopeLogicAND, requiredScopes...)
}

// RequireAnyScope creates middleware that checks if the authenticated user has at least one of the required scopes.
// It uses OR logic - the user needs any one of the specified scopes.
// Returns 401 if no claims are found (not authenticated) and 403 if scopes are insufficient.
//
// Requirements: 6.1, 6.2, 6.3, 6.4, 6.5
func RequireAnyScope(requiredScopes ...string) router.MiddlewareFunc {
	return RequireScopesWithLogic(ScopeLogicOR, requiredScopes...)
}

// RequireScopesWithLogic creates middleware that checks if the authenticated user has the required scopes
// using the specified logic (AND or OR).
// Returns 401 if no claims are found (not authenticated) and 403 if scopes are insufficient.
//
// Requirements: 6.1, 6.2, 6.3, 6.4, 6.5
func RequireScopesWithLogic(logic ScopeLogic, requiredScopes ...string) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Extract claims from context
			claimsValue := c.Get(ClaimsKey)
			if claimsValue == nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "missing authentication",
				})
			}

			claims, ok := claimsValue.(*auth.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid authentication",
				})
			}

			// Check if user has required scopes based on logic
			var hasPermission bool
			if logic == ScopeLogicOR {
				hasPermission = hasAnyScope(claims.Scopes, requiredScopes)
			} else {
				hasPermission = hasAllScopes(claims.Scopes, requiredScopes)
			}

			if !hasPermission {
				logicStr := "all"
				if logic == ScopeLogicOR {
					logicStr = "any"
				}
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"error":           "insufficient permissions",
					"required_scopes": requiredScopes,
					"logic":           logicStr,
				})
			}

			return next(c)
		}
	}
}

// ClaimsGuard evaluates generic claim rules against route/header/query values.
func ClaimsGuard(rules ...ClaimRule) router.MiddlewareFunc {
	return ClaimsGuardWithMappings(nil, rules...)
}

// ClaimsGuardWithMappings evaluates generic claim rules against route/header/query values using claim mappings.
func ClaimsGuardWithMappings(mappings map[string][]string, rules ...ClaimRule) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			claimsValue := c.Get(ClaimsKey)
			if claimsValue == nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "missing authentication",
				})
			}

			claims, ok := claimsValue.(*auth.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid authentication",
				})
			}

			for _, rule := range rules {
				if strings.TrimSpace(rule.Claim) == "" {
					continue
				}

				claimValue := resolveClaimValue(claims, rule.Claim, rule.Aliases, mappings)
				ok, err := evaluateClaimRule(c, rule, claimValue)
				if err != nil {
					return c.JSON(http.StatusForbidden, map[string]interface{}{
						"error": err.Error(),
					})
				}
				if ok || rule.Optional {
					continue
				}

				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"error": fmt.Sprintf("claim guard failed for %s with operator %s", strings.TrimSpace(rule.Claim), strings.TrimSpace(string(rule.Operator))),
				})
			}

			return next(c)
		}
	}
}

// ClaimsGuardFromConfig builds claims guard middleware from auth configuration.
func ClaimsGuardFromConfig(authCfg config.AuthConfig) router.MiddlewareFunc {
	rules := ClaimRulesFromConfig(authCfg)
	if len(rules) == 0 {
		return ClaimsGuardWithMappings(authCfg.Claims.Mappings)
	}
	return ClaimsGuardWithMappings(authCfg.Claims.Mappings, rules...)
}

// ClaimRulesFromConfig maps config-based claim rules to middleware rules.
func ClaimRulesFromConfig(authCfg config.AuthConfig) []ClaimRule {
	rules := make([]ClaimRule, 0, len(authCfg.Claims.Rules))
	for _, rule := range authCfg.Claims.Rules {
		claim := strings.TrimSpace(rule.Claim)
		if claim == "" {
			continue
		}
		op := strings.ToLower(strings.TrimSpace(rule.Operator))
		if op == "" {
			op = string(ClaimOperatorRequired)
		}
		src := strings.ToLower(strings.TrimSpace(rule.Source))
		rules = append(rules, ClaimRule{
			Claim:    claim,
			Aliases:  rule.Aliases,
			Source:   ClaimValueSource(src),
			Key:      rule.Key,
			Operator: ClaimOperator(op),
			Values:   rule.Values,
			Optional: rule.Optional,
		})
	}
	return rules
}

func evaluateClaimRule(c router.Context, rule ClaimRule, claimValue string) (bool, error) {
	operator := strings.ToLower(strings.TrimSpace(string(rule.Operator)))
	if operator == "" {
		operator = string(ClaimOperatorRequired)
	}

	switch ClaimOperator(operator) {
	case ClaimOperatorRequired:
		return claimValue != "", nil
	case ClaimOperatorEquals:
		expected, err := valueFromSource(c, rule.Source, rule.Key)
		if err != nil {
			return false, err
		}
		return claimValue != "" && claimValue == expected, nil
	case ClaimOperatorOneOf:
		if claimValue == "" {
			return false, nil
		}
		for _, candidate := range rule.Values {
			if claimValue == strings.TrimSpace(candidate) {
				return true, nil
			}
		}
		return false, nil
	case ClaimOperatorRegex:
		if claimValue == "" {
			return false, nil
		}
		if len(rule.Values) == 0 || strings.TrimSpace(rule.Values[0]) == "" {
			return false, fmt.Errorf("regex operator requires one pattern in values")
		}
		pattern := strings.TrimSpace(rule.Values[0])
		matched, err := regexp.MatchString(pattern, claimValue)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern: %w", err)
		}
		return matched, nil
	default:
		return false, fmt.Errorf("unsupported claim operator %q", rule.Operator)
	}
}

func valueFromSource(c router.Context, source ClaimValueSource, key string) (string, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return "", fmt.Errorf("rule key is required")
	}
	switch ClaimValueSource(strings.ToLower(strings.TrimSpace(string(source)))) {
	case ClaimValueSourceRoute:
		return strings.TrimSpace(c.Param(trimmedKey)), nil
	case ClaimValueSourceHeader:
		return strings.TrimSpace(c.Request().Header.Get(trimmedKey)), nil
	case ClaimValueSourceQuery:
		return strings.TrimSpace(c.Query(trimmedKey)), nil
	default:
		return "", fmt.Errorf("unsupported claim source %q", source)
	}
}

func resolveClaimValue(claims *auth.Claims, name string, aliases []string, mappings map[string][]string) string {
	candidates := append([]string{name}, aliases...)
	for _, candidate := range candidates {
		if value := claimValue(claims, candidate, mappings); value != "" {
			return value
		}
	}
	return ""
}

func claimValue(claims *auth.Claims, key string, mappings map[string][]string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))

	if canonical := resolveCanonicalClaim(normalized, mappings); canonical != "" {
		if value := canonicalClaimValue(claims, canonical); value != "" {
			return value
		}
	}

	if value, ok := claimToString(claims.Custom[strings.TrimSpace(key)]); ok {
		return value
	}

	if canonical := resolveCanonicalClaim(normalized, mappings); canonical != "" {
		for claimKey, claimValue := range claims.Custom {
			if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(claimKey)), canonicalSuffix(canonical)) {
				continue
			}
			if value, ok := claimToString(claimValue); ok {
				return value
			}
		}
	}

	return ""
}

func canonicalClaimValue(claims *auth.Claims, canonical string) string {
	switch canonical {
	case "tenant_id":
		return strings.TrimSpace(claims.TenantID)
	case "subject":
		return strings.TrimSpace(claims.Subject)
	case "issuer":
		return strings.TrimSpace(claims.Issuer)
	case "roles":
		if len(claims.Roles) > 0 {
			return strings.Join(claims.Roles, ",")
		}
	case "scopes":
		if len(claims.Scopes) > 0 {
			return strings.Join(claims.Scopes, " ")
		}
	case "audience":
		if len(claims.Audience) > 0 {
			return strings.Join(claims.Audience, " ")
		}
	}
	return ""
}

func resolveCanonicalClaim(normalizedKey string, mappings map[string][]string) string {
	effective := effectiveClaimMappings(mappings)
	for canonical, aliases := range effective {
		if normalizedKey == canonical {
			return canonical
		}
		for _, alias := range aliases {
			if normalizedKey == strings.ToLower(strings.TrimSpace(alias)) {
				return canonical
			}
		}
	}
	return ""
}

func effectiveClaimMappings(mappings map[string][]string) map[string][]string {
	out := map[string][]string{
		"subject":   {"sub"},
		"issuer":    {"iss"},
		"audience":  {"aud"},
		"scopes":    {"scope", "scopes"},
		"roles":     {"role", "roles"},
		"tenant_id": {"tenant_id", "tenantId"},
	}
	for claim, aliases := range mappings {
		normalizedClaim := strings.ToLower(strings.TrimSpace(claim))
		if normalizedClaim == "" {
			continue
		}
		clean := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			if trimmed := strings.TrimSpace(alias); trimmed != "" {
				clean = append(clean, trimmed)
			}
		}
		out[normalizedClaim] = clean
	}
	return out
}

func canonicalSuffix(canonical string) string {
	switch canonical {
	case "tenant_id":
		return "/tenant_id"
	case "subject":
		return "/sub"
	case "issuer":
		return "/iss"
	case "roles":
		return "/roles"
	case "scopes":
		return "/scope"
	case "audience":
		return "/aud"
	default:
		return "/" + canonical
	}
}

func claimToString(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return trimmed, trimmed != ""
	case fmt.Stringer:
		trimmed := strings.TrimSpace(typed.String())
		return trimmed, trimmed != ""
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return fmt.Sprintf("%v", typed), true
	default:
		return "", false
	}
}

// IdentityHeaderConfig defines header names used when propagating identity upstream.
type IdentityHeaderConfig struct {
	TenantHeader  string
	SubjectHeader string
	ScopesHeader  string
	RolesHeader   string
}

func (c *IdentityHeaderConfig) normalize() {
	if strings.TrimSpace(c.TenantHeader) == "" {
		c.TenantHeader = DefaultTenantHeader
	}
	if strings.TrimSpace(c.SubjectHeader) == "" {
		c.SubjectHeader = DefaultSubjectHeader
	}
	if strings.TrimSpace(c.ScopesHeader) == "" {
		c.ScopesHeader = DefaultScopesHeader
	}
	if strings.TrimSpace(c.RolesHeader) == "" {
		c.RolesHeader = DefaultRolesHeader
	}
}

// ForwardIdentityHeaders propagates identity information from JWT claims to request headers.
func ForwardIdentityHeaders(cfg IdentityHeaderConfig) router.MiddlewareFunc {
	cfg.normalize()

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			claimsValue := c.Get(ClaimsKey)
			if claimsValue == nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "missing authentication",
				})
			}

			claims, ok := claimsValue.(*auth.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error": "invalid authentication",
				})
			}

			if strings.TrimSpace(claims.TenantID) != "" {
				c.Request().Header.Set(cfg.TenantHeader, claims.TenantID)
			}
			if strings.TrimSpace(claims.Subject) != "" {
				c.Request().Header.Set(cfg.SubjectHeader, claims.Subject)
			}
			if len(claims.Scopes) > 0 {
				c.Request().Header.Set(cfg.ScopesHeader, strings.Join(claims.Scopes, " "))
			}
			if len(claims.Roles) > 0 {
				c.Request().Header.Set(cfg.RolesHeader, strings.Join(claims.Roles, ","))
			}

			return next(c)
		}
	}
}

// hasAllScopes checks if the user has all required scopes (AND logic).
func hasAllScopes(userScopes, requiredScopes []string) bool {
	// Empty requirements always pass
	if len(requiredScopes) == 0 {
		return true
	}

	// Build a set of user scopes for O(1) lookup
	scopeSet := make(map[string]bool, len(userScopes))
	for _, scope := range userScopes {
		scopeSet[scope] = true
	}

	// Check that all required scopes are present
	for _, required := range requiredScopes {
		if !scopeSet[required] {
			return false
		}
	}

	return true
}

// HasAllScopes checks if the user has all required scopes (AND logic).
func HasAllScopes(userScopes, requiredScopes []string) bool {
	return hasAllScopes(userScopes, requiredScopes)
}

// hasAnyScope checks if the user has at least one of the required scopes (OR logic).
func hasAnyScope(userScopes, requiredScopes []string) bool {
	// Empty requirements always pass
	if len(requiredScopes) == 0 {
		return true
	}

	// Build a set of user scopes for O(1) lookup
	scopeSet := make(map[string]bool, len(userScopes))
	for _, scope := range userScopes {
		scopeSet[scope] = true
	}

	// Check if any required scope is present
	for _, required := range requiredScopes {
		if scopeSet[required] {
			return true
		}
	}

	return false
}

// HasAnyScope checks if the user has at least one required scope (OR logic).
func HasAnyScope(userScopes, requiredScopes []string) bool {
	return hasAnyScope(userScopes, requiredScopes)
}

// hasRequiredScopes is deprecated. Use hasAllScopes instead.
// Kept for backward compatibility.
func hasRequiredScopes(userScopes, requiredScopes []string) bool {
	return hasAllScopes(userScopes, requiredScopes)
}
