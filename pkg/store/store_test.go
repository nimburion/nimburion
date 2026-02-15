package store

import (
	"context"
	"testing"
)

type fakeAdapter struct{}

func (f *fakeAdapter) HealthCheck(context.Context) error { return nil }
func (f *fakeAdapter) Close() error                      { return nil }

func TestAdapterContract(t *testing.T) {
	var _ Adapter = (*fakeAdapter)(nil)

	a := &fakeAdapter{}
	if err := a.HealthCheck(context.Background()); err != nil {
		t.Fatalf("healthcheck: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
