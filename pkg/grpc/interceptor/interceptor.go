package interceptor

import (
	"context"

	grpcmetadata "github.com/nimburion/nimburion/pkg/grpc/metadata"
	grpcstatusmap "github.com/nimburion/nimburion/pkg/grpc/status"
	grpcvalidation "github.com/nimburion/nimburion/pkg/grpc/validation"
	"google.golang.org/grpc"
)

// ChainUnary composes unary interceptors in order.
func ChainUnary(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		chained := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			next := chained
			current := interceptors[i]
			chained = func(current grpc.UnaryServerInterceptor, next grpc.UnaryHandler) grpc.UnaryHandler {
				return func(ctx context.Context, req any) (any, error) {
					return current(ctx, req, info, next)
				}
			}(current, next)
		}
		return chained(ctx, req)
	}
}

// ChainStream composes streaming interceptors in order.
func ChainStream(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		chained := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			next := chained
			current := interceptors[i]
			chained = func(current grpc.StreamServerInterceptor, next grpc.StreamHandler) grpc.StreamHandler {
				return func(srv any, stream grpc.ServerStream) error {
					return current(srv, stream, info, next)
				}
			}(current, next)
		}
		return chained(srv, stream)
	}
}

// ValidationUnaryInterceptor applies layered request validation before business handling.
func ValidationUnaryInterceptor(pipeline grpcvalidation.Pipeline) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := pipeline.Validate(ctx, req); err != nil {
			return nil, grpcstatusmap.Error(err)
		}
		return handler(ctx, req)
	}
}

// StatusUnaryInterceptor maps framework errors to gRPC status errors.
func StatusUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, grpcstatusmap.Error(err)
		}
		return resp, nil
	}
}

// MetadataUnaryInterceptor extracts incoming metadata and stores it in the context.
func MetadataUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md := grpcmetadata.FromIncomingContext(ctx)
		ctx = context.WithValue(ctx, metadataContextKey{}, md)
		return handler(ctx, req)
	}
}

type metadataContextKey struct{}

// ContextMetadata returns metadata previously attached by MetadataUnaryInterceptor.
func ContextMetadata(ctx context.Context) grpcmetadata.Map {
	values, ok := ctx.Value(metadataContextKey{}).(grpcmetadata.Map)
	if !ok {
		return nil
	}
	return values
}
