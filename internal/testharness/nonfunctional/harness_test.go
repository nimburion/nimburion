package nonfunctional

import "testing"

func TestEnabled_DefaultsToTrue(t *testing.T) {
	t.Setenv("NIMB_NONFUNCTIONAL", "")
	if !Enabled(CategoryResilience) {
		t.Fatal("expected category to be enabled by default")
	}
}

func TestEnabled_MatchesExplicitCategory(t *testing.T) {
	t.Setenv("NIMB_NONFUNCTIONAL", "resilience,ordering")
	if !Enabled(CategoryOrdering) {
		t.Fatal("expected ordering category to be enabled")
	}
	if Enabled(CategorySecurity) {
		t.Fatal("expected security category to stay disabled")
	}
}
