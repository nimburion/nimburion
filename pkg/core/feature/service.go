package feature

import "fmt"

// Service looks up a runtime service by name and type-asserts it to T.
//
// It is the typed front door over Runtime.LookupService: instead of writing
// hand-rolled `any` assertions at each call site, features depend on a single
// generic helper that resolves the registered value and converts it safely.
//
// On a missing service or a type mismatch it returns the zero value of T and
// false, mirroring the comma-ok convention of the underlying registry.
func Service[T any](r Runtime, name string) (T, bool) {
	var zero T
	if r == nil {
		return zero, false
	}
	value, ok := r.LookupService(name)
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}

// MustService looks up a runtime service by name and type-asserts it to T,
// panicking with a clear message when the service is missing or has the wrong
// type.
//
// A failure here always indicates a wiring error: a feature declared a hard
// dependency on a service that was never registered (or was registered with a
// different type). Use Service for optional dependencies where a miss is a
// normal, recoverable condition.
func MustService[T any](r Runtime, name string) T {
	value, ok := Service[T](r, name)
	if !ok {
		var zero T
		panic(fmt.Sprintf(
			"feature.MustService: service %q is not registered as %T (wiring error)",
			name, zero,
		))
	}
	return value
}
