package health

import (
	"context"

	grpcserver "github.com/nimburion/nimburion/pkg/grpc/server"
	frameworkhealth "github.com/nimburion/nimburion/pkg/health"
	"google.golang.org/grpc"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

// Service exposes the shared health registry as a gRPC health service.
type Service struct {
	grpc_health_v1.UnimplementedHealthServer
	registry *frameworkhealth.Registry
}

// NewService creates a gRPC health service backed by the shared health registry.
func NewService(registry *frameworkhealth.Registry) *Service {
	if registry == nil {
		registry = frameworkhealth.NewRegistry()
	}
	return &Service{registry: registry}
}

// Registration returns one optional gRPC registration for the health service.
func Registration(registry *frameworkhealth.Registry) grpcserver.Registration {
	return grpcserver.Registration{
		Name: "grpc_health",
		Fn: func(srv *grpc.Server) error {
			grpc_health_v1.RegisterHealthServer(srv, NewService(registry))
			return nil
		},
	}
}

// Check reports SERVING for healthy or degraded shared registry state, NOT_SERVING otherwise.
func (s *Service) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	result := s.registry.Check(ctx)
	status := grpc_health_v1.HealthCheckResponse_SERVING
	if !result.IsReady() {
		status = grpc_health_v1.HealthCheckResponse_NOT_SERVING
	}
	return &grpc_health_v1.HealthCheckResponse{Status: status}, nil
}

// Watch is not implemented in this first milestone and returns the current snapshot.
func (s *Service) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	resp, err := s.Check(stream.Context(), req)
	if err != nil {
		return err
	}
	if err := stream.Send(resp); err != nil {
		return err
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}
