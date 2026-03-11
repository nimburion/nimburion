# pkg/grpc/auth

Transport-specific gRPC authentication and authorization glue.

This package validates bearer metadata, propagates shared claims and tenant
context, and exposes unary and streaming scope enforcement interceptors without
redefining shared auth, policy, or tenant contracts.
