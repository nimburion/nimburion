package i18n

import "context"

type contextKey string

const (
	localeContextKey     contextKey = "nimburion.i18n.locale"
	translatorContextKey contextKey = "nimburion.i18n.translator"
)

// WithLocale stores the resolved locale in context.
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey, locale)
}

// GetLocale returns the locale from context, if set.
func GetLocale(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	locale, _ := ctx.Value(localeContextKey).(string)
	return locale
}

// WithTranslator stores a translator in context.
func WithTranslator(ctx context.Context, translator Translator) context.Context {
	return context.WithValue(ctx, translatorContextKey, translator)
}

// TranslatorFromContext returns the translator in context.
// If not available, a fallback translator is returned.
func TranslatorFromContext(ctx context.Context) Translator {
	if ctx == nil {
		return fallbackTranslator{}
	}
	translator, _ := ctx.Value(translatorContextKey).(Translator)
	if translator == nil {
		return fallbackTranslator{}
	}
	return translator
}

type fallbackTranslator struct{}

func (fallbackTranslator) T(key string, args ...interface{}) string {
	_ = args
	return key
}
