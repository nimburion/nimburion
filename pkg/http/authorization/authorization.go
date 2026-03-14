// Package authorization provides HTTP authorization middleware.
package authorization

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/nimburion/nimburion/pkg/auth"
	authconfig "github.com/nimburion/nimburion/pkg/auth/config"
	"github.com/nimburion/nimburion/pkg/http/authentication"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/policy"
)

type (
	// ClaimValueSource aliases the policy value source type for HTTP authorization helpers.
	ClaimValueSource = policy.ValueSource
	// ClaimOperator aliases the policy claim operator type for HTTP authorization helpers.
	ClaimOperator = policy.Operator
	// ScopeLogic aliases the policy scope logic type for HTTP authorization helpers.
	ScopeLogic = policy.ScopeLogic
	// ScopeRequirement aliases the policy scope requirement type for HTTP authorization helpers.
	ScopeRequirement = policy.ScopeRequirement
	// ClaimRule aliases the policy claim rule type for HTTP authorization helpers.
	ClaimRule = policy.ClaimRule
)

const (
	// ClaimValueSourceRoute resolves claim comparison values from route parameters.
	ClaimValueSourceRoute = policy.ValueSourceRoute
	// ClaimValueSourceHeader resolves claim comparison values from HTTP headers.
	ClaimValueSourceHeader = policy.ValueSourceHeader
	// ClaimValueSourceQuery resolves claim comparison values from query parameters.
	ClaimValueSourceQuery = policy.ValueSourceQuery

	// ClaimOperatorRequired requires the claim to be present.
	ClaimOperatorRequired = policy.OperatorRequired
	// ClaimOperatorEquals requires the claim to equal the resolved comparison value.
	ClaimOperatorEquals = policy.OperatorEquals
	// ClaimOperatorOneOf requires the claim to match one of the configured values.
	ClaimOperatorOneOf = policy.OperatorOneOf
	// ClaimOperatorRegex requires the claim to match the configured regular expression.
	ClaimOperatorRegex = policy.OperatorRegex

	// ScopeLogicAND requires all scopes to be present.
	ScopeLogicAND = policy.ScopeLogicAND
	// ScopeLogicOR requires at least one scope to be present.
	ScopeLogicOR = policy.ScopeLogicOR
)

// RequireScopes enforces that all requiredScopes are present on the authenticated subject.
func RequireScopes(requiredScopes ...string) router.MiddlewareFunc {
	return RequireScopesWithLogic(ScopeLogicAND, requiredScopes...)
}

// RequireAnyScope enforces that at least one of requiredScopes is present.
func RequireAnyScope(requiredScopes ...string) router.MiddlewareFunc {
	return RequireScopesWithLogic(ScopeLogicOR, requiredScopes...)
}

// RequireScopesWithLogic enforces requiredScopes using the provided scope logic.
func RequireScopesWithLogic(logic ScopeLogic, requiredScopes ...string) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			claimsValue := c.Get(authentication.ClaimsKey)
			if claimsValue == nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{"error": "missing authentication"})
			}
			claims, ok := claimsValue.(*auth.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{"error": "invalid authentication"})
			}

			hasPermission := policy.EvaluateScopes(subjectFromClaims(claims), policy.ScopeRequirement{
				Scopes: requiredScopes,
				Logic:  policy.ScopeLogic(logic),
			})
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

// ClaimsGuard enforces declarative claim rules against authenticated claims.
func ClaimsGuard(rules ...ClaimRule) router.MiddlewareFunc {
	return ClaimsGuardWithMappings(nil, rules...)
}

