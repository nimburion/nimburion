# pkg/grpc/health

Optional gRPC health-service exposure backed by the shared `pkg/health.Registry`.

The package is opt-in and does not become part of the runtime unless its
registration is explicitly added to a gRPC server assembly.
