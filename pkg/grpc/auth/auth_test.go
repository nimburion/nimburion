package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcmetadata "google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"

	frameworkauth "github.com/nimburion/nimburion/pkg/auth"

	"github.com/nimburion/nimburion/pkg/policy"
	"github.com/nimburion/nimburion/pkg/tenant"
)

type stubValidator struct {
	claims *frameworkauth.Claims
	err    error
}

func (s stubValidator) Validate(_ context.Context, _ string) (*frameworkauth.Claims, error) {
	return s.claims, s.err
}

type fakeServerStream struct {
	ctx context.Context
}

func (f fakeServerStream) SetHeader(grpcmetadata.MD) error  { return nil }
func (f fakeServerStream) SendHeader(grpcmetadata.MD) error { return nil }
func (f fakeServerStream) SetTrailer(grpcmetadata.MD)       {}
func (f fakeServerStream) Context() context.Context         { return f.ctx }
func (f fakeServerStream) SendMsg(any) error                { return nil }
func (f fakeServerStream) RecvMsg(any) error                { return nil }

func TestAuthenticateUnaryInjectsClaimsAndTenant(t *testing.T) {
	interceptor := AuthenticateUnary(stubValidator{
		claims: &frameworkauth.Claims{
			Subject:  "user-1",
			TenantID: "tenant-1",
			Scopes:   []string{"jobs:run"},
		},
	})
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs("authorization", "Bearer token"))
	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		if !ok || claims.Subject != "user-1" {
			t.Fatal("expected claims in context")
		}
		tenantCtx, ok := tenant.FromContext(ctx)
		if !ok || tenantCtx.Tenant.ID != "tenant-1" {
			t.Fatal("expected tenant context")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireScopesUnaryRejectsMissingScope(t *testing.T) {
	ctx := context.WithValue(context.Background(), claimsContextKey{}, &frameworkauth.Claims{Scopes: []string{"read"}})
	interceptor := RequireScopesUnary(policy.ScopeRequirement{Scopes: []string{"write"}, Logic: policy.ScopeLogicAND})
	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})
	st, _ := grpcstatus.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %s", st.Code())
	}
}

func TestAuthenticateStreamWrapsContext(t *testing.T) {
	interceptor := AuthenticateStream(stubValidator{
		claims: &frameworkauth.Claims{Subject: "stream-user"},
	})
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs("authorization", "Bearer token"))
	err := interceptor(nil, fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{}, func(_ any, stream grpc.ServerStream) error {
		claims, ok := ClaimsFromContext(stream.Context())
		if !ok || claims.Subject != "stream-user" {
			t.Fatal("expected claims in stream context")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
