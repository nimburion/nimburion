package auth

import (
	"context"
	"errors"
	"strings"

	frameworkauth "github.com/nimburion/nimburion/pkg/auth"
	grpcmetadata "github.com/nimburion/nimburion/pkg/grpc/metadata"
	grpcstatusmap "github.com/nimburion/nimburion/pkg/grpc/status"
	grpcstream "github.com/nimburion/nimburion/pkg/grpc/stream"
	"github.com/nimburion/nimburion/pkg/policy"
	"github.com/nimburion/nimburion/pkg/tenant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type claimsContextKey struct{}

// ClaimsFromContext returns JWT claims attached by authentication interceptors.
func ClaimsFromContext(ctx context.Context) (*frameworkauth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*frameworkauth.Claims)
	return claims, ok
}

// AuthenticateUnary validates bearer metadata and injects claims into the context.
func AuthenticateUnary(validator frameworkauth.JWTValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		nextCtx, err := authenticateContext(ctx, validator)
		if err != nil {
			return nil, err
		}
		return handler(nextCtx, req)
	}
}

// AuthenticateStream validates bearer metadata and injects claims into the stream context.
func AuthenticateStream(validator frameworkauth.JWTValidator) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		nextCtx, err := authenticateContext(stream.Context(), validator)
		if err != nil {
			return err
		}
		return handler(srv, grpcstream.WrapContext(stream, nextCtx))
	}
}

// RequireScopesUnary enforces scope requirements on unary RPCs.
func RequireScopesUnary(requirement policy.ScopeRequirement) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := authorizeContext(ctx, requirement); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// RequireScopesStream enforces scope requirements on streaming RPCs.
func RequireScopesStream(requirement policy.ScopeRequirement) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := authorizeContext(stream.Context(), requirement); err != nil {
			return err
		}
		return handler(srv, stream)
	}
}

func authenticateContext(ctx context.Context, validator frameworkauth.JWTValidator) (context.Context, error) {
	if validator == nil {
		return nil, grpcstatus.Error(codes.Unauthenticated, "jwt validator is required")
	}
	token, err := bearerToken(grpcmetadata.FromIncomingContext(ctx))
	if err != nil {
		return nil, err
	}
	claims, err := validator.Validate(ctx, token)
	if err != nil {
		return nil, grpcstatus.Error(codes.Unauthenticated, "invalid token")
	}
	nextCtx := frameworkauth.WithClaims(ctx, claims)
	nextCtx = context.WithValue(nextCtx, claimsContextKey{}, claims)
	if strings.TrimSpace(claims.TenantID) != "" {
		nextCtx = tenant.WithContext(nextCtx, tenant.Context{
			Tenant:  tenant.Identity{ID: claims.TenantID},
			Subject: claims.Subject,
		})
	}
	return nextCtx, nil
}

func authorizeContext(ctx context.Context, requirement policy.ScopeRequirement) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return grpcstatus.Error(codes.Unauthenticated, "missing authentication")
	}
	subject := policy.Subject{
		ID:       strings.TrimSpace(claims.Subject),
		TenantID: strings.TrimSpace(claims.TenantID),
		Scopes:   append([]string(nil), claims.Scopes...),
		Roles:    append([]string(nil), claims.Roles...),
	}
	if !policy.EvaluateScopes(subject, requirement) {
		return grpcstatus.Error(codes.PermissionDenied, "insufficient permissions")
	}
	return nil
}

func bearerToken(md grpcmetadata.Map) (string, error) {
	authorization := strings.TrimSpace(md.First("authorization"))
	if authorization == "" {
		return "", grpcstatus.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", grpcstatus.Error(codes.Unauthenticated, "invalid authorization metadata")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", grpcstatus.Error(codes.Unauthenticated, "missing bearer token")
	}
	return token, nil
}

// UnaryAuditContext derives a minimal audit record skeleton from authenticated context.
func UnaryAuditContext(ctx context.Context, action string) (string, string, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return "", "", errors.New("missing claims")
	}
	if action == "" {
		return "", "", errors.New("audit action is required")
	}
	return claims.Subject, claims.TenantID, nil
}

// StatusError converts framework auth failures through the shared gRPC status mapper.
func StatusError(err error) error {
	return grpcstatusmap.Error(err)
}
