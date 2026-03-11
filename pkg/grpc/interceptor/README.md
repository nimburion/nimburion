# pkg/grpc/interceptor

Owns unary and streaming interception contracts for the gRPC transport family.

It keeps unary and streaming pipelines explicit and provides transport-facing
interceptors for metadata, validation, and status mapping.