// ClaimsGuardWithMappings enforces claim rules using canonical claim mappings.
func ClaimsGuardWithMappings(mappings map[string][]string, rules ...ClaimRule) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			claimsValue := c.Get(authentication.ClaimsKey)
			if claimsValue == nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{"error": "missing authentication"})
			}
			claims, ok := claimsValue.(*auth.Claims)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{"error": "invalid authentication"})
			}
			for _, rule := range rules {
				if strings.TrimSpace(rule.Claim) == "" {
					continue
				}
				ok, err := policy.EvaluateClaimRule(c.Request().Context(), subjectFromClaimsWithMappings(claims, mappings), httpValueResolver{ctx: c}, rule)
				if err != nil {
					if requestLogger, loggerOK := c.Get("logger").(logger.Logger); loggerOK && requestLogger != nil {
						requestLogger.WithContext(c.Request().Context()).Warn("claim evaluation failed", "error", err)
					}
					return c.JSON(http.StatusForbidden, map[string]interface{}{"error": "claim evaluation failed"})
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

// ClaimsGuardFromConfig builds a claim guard from auth configuration.
func ClaimsGuardFromConfig(authCfg authconfig.Config) router.MiddlewareFunc {
	rules := ClaimRulesFromConfig(authCfg)
	if len(rules) == 0 {
		return ClaimsGuardWithMappings(authCfg.Claims.Mappings)
	}
	return ClaimsGuardWithMappings(authCfg.Claims.Mappings, rules...)
}

// ClaimRulesFromConfig converts auth config claim rules into policy claim rules.
func ClaimRulesFromConfig(authCfg authconfig.Config) []ClaimRule {
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
			Source:   policy.ValueSource(src),
			Key:      rule.Key,
			Operator: policy.Operator(op),
			Values:   rule.Values,
			Optional: rule.Optional,
		})
	}
	return rules
}

type httpValueResolver struct {
	ctx router.Context
}

func (r httpValueResolver) Value(_ context.Context, source policy.ValueSource, key string) (string, error) {
	return valueFromSource(r.ctx, source, key)
}

func valueFromSource(c router.Context, source policy.ValueSource, key string) (string, error) {
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return "", fmt.Errorf("rule key is required")
	}
	switch policy.ValueSource(strings.ToLower(strings.TrimSpace(string(source)))) {
	case policy.ValueSourceRoute:
		return strings.TrimSpace(c.Param(trimmedKey)), nil
	case policy.ValueSourceHeader:
		return strings.TrimSpace(c.Request().Header.Get(trimmedKey)), nil
	case policy.ValueSourceQuery:
		return strings.TrimSpace(c.Query(trimmedKey)), nil
	default:
		return "", fmt.Errorf("unsupported claim source %q", source)
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

// HasAllScopes reports whether userScopes contains every required scope.
func HasAllScopes(userScopes, requiredScopes []string) bool {
	return policy.EvaluateScopes(policy.Subject{Scopes: userScopes}, policy.ScopeRequirement{
		Scopes: requiredScopes,
		Logic:  policy.ScopeLogicAND,
	})
}

// HasAnyScope reports whether userScopes contains at least one required scope.
func HasAnyScope(userScopes, requiredScopes []string) bool {
	return policy.EvaluateScopes(policy.Subject{Scopes: userScopes}, policy.ScopeRequirement{
		Scopes: requiredScopes,
		Logic:  policy.ScopeLogicOR,
	})
}

func subjectFromClaims(claims *auth.Claims) policy.Subject {
	return subjectFromClaimsWithMappings(claims, nil)
}

func subjectFromClaimsWithMappings(claims *auth.Claims, mappings map[string][]string) policy.Subject {
	if claims == nil {
		return policy.Subject{}
	}
	attrs := map[string]string{}
	for key, value := range claims.Custom {
		if stringValue, ok := claimToString(value); ok {
			attrs[key] = stringValue
		}
	}
	for canonical, aliases := range mappings {
		for _, alias := range aliases {
			if value, ok := attrs[alias]; ok && value != "" {
				attrs[canonical] = value
				break
			}
		}
	}
	return policy.Subject{
		ID:       strings.TrimSpace(claims.Subject),
		TenantID: strings.TrimSpace(claims.TenantID),
		Scopes:   append([]string(nil), claims.Scopes...),
		Roles:    append([]string(nil), claims.Roles...),
		Attrs:    attrs,
	}
}
