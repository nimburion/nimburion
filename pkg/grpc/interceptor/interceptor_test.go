package interceptor

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcmetadata "google.golang.org/grpc/metadata"
	gstatus "google.golang.org/grpc/status"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	grpcvalidation "github.com/nimburion/nimburion/pkg/grpc/validation"
)

func TestChainUnaryPreservesOrder(t *testing.T) {
	var calls []string
	chain := ChainUnary(
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			calls = append(calls, "a")
			return handler(ctx, req)
		},
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			calls = append(calls, "b")
			return handler(ctx, req)
		},
	)

	_, err := chain(context.Background(), "req", &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
		calls = append(calls, "handler")
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := []string{"a", "b", "handler"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected order: %#v", calls)
	}
}

func TestValidationUnaryInterceptorDistinguishesLayer(t *testing.T) {
	interceptor := ValidationUnaryInterceptor(grpcvalidation.Pipeline{
		Contract: []grpcvalidation.Validator{
			grpcvalidation.ValidatorFunc(func(context.Context, any) error {
				return errors.New("schema mismatch")
			}),
		},
	})
	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})
	st, _ := gstatus.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %s", st.Code())
	}
}

func TestStatusUnaryInterceptorMapsAppError(t *testing.T) {
	interceptor := StatusUnaryInterceptor()
	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
		return nil, coreerrors.New("auth.forbidden", nil, nil).WithMessage("forbidden").WithHTTPStatus(403)
	})
	st, _ := gstatus.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %s", st.Code())
	}
}

func TestMetadataUnaryInterceptorExposesIncomingMetadata(t *testing.T) {
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs("x-request-id", "abc"))
	interceptor := MetadataUnaryInterceptor()
	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		if got := ContextMetadata(ctx).First("x-request-id"); got != "abc" {
			t.Fatalf("expected metadata in context, got %q", got)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
