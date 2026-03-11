package stream

import (
	"context"
	"testing"
	"time"

	grpcmetadata "google.golang.org/grpc/metadata"
)

type fakeServerStream struct {
	ctx context.Context
}

func (f fakeServerStream) SetHeader(grpcmetadata.MD) error  { return nil }
func (f fakeServerStream) SendHeader(grpcmetadata.MD) error { return nil }
func (f fakeServerStream) SetTrailer(grpcmetadata.MD)       {}
func (f fakeServerStream) Context() context.Context         { return f.ctx }
func (f fakeServerStream) SendMsg(any) error                { return nil }
func (f fakeServerStream) RecvMsg(any) error                { return nil }

func TestWrapContextReplacesStreamContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wrapped := WrapContext(fakeServerStream{ctx: context.Background()}, ctx)
	if wrapped.Context() != ctx {
		t.Fatal("expected wrapped context")
	}
}

func TestIncomingMetadata(t *testing.T) {
	ctx := grpcmetadata.NewIncomingContext(context.Background(), grpcmetadata.Pairs("x-request-id", "abc"))
	stream := fakeServerStream{ctx: ctx}
	if got := IncomingMetadata(stream).First("x-request-id"); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
}
