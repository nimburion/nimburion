package scheduler

import (
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/health"
)

const defaultLockProviderHealthCheckName = "scheduler-lock-provider"

// NewLockProviderHealthChecker creates a standard health checker for scheduler lock providers.
func NewLockProviderHealthChecker(name string, provider LockProvider, timeout time.Duration) health.Checker {
	checkName := strings.TrimSpace(name)
	if checkName == "" {
		checkName = defaultLockProviderHealthCheckName
	}
	return health.NewAdapterChecker(checkName, provider, timeout)
}
