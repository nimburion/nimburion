# pkg/grpc

First-class gRPC transport family root.

`pkg/grpc` owns gRPC-specific server bootstrap, interception, metadata handling,
validation layering, and framework-to-gRPC status mapping. Applications that do
not use gRPC can ignore this family entirely.
