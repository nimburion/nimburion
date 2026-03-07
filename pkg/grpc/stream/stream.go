package stream

import (
	"context"
	"time"

	grpcmetadata "github.com/nimburion/nimburion/pkg/grpc/metadata"
	"google.golang.org/grpc"
)

// WrappedServerStream overrides the context carried by one grpc.ServerStream.
type WrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// WrapContext returns a stream wrapper with a replaced context.
func WrapContext(stream grpc.ServerStream, ctx context.Context) *WrappedServerStream {
	return &WrappedServerStream{
		ServerStream: stream,
		ctx:          ctx,
	}
}

// Context returns the wrapped stream context.
func (s *WrappedServerStream) Context() context.Context {
	return s.ctx
}

// IncomingMetadata returns normalized incoming metadata for the stream.
func IncomingMetadata(stream grpc.ServerStream) grpcmetadata.Map {
	return grpcmetadata.FromIncomingContext(stream.Context())
}

// Deadline returns the stream deadline, if any.
func Deadline(stream grpc.ServerStream) (time.Time, bool) {
	return stream.Context().Deadline()
}
