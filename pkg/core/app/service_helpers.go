package app

import "github.com/nimburion/nimburion/pkg/core/feature"

// GetService retrieves one runtime service and performs a typed assertion in one step.
func GetService[T any](runtime feature.Runtime, name string) (T, bool) {
	var zero T
	if runtime == nil {
		return zero, false
	}
	service, ok := runtime.LookupService(name)
	if !ok {
		return zero, false
	}
	typed, ok := service.(T)
	if !ok {
		return zero, false
	}
	return typed, true
}
