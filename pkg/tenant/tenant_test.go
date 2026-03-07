package tenant

import (
	"context"
	"testing"
)

func TestContextRoundTrip(t *testing.T) {
	tc := Context{
		Tenant:  Identity{ID: "tenant-a"},
		Subject: "user-1",
		Attrs:   map[string]string{"region": "eu"},
	}

	ctx := WithContext(context.Background(), tc)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected tenant context")
	}
	if got.Tenant.ID != "tenant-a" || got.Subject != "user-1" {
		t.Fatalf("unexpected tenant context: %+v", got)
	}
}
