package policy

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

type Subject struct {
	ID       string
	TenantID string
	Scopes   []string
	Roles    []string
	Attrs    map[string]string
}

type ScopeLogic int

const (
	ScopeLogicAND ScopeLogic = iota
	ScopeLogicOR
)

type ScopeRequirement struct {
	Scopes []string
	Logic  ScopeLogic
}

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

type ValueSource string
type Operator string

const (
	ValueSourceSubject ValueSource = "subject"
	ValueSourceRoute   ValueSource = "route"
	ValueSourceHeader  ValueSource = "header"
	ValueSourceQuery   ValueSource = "query"

	OperatorRequired Operator = "required"
	OperatorEquals   Operator = "equals"
	OperatorOneOf    Operator = "one_of"
	OperatorRegex    Operator = "regex"
)

type ValueResolver interface {
	Value(ctx context.Context, source ValueSource, key string) (string, error)
}

type ClaimRule struct {
	Claim    string
	Aliases  []string
	Source   ValueSource
	Key      string
	Operator Operator
	Values   []string
	Optional bool
}

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

type Authorizer interface {
	Allow(ctx context.Context, subject Subject, action string, resource string, attrs map[string]string) (Decision, error)
}

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
