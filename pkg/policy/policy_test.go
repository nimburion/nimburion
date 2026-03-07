package policy

import (
	"context"
	"testing"
)

type stubResolver struct {
	values map[string]string
}

func (r stubResolver) Value(_ context.Context, source ValueSource, key string) (string, error) {
	return r.values[string(source)+":"+key], nil
}

func TestEvaluateScopes(t *testing.T) {
	subject := Subject{Scopes: []string{"read", "write"}}
	if !EvaluateScopes(subject, ScopeRequirement{Scopes: []string{"read"}, Logic: ScopeLogicAND}) {
		t.Fatal("expected required scope to pass")
	}
	if EvaluateScopes(subject, ScopeRequirement{Scopes: []string{"admin"}, Logic: ScopeLogicAND}) {
		t.Fatal("expected missing scope to fail")
	}
}

func TestEvaluateClaimRuleEquals(t *testing.T) {
	subject := Subject{TenantID: "tenant-a"}
	ok, err := EvaluateClaimRule(context.Background(), subject, stubResolver{values: map[string]string{"route:tenantId": "tenant-a"}}, ClaimRule{
		Claim:    "tenant_id",
		Source:   ValueSourceRoute,
		Key:      "tenantId",
		Operator: OperatorEquals,
	})
	if err != nil {
		t.Fatalf("evaluate claim rule: %v", err)
	}
	if !ok {
		t.Fatal("expected tenant rule to pass")
	}
}
