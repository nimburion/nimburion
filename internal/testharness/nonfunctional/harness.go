// Package nonfunctional provides helpers for non-functional test harnesses.
package nonfunctional

import (
	"os"
	"strings"
	"testing"
)

// Category identifies one standard non-functional verification family.
type Category string

const (
	// CategoryPerformance enables performance-focused non-functional tests.
	CategoryPerformance Category = "performance"
	// CategoryLoad enables load-focused non-functional tests.
	CategoryLoad Category = "load"
	// CategorySoak enables soak-focused non-functional tests.
	CategorySoak Category = "soak"
	// CategoryResilience enables resilience-focused non-functional tests.
	CategoryResilience Category = "resilience"
	// CategorySecurity enables security-focused non-functional tests.
	CategorySecurity Category = "security"
	// CategoryCompatibility enables compatibility-focused non-functional tests.
	CategoryCompatibility Category = "compatibility"
	// CategoryRace enables race-detection-focused non-functional tests.
	CategoryRace Category = "race"
	// CategoryOrdering enables ordering-focused non-functional tests.
	CategoryOrdering Category = "ordering"
)

// Enabled reports whether the category is enabled through NIMB_NONFUNCTIONAL.
func Enabled(category Category) bool {
	raw := strings.TrimSpace(os.Getenv("NIMB_NONFUNCTIONAL"))
	if raw == "" || strings.EqualFold(raw, "all") {
		return true
	}

	for _, item := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(item), string(category)) {
			return true
		}
	}
	return false
}

// RequireEnabled skips the test unless the category is enabled.
func RequireEnabled(t testing.TB, category Category) {
	t.Helper()
	if !Enabled(category) {
		t.Skipf("non-functional category %q disabled; set NIMB_NONFUNCTIONAL=%s or all", category, category)
	}
}

// Run executes one non-functional test body under a standard category.
func Run(t *testing.T, category Category, fn func(*testing.T)) {
	t.Helper()
	RequireEnabled(t, category)
	fn(t)
}
