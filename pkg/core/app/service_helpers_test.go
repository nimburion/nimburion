package app

import "testing"

func TestGetService(t *testing.T) {
	runtime := &Runtime{}
	runtime.RegisterService("cache", "redis")

	service, ok := GetService[string](runtime, "cache")
	if !ok {
		t.Fatal("expected service lookup to succeed")
	}
	if service != "redis" {
		t.Fatalf("expected redis, got %q", service)
	}
}

func TestGetService_WrongType(t *testing.T) {
	runtime := &Runtime{}
	runtime.RegisterService("cache", "redis")

	_, ok := GetService[int](runtime, "cache")
	if ok {
		t.Fatal("expected service lookup to fail for wrong type")
	}
}

func TestGetService_MissingService(t *testing.T) {
	runtime := &Runtime{}

	_, ok := GetService[string](runtime, "missing")
	if ok {
		t.Fatal("expected service lookup to fail for missing service")
	}
}
