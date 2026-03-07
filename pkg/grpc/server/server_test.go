package server

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestBuildRequiresAddressOrListener(t *testing.T) {
	_, err := Build(Options{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRegistersServices(t *testing.T) {
	called := false
	srv, err := Build(Options{
		Listener: bufconn.Listen(1024),
		Registrations: []Registration{
			{
				Name: "test",
				Fn: func(server *grpc.Server) error {
					called = server != nil
					return nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called || srv == nil {
		t.Fatal("expected registration to run")
	}
}

func TestRunUsesCoreLifecycle(t *testing.T) {
	listener := bufconn.Listen(1024)
	srv, err := Build(Options{Listener: listener})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, srv, Options{Listener: listener, AppName: "grpc-app"})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop")
	}
}
