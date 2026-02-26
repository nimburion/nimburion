package i18n

import (
	"context"
	"testing"
)

func TestWithLocale(t *testing.T) {
	ctx := WithLocale(context.Background(), "fr-FR")
	locale := GetLocale(ctx)
	if locale != "fr-FR" {
		t.Errorf("expected fr-FR, got %s", locale)
	}
}

func TestGetLocaleDefault(t *testing.T) {
	locale := GetLocale(context.Background())
	if locale != "" {
		t.Errorf("expected empty string, got %s", locale)
	}
}

func TestGetLocaleNilContext(t *testing.T) {
	locale := GetLocale(nil)
	if locale != "" {
		t.Errorf("expected empty string, got %s", locale)
	}
}

func TestWithTranslator(t *testing.T) {
	catalog := NewCatalog("en", "base")
	translator := catalog.ForLocale("en")
	ctx := WithTranslator(context.Background(), translator)
	tr := TranslatorFromContext(ctx)
	if tr == nil {
		t.Fatal("expected translator, got nil")
	}
}

func TestTranslatorFromContextNil(t *testing.T) {
	tr := TranslatorFromContext(nil)
	if tr == nil {
		t.Fatal("expected fallback translator, got nil")
	}
	result := tr.T("test.key")
	if result != "test.key" {
		t.Errorf("expected test.key, got %s", result)
	}
}

func TestTranslatorFromContextNoTranslator(t *testing.T) {
	tr := TranslatorFromContext(context.Background())
	if tr == nil {
		t.Fatal("expected fallback translator, got nil")
	}
	result := tr.T("test.key")
	if result != "test.key" {
		t.Errorf("expected test.key, got %s", result)
	}
}
