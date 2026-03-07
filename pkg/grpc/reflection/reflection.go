package reflection

import (
	grpcserver "github.com/nimburion/nimburion/pkg/grpc/server"
	"google.golang.org/grpc"
	grpcreflection "google.golang.org/grpc/reflection"
)

// Registration returns one optional gRPC registration for server reflection.
func Registration() grpcserver.Registration {
	return grpcserver.Registration{
		Name: "grpc_reflection",
		Fn: func(srv *grpc.Server) error {
			grpcreflection.Register(srv)
			return nil
		},
	}
}
