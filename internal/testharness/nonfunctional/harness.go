package nonfunctional

import (
	"os"
	"strings"
	"testing"
)

// Category identifies one standard non-functional verification family.
type Category string

const (
	CategoryPerformance   Category = "performance"
	CategoryLoad          Category = "load"
	CategorySoak          Category = "soak"
	CategoryResilience    Category = "resilience"
	CategorySecurity      Category = "security"
	CategoryCompatibility Category = "compatibility"
	CategoryRace          Category = "race"
	CategoryOrdering      Category = "ordering"
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
