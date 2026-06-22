package feature

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/featureflag"
	"github.com/nimburion/nimburion/pkg/health"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/observability/metrics"
	"github.com/nimburion/nimburion/pkg/observability/tracing"
)

// fakeRuntime is a minimal Runtime implementation backed by an in-memory
// service map. Only the service-registry surface is exercised by these tests;
// the rest of the interface is satisfied with zero-value stubs.
type fakeRuntime struct {
	services map[string]any
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{services: make(map[string]any)}
}

func (f *fakeRuntime) AppName() string                                      { return "" }
func (f *fakeRuntime) ConfigValue() any                                     { return nil }
func (f *fakeRuntime) DebugEnabled() bool                                   { return false }
func (f *fakeRuntime) Log() logger.Logger                                   { return nil }
func (f *fakeRuntime) FeatureFlags() *featureflag.Registry                  { return nil }
func (f *fakeRuntime) RuntimePosture() *featureflag.RuntimePosture          { return nil }
func (f *fakeRuntime) FailureInjector() FailureInjector                     { return nil }
func (f *fakeRuntime) DeploymentPosture() DeploymentPosture                 { return nil }
func (f *fakeRuntime) SignalCatalog() SignalCatalog                         { return nil }
func (f *fakeRuntime) HealthRegistry() *health.Registry                     { return nil }
func (f *fakeRuntime) MetricsRegistry() *metrics.Registry                   { return nil }
func (f *fakeRuntime) TracerProvider() *tracing.TracerProvider              { return nil }
func (f *fakeRuntime) IntrospectionRegistry() IntrospectionRegistry         { return nil }
func (f *fakeRuntime) RegisterService(name string, service any)             { f.services[name] = service }

func (f *fakeRuntime) LookupService(name string) (any, bool) {
	service, ok := f.services[name]
	return service, ok
}

type stubCache struct {
	addr string
}

func TestService_Hit(t *testing.T) {
	t.Parallel()

	rt := newFakeRuntime()
	want := &stubCache{addr: "localhost:6379"}
	rt.RegisterService("cache", want)

	got, ok := Service[*stubCache](rt, "cache")
	if !ok {
		t.Fatal("expected typed lookup to succeed")
	}
	if got != want {
		t.Fatalf("Service() = %v, want %v", got, want)
	}
}

func TestService_Miss(t *testing.T) {
	t.Parallel()

	rt := newFakeRuntime()

	got, ok := Service[*stubCache](rt, "absent")
	if ok {
		t.Fatal("expected typed lookup to miss")
	}
	if got != nil {
		t.Fatalf("Service() = %v, want zero value", got)
	}
}

func TestService_TypeMismatch(t *testing.T) {
	t.Parallel()

	rt := newFakeRuntime()
	rt.RegisterService("cache", "a plain string, not a *stubCache")

	got, ok := Service[*stubCache](rt, "cache")
	if ok {
		t.Fatal("expected typed lookup to fail on type mismatch")
	}
	if got != nil {
		t.Fatalf("Service() = %v, want zero value", got)
	}
}

func TestService_NilRuntime(t *testing.T) {
	t.Parallel()

	got, ok := Service[*stubCache](nil, "cache")
	if ok || got != nil {
		t.Fatalf("Service(nil) = %v, %v; want zero, false", got, ok)
	}
}

func TestMustService_Hit(t *testing.T) {
	t.Parallel()

	rt := newFakeRuntime()
	want := &stubCache{addr: "localhost:6379"}
	rt.RegisterService("cache", want)

	if got := MustService[*stubCache](rt, "cache"); got != want {
		t.Fatalf("MustService() = %v, want %v", got, want)
	}
}

func TestMustService_PanicsOnMiss(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustService to panic on miss")
		}
	}()

	rt := newFakeRuntime()
	_ = MustService[*stubCache](rt, "absent")
}

func TestMustService_PanicsOnTypeMismatch(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustService to panic on type mismatch")
		}
	}()

	rt := newFakeRuntime()
	rt.RegisterService("cache", 42)
	_ = MustService[*stubCache](rt, "cache")
}
