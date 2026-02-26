package testutil

import (
	"os"
	"testing"
)

// SkipIfShort skips the test if running in short mode
func SkipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// SkipIfCI skips the test if running in CI environment
func SkipIfCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("skipping test in CI environment")
	}
}

// RequireIntegration skips the test unless INTEGRATION_TESTS=1 is set
func RequireIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("INTEGRATION_TESTS") == "" && os.Getenv("CI") != "" {
		t.Skip("skipping integration test (set INTEGRATION_TESTS=1 to run)")
	}
}
