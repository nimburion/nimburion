package metadata

import (
	"context"
	"strings"

	grpcmetadata "google.golang.org/grpc/metadata"
)

// Map is a normalized metadata view.
type Map map[string][]string

// FromIncomingContext extracts incoming metadata as a normalized map.
func FromIncomingContext(ctx context.Context) Map {
	md, ok := grpcmetadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	return clone(md)
}

// FromOutgoingContext extracts outgoing metadata as a normalized map.
func FromOutgoingContext(ctx context.Context) Map {
	md, ok := grpcmetadata.FromOutgoingContext(ctx)
	if !ok {
		return nil
	}
	return clone(md)
}

// Get returns all values associated with one metadata key.
func (m Map) Get(key string) []string {
	if len(m) == 0 {
		return nil
	}
	return append([]string(nil), m[strings.ToLower(strings.TrimSpace(key))]...)
}

// First returns the first value associated with one metadata key.
func (m Map) First(key string) string {
	values := m.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// InjectIncoming stores metadata into an incoming context.
func InjectIncoming(ctx context.Context, values Map) context.Context {
	return grpcmetadata.NewIncomingContext(ctx, grpcmetadata.MD(values))
}

// InjectOutgoing stores metadata into an outgoing context.
func InjectOutgoing(ctx context.Context, values Map) context.Context {
	return grpcmetadata.NewOutgoingContext(ctx, grpcmetadata.MD(values))
}

func clone(md grpcmetadata.MD) Map {
	out := make(Map, len(md))
	for key, values := range md {
		out[strings.ToLower(strings.TrimSpace(key))] = append([]string(nil), values...)
	}
	return out
}
