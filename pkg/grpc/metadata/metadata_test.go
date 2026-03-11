package metadata

import (
	"context"
	"testing"

	grpcmetadata "google.golang.org/grpc/metadata"
)

func TestFromIncomingContextNormalizesKeys(t *testing.T) {
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs("X-Request-Id", "abc"))

	md := FromIncomingContext(ctx)

	if got := md.First("x-request-id"); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
}

func TestInjectOutgoing(t *testing.T) {
	ctx := InjectOutgoing(context.Background(), Map{"x-tenant-id": {"tenant-1"}})

	md := FromOutgoingContext(ctx)
	if got := md.First("x-tenant-id"); got != "tenant-1" {
		t.Fatalf("expected tenant-1, got %q", got)
	}
}
