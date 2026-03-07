package health

import (
	"context"
	"testing"

	frameworkhealth "github.com/nimburion/nimburion/pkg/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

func TestCheckMapsRegistryReadiness(t *testing.T) {
	registry := frameworkhealth.NewRegistry()
	registry.RegisterFunc("db", func(ctx context.Context) frameworkhealth.CheckResult {
		return frameworkhealth.CheckResult{Name: "db", Status: frameworkhealth.StatusDegraded}
	})

	resp, err := NewService(registry).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("expected SERVING for degraded, got %v", resp.Status)
	}
}
