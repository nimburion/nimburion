package jobs

import (
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/health"
)

const (
	defaultBackendHealthCheckName = "jobs-backend"
	defaultRuntimeHealthCheckName = "jobs-runtime"
)

// NewBackendHealthChecker creates a standard health checker for a jobs backend.
func NewBackendHealthChecker(name string, backend Backend, timeout time.Duration) health.Checker {
	checkName := normalizeHealthCheckName(name, defaultBackendHealthCheckName)
	return health.NewAdapterChecker(checkName, backend, timeout)
}

// NewRuntimeHealthChecker creates a standard health checker for a jobs runtime.
func NewRuntimeHealthChecker(name string, runtime Runtime, timeout time.Duration) health.Checker {
	checkName := normalizeHealthCheckName(name, defaultRuntimeHealthCheckName)
	return health.NewAdapterChecker(checkName, runtime, timeout)
}

func normalizeHealthCheckName(name, fallback string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
