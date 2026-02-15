package i18n

import (
	"testing"
)

func TestCatalog_TranslateWithFallbackAndParams(t *testing.T) {
	catalog := NewCatalog("en", "base")
	catalog.Add("en", map[string]string{
		"errors.not_found": "resource {id} not found",
	})
	catalog.Add("it", map[string]string{
		"errors.not_found": "risorsa {id} non trovata",
	})

	translator := catalog.ForLocale("it-IT")
	got := translator.T("errors.not_found", Params{"id": "42"})
	if got != "risorsa 42 non trovata" {
		t.Fatalf("unexpected translation: %q", got)
	}
}

func TestCatalog_MissingKeyFallsBackToKey(t *testing.T) {
	catalog := NewCatalog("en", "base")
	translator := catalog.ForLocale("en")
	got := translator.T("validation.unknown")
	if got != "validation.unknown" {
		t.Fatalf("expected fallback to key, got %q", got)
	}
}
