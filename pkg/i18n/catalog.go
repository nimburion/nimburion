package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Catalog stores locale-key translations in memory.
type Catalog struct {
	defaultLocale string
	fallbackMode  string
	messages      map[string]map[string]string
}

// NewCatalog creates an empty translation catalog.
func NewCatalog(defaultLocale, fallbackMode string) *Catalog {
	mode := strings.ToLower(strings.TrimSpace(fallbackMode))
	if mode == "" {
		mode = "base"
	}
	return &Catalog{
		defaultLocale: normalizeLocale(defaultLocale),
		fallbackMode:  mode,
		messages:      map[string]map[string]string{},
	}
}

// LoadCatalogDir loads translation files from a directory.
// Supported files: *.json, *.yaml, *.yml named as locale files (e.g. en.json, it-IT.yaml).
func LoadCatalogDir(path, defaultLocale, fallbackMode string) (*Catalog, error) {
	catalog := NewCatalog(defaultLocale, fallbackMode)
	if strings.TrimSpace(path) == "" {
		return catalog, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read i18n catalog dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(path, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		locale := strings.TrimSpace(strings.TrimSuffix(entry.Name(), ext))
		if locale == "" {
			continue
		}
		switch ext {
		case ".json":
			if err := loadJSONCatalog(catalog, locale, filePath); err != nil {
				return nil, err
			}
		case ".yaml", ".yml":
			if err := loadYAMLCatalog(catalog, locale, filePath); err != nil {
				return nil, err
			}
		}
	}
	return catalog, nil
}

// ForLocale returns a locale-bound translator.
func (c *Catalog) ForLocale(locale string) Translator {
	return localizedTranslator{
		catalog: c,
		locale:  normalizeLocale(locale),
	}
}

// Add inserts translations for a locale.
func (c *Catalog) Add(locale string, entries map[string]string) {
	locale = normalizeLocale(locale)
	if locale == "" {
		return
	}
	if c.messages[locale] == nil {
		c.messages[locale] = map[string]string{}
	}
	for key, value := range entries {
		if strings.TrimSpace(key) == "" {
			continue
		}
		c.messages[locale][key] = value
	}
}

type localizedTranslator struct {
	catalog *Catalog
	locale  string
}

func (t localizedTranslator) T(key string, args ...interface{}) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if t.catalog == nil {
		return key
	}

	template := t.catalog.lookup(t.locale, key)
	if template == "" {
		return key
	}

	params := parseTemplateArgs(args...)
	return applyTemplateParams(template, params)
}

func (c *Catalog) lookup(locale, key string) string {
	for _, candidate := range c.fallbackLocales(locale) {
		if entries := c.messages[candidate]; entries != nil {
			if value, ok := entries[key]; ok {
				return value
			}
		}
	}
	return ""
}

func (c *Catalog) fallbackLocales(locale string) []string {
	ordered := []string{}
	seen := map[string]struct{}{}

	add := func(item string) {
		item = normalizeLocale(item)
		if item == "" {
			return
		}
		if _, exists := seen[item]; exists {
			return
		}
		seen[item] = struct{}{}
		ordered = append(ordered, item)
	}

	add(locale)
	if strings.EqualFold(c.fallbackMode, "base") {
		add(baseLocale(locale))
	}
	add(c.defaultLocale)
	if strings.EqualFold(c.fallbackMode, "base") {
		add(baseLocale(c.defaultLocale))
	}
	return ordered
}

func parseTemplateArgs(args ...interface{}) map[string]interface{} {
	if len(args) == 0 {
		return nil
	}
	if len(args) == 1 {
		switch v := args[0].(type) {
		case map[string]interface{}:
			return v
		case Params:
			out := make(map[string]interface{}, len(v))
			for key, value := range v {
				out[key] = value
			}
			return out
		}
	}

	params := map[string]interface{}{}
	for idx := 0; idx+1 < len(args); idx += 2 {
		key, ok := args[idx].(string)
		if !ok {
			continue
		}
		params[key] = args[idx+1]
	}
	return params
}

func applyTemplateParams(template string, params map[string]interface{}) string {
	if len(params) == 0 {
		return template
	}
	// Deterministic replacement order for stable tests.
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := template
	for _, key := range keys {
		value := fmt.Sprint(params[key])
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
		out = strings.ReplaceAll(out, "{"+key+"}", value)
	}
	return out
}

func loadJSONCatalog(catalog *Catalog, locale, filePath string) error {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read i18n json catalog %s: %w", filePath, err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode i18n json catalog %s: %w", filePath, err)
	}
	catalog.Add(locale, flattenCatalog(payload, ""))
	return nil
}

func loadYAMLCatalog(catalog *Catalog, locale, filePath string) error {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read i18n yaml catalog %s: %w", filePath, err)
	}
	var payload map[string]interface{}
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode i18n yaml catalog %s: %w", filePath, err)
	}
	catalog.Add(locale, flattenCatalog(payload, ""))
	return nil
}

func flattenCatalog(payload map[string]interface{}, prefix string) map[string]string {
	out := map[string]string{}
	for key, value := range payload {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch node := value.(type) {
		case map[string]interface{}:
			child := flattenCatalog(node, fullKey)
			for k, v := range child {
				out[k] = v
			}
		case string:
			out[fullKey] = node
		}
	}
	return out
}

var localeCleaner = regexp.MustCompile(`[^a-zA-Z0-9\-]`)

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(strings.ReplaceAll(locale, "_", "-"))
	locale = localeCleaner.ReplaceAllString(locale, "")
	return strings.ToLower(locale)
}

func baseLocale(locale string) string {
	locale = normalizeLocale(locale)
	if idx := strings.Index(locale, "-"); idx > 0 {
		return locale[:idx]
	}
	return locale
}
