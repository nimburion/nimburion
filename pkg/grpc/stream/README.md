# pkg/grpc/stream

Explicit streaming helpers for gRPC server streams.

The package keeps stream-only concerns separate from unary RPC handling and
provides context-wrapping helpers for cancellation, deadline, and metadata
propagation.
