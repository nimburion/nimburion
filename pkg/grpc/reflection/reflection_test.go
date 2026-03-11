package reflection

import (
	"testing"

	"google.golang.org/grpc"
)

func TestRegistrationRegistersReflection(t *testing.T) {
	srv := grpc.NewServer()
	if err := Registration().Fn(srv); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
