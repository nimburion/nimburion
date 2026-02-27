package i18n

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	frameworki18n "github.com/nimburion/nimburion/pkg/i18n"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// Context key constants for i18n
const (
	// LocaleContextKey is the context key for storing locale
	LocaleContextKey = "locale"
	// TranslatorContextKey is the context key for storing translator
	TranslatorContextKey = "translator"
)

// Config controls locale resolution and translation catalogs.
type Config struct {
	Enabled              bool
	DefaultLocale        string
	SupportedLocales     []string
	QueryParam           string
	HeaderName           string
	FallbackMode         string // base, default
	CatalogPath          string
	ExcludedPathPrefixes []string
}

// DefaultConfig returns explicit i18n defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:          false,
		DefaultLocale:    "en",
		SupportedLocales: []string{"en"},
		QueryParam:       "lang",
		HeaderName:       "X-Locale",
		FallbackMode:     "base",
		CatalogPath:      "",
		ExcludedPathPrefixes: []string{
			"/metrics",
			"/health",
			"/version",
			"/ready",
		},
	}
}

// Middleware resolves locale and stores locale + translator in both router.Context and request.Context.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalizeConfig(cfg)
	catalog, err := frameworki18n.LoadCatalogDir(cfg.CatalogPath, cfg.DefaultLocale, cfg.FallbackMode)
	if err != nil {
		// Fail open: keep middleware functional with key-fallback translator.
		catalog = frameworki18n.NewCatalog(cfg.DefaultLocale, cfg.FallbackMode)
	}

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled || c.Request() == nil {
				return next(c)
			}
			if isExcluded(c.Request().URL.Path, cfg.ExcludedPathPrefixes) {
				return next(c)
			}

			locale := resolveLocale(c.Request(), cfg)
			translator := catalog.ForLocale(locale)

			ctx := c.Request().Context()
			ctx = frameworki18n.WithLocale(ctx, locale)
			ctx = frameworki18n.WithTranslator(ctx, translator)
			c.SetRequest(c.Request().WithContext(ctx))

			c.Set(LocaleContextKey, locale)
			c.Set(TranslatorContextKey, translator)

			c.Response().Header().Set("Content-Language", locale)
			appendVary(c.Response().Header(), "Accept-Language")
			if header := strings.TrimSpace(cfg.HeaderName); header != "" {
				appendVary(c.Response().Header(), header)
			}

			return next(c)
		}
	}
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if !cfg.Enabled && cfg.DefaultLocale == "" && len(cfg.SupportedLocales) == 0 && cfg.QueryParam == "" && cfg.HeaderName == "" && cfg.FallbackMode == "" && cfg.CatalogPath == "" && len(cfg.ExcludedPathPrefixes) == 0 {
		return def
	}
	if strings.TrimSpace(cfg.DefaultLocale) == "" {
		cfg.DefaultLocale = def.DefaultLocale
	}
	if len(cfg.SupportedLocales) == 0 {
		cfg.SupportedLocales = def.SupportedLocales
	}
	if strings.TrimSpace(cfg.QueryParam) == "" {
		cfg.QueryParam = def.QueryParam
	}
	if strings.TrimSpace(cfg.HeaderName) == "" {
		cfg.HeaderName = def.HeaderName
	}
	if strings.TrimSpace(cfg.FallbackMode) == "" {
		cfg.FallbackMode = def.FallbackMode
	}
	return cfg
}

func isExcluded(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func resolveLocale(r *http.Request, cfg Config) string {
	candidates := make([]string, 0, 4)

	if qp := strings.TrimSpace(cfg.QueryParam); qp != "" {
		if value := strings.TrimSpace(r.URL.Query().Get(qp)); value != "" {
			candidates = append(candidates, value)
		}
	}
	if header := strings.TrimSpace(cfg.HeaderName); header != "" {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			candidates = append(candidates, value)
		}
	}
	candidates = append(candidates, parseAcceptLanguage(r.Header.Get("Accept-Language"))...)

	return pickSupportedLocale(candidates, cfg)
}

func pickSupportedLocale(candidates []string, cfg Config) string {
	supported := make([]string, 0, len(cfg.SupportedLocales))
	supportedSet := make(map[string]string, len(cfg.SupportedLocales))
	for _, locale := range cfg.SupportedLocales {
		normalized := normalizeLocale(locale)
		if normalized == "" {
			continue
		}
		if _, exists := supportedSet[normalized]; !exists {
			supported = append(supported, normalized)
			supportedSet[normalized] = normalized
		}
	}

	defaultLocale := normalizeLocale(cfg.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	for _, candidate := range candidates {
		norm := normalizeLocale(candidate)
		if norm == "" {
			continue
		}
		if supportedLocale, ok := supportedSet[norm]; ok {
			return supportedLocale
		}
		if strings.EqualFold(cfg.FallbackMode, "base") {
			if base := baseLocale(norm); base != "" {
				if supportedLocale, ok := supportedSet[base]; ok {
					return supportedLocale
				}
			}
		}
	}

	if _, ok := supportedSet[defaultLocale]; ok {
		return defaultLocale
	}
	if len(supported) > 0 {
		return supported[0]
	}
	return defaultLocale
}

type langQ struct {
	lang string
	q    float64
}

func parseAcceptLanguage(raw string) []string {
	parts := strings.Split(raw, ",")
	items := make([]langQ, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sections := strings.Split(part, ";")
		lang := strings.TrimSpace(sections[0])
		if lang == "" || lang == "*" {
			continue
		}
		q := 1.0
		for _, section := range sections[1:] {
			kv := strings.SplitN(strings.TrimSpace(section), "=", 2)
			if len(kv) != 2 || strings.ToLower(kv[0]) != "q" {
				continue
			}
			if parsed, err := strconv.ParseFloat(kv[1], 64); err == nil {
				q = parsed
			}
		}
		if q <= 0 {
			continue
		}
		items = append(items, langQ{lang: lang, q: q})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].q > items[j].q
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.lang)
	}
	return out
}

func appendVary(header http.Header, value string) {
	current := header.Get("Vary")
	if current == "" {
		header.Set("Vary", value)
		return
	}
	for _, part := range strings.Split(current, ",") {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}
	header.Set("Vary", current+", "+value)
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(strings.ReplaceAll(locale, "_", "-"))
	return strings.ToLower(locale)
}

func baseLocale(locale string) string {
	if idx := strings.Index(locale, "-"); idx > 0 {
		return locale[:idx]
	}
	return locale
}

// GetLocale returns the resolved locale from request context.
func GetLocale(ctx context.Context) string {
	return frameworki18n.GetLocale(ctx)
}

// TranslatorFromContext returns the locale-bound translator from request context.
func TranslatorFromContext(ctx context.Context) frameworki18n.Translator {
	return frameworki18n.TranslatorFromContext(ctx)
}
