// Package policy defines authorization policy contracts and evaluators.
package policy

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

// Subject describes the identity attributes evaluated by authorization policies.
type Subject struct {
	ID       string
	TenantID string
	Scopes   []string
	Roles    []string
	Attrs    map[string]string
}

// ScopeLogic defines how required scopes are combined.
type ScopeLogic int

const (
	// ScopeLogicAND requires all listed scopes.
	ScopeLogicAND ScopeLogic = iota
	// ScopeLogicOR requires at least one listed scope.
	ScopeLogicOR
)

// ScopeRequirement declares the scopes required for one policy check.
type ScopeRequirement struct {
	Scopes []string
	Logic  ScopeLogic
}

// EvaluateScopes reports whether subject satisfies requirement.
func EvaluateScopes(subject Subject, requirement ScopeRequirement) bool {
	requiredScopes := normalizeStrings(requirement.Scopes)
	if len(requiredScopes) == 0 {
		return true
	}

	userScopes := normalizeStrings(subject.Scopes)
	scopeSet := make(map[string]bool, len(userScopes))
	for _, scope := range userScopes {
		scopeSet[scope] = true
	}

	if requirement.Logic == ScopeLogicOR {
		for _, required := range requiredScopes {
			if scopeSet[required] {
				return true
			}
		}
		return false
	}

	for _, required := range requiredScopes {
		if !scopeSet[required] {
			return false
		}
	}
	return true
}

type (
	// ValueSource identifies where a policy value is resolved from.
	ValueSource string
	// Operator identifies the predicate applied to a resolved claim.
	Operator string
)

const (
	// ValueSourceSubject resolves values from subject attributes.
	ValueSourceSubject ValueSource = "subject"
	// ValueSourceRoute resolves values from route parameters.
	ValueSourceRoute ValueSource = "route"
	// ValueSourceHeader resolves values from request headers.
	ValueSourceHeader ValueSource = "header"
	// ValueSourceQuery resolves values from query parameters.
	ValueSourceQuery ValueSource = "query"

	// OperatorRequired requires a claim to be present.
	OperatorRequired Operator = "required"
	// OperatorEquals requires a claim to equal one resolved value.
	OperatorEquals Operator = "equals"
	// OperatorOneOf requires a claim to match one configured value.
	OperatorOneOf Operator = "one_of"
	// OperatorRegex requires a claim to match a regular expression.
	OperatorRegex Operator = "regex"
)

// ValueResolver resolves comparison values for claim rules.
type ValueResolver interface {
	Value(ctx context.Context, source ValueSource, key string) (string, error)
}

// ClaimRule declares one subject claim predicate.
type ClaimRule struct {
	Claim    string
	Aliases  []string
	Source   ValueSource
	Key      string
	Operator Operator
	Values   []string
	Optional bool
}

// EvaluateClaimRule evaluates one claim rule against subject and resolver.
func EvaluateClaimRule(ctx context.Context, subject Subject, resolver ValueResolver, rule ClaimRule) (bool, error) {
	claimValue := ResolveSubjectValue(subject, rule.Claim, rule.Aliases)
	operator := strings.ToLower(strings.TrimSpace(string(rule.Operator)))
	if operator == "" {
		operator = string(OperatorRequired)
	}

	switch Operator(operator) {
	case OperatorRequired:
		return claimValue != "", nil
	case OperatorEquals:
		if resolver == nil {
			return false, errors.New("value resolver is required")
		}
		expected, err := resolver.Value(ctx, rule.Source, rule.Key)
		if err != nil {
			return false, err
		}
		return claimValue != "" && claimValue == expected, nil
	case OperatorOneOf:
		if claimValue == "" {
			return false, nil
		}
		for _, candidate := range rule.Values {
			if claimValue == strings.TrimSpace(candidate) {
				return true, nil
			}
		}
		return false, nil
	case OperatorRegex:
		if claimValue == "" {
			return false, nil
		}
		if len(rule.Values) == 0 || strings.TrimSpace(rule.Values[0]) == "" {
			return false, errors.New("regex operator requires one pattern in values")
		}
		return regexp.MatchString(strings.TrimSpace(rule.Values[0]), claimValue)
	default:
		return false, errors.New("unsupported claim operator")
	}
}

// ResolveSubjectValue resolves one named subject attribute using aliases as fallbacks.
func ResolveSubjectValue(subject Subject, name string, aliases []string) string {
	candidates := append([]string{name}, aliases...)
	for _, candidate := range candidates {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "tenant_id":
			if subject.TenantID != "" {
				return subject.TenantID
			}
		case "subject":
			if subject.ID != "" {
				return subject.ID
			}
		case "roles":
			if len(subject.Roles) > 0 {
				return strings.Join(subject.Roles, ",")
			}
		case "scopes":
			if len(subject.Scopes) > 0 {
				return strings.Join(subject.Scopes, " ")
			}
		default:
			if value := strings.TrimSpace(subject.Attrs[strings.TrimSpace(candidate)]); value != "" {
				return value
			}
		}
	}
	return ""
}

// Authorizer decides whether subject may perform action on resource.
type Authorizer interface {
	Allow(ctx context.Context, subject Subject, action, resource string, attrs map[string]string) (Decision, error)
}

// Decision is the result of an authorization check.
type Decision struct {
	Allowed bool
	Reason  string
}

func normalizeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
