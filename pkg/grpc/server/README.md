# pkg/grpc/server

Owns gRPC runtime bootstrap, service registration, and graceful shutdown.

This package integrates with `pkg/core/app` so a gRPC-only service can run
through the shared application lifecycle without importing HTTP runtime code.
