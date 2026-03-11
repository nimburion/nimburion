package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/google/jsonschema-go/jsonschema"
	nimburioncfg "github.com/nimburion/nimburion/pkg/config"
)

// CoreSchemaDisabler can be implemented by extension structs to remove specific
// core root sections (for example "email" or "eventbus") from generated schema.
type CoreSchemaDisabler interface {
	DisabledCoreConfigSections() []string
}

// BuildSchema returns a JSON Schema for nimburion Config merged with the provided extensions.
func BuildSchema(extensions ...any) (*jsonschema.Schema, error) {
	return BuildSchemaWithDefaults(nil, extensions...)
}

// BuildSchemaWithDefaults builds the schema and injects defaults.
// If baseDefaults is nil, nimburion DefaultConfig() is used.
func BuildSchemaWithDefaults(baseDefaults any, extensions ...any) (*jsonschema.Schema, error) {
	opts := &jsonschema.ForOptions{
		IgnoreInvalidTypes: true,
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeOf(time.Duration(0)): {Type: "string"},
		},
	}

	baseSchema, err := jsonschema.ForType(reflect.TypeOf(nimburioncfg.Config{}), opts)
	if err != nil {
		return nil, fmt.Errorf("build base schema: %w", err)
	}
	applyFieldNames(baseSchema, reflect.TypeOf(nimburioncfg.Config{}))

	schema := baseSchema
	disabledSections := map[string]struct{}{}
	for _, ext := range extensions {
		if ext == nil {
			continue
		}
		collectDisabledCoreSections(ext, disabledSections)
		typeOf := reflect.TypeOf(ext)
		for typeOf.Kind() == reflect.Pointer {
			typeOf = typeOf.Elem()
		}
		if typeOf.Kind() != reflect.Struct {
			return nil, fmt.Errorf("extension must be a struct, got %s", typeOf.Kind())
		}
		extSchema, err := jsonschema.ForType(typeOf, opts)
		if err != nil {
			return nil, fmt.Errorf("build extension schema: %w", err)
		}
		applyFieldNames(extSchema, typeOf)
		schema = mergeSchemas(schema, extSchema)
	}
	disableCoreSections(schema, disabledSections)

	if baseDefaults == nil {
		baseDefaults = nimburioncfg.DefaultConfig()
	}
	injectDefaults(schema, reflect.ValueOf(baseDefaults))
	for _, ext := range extensions {
		if ext == nil {
			continue
		}
		injectDefaults(schema, reflect.ValueOf(ext))
	}
	pruneRequiredWithDefaults(schema)

	serviceName := "Application"
	if cfg, ok := baseDefaults.(*nimburioncfg.Config); ok && strings.TrimSpace(cfg.App.Name) != "" {
		serviceName = cfg.App.Name
	} else if cfg, ok := baseDefaults.(nimburioncfg.Config); ok && strings.TrimSpace(cfg.App.Name) != "" {
		serviceName = cfg.App.Name
	}

	schema.Title = serviceName + " Configuration"
	schema.Description = "Schema for " + serviceName + " configuration."
	schema.Schema = "https://json-schema.org/draft/2020-12/schema"
	return schema, nil
}

func applyFieldNames(schema *jsonschema.Schema, t reflect.Type) {
	if schema == nil || t == nil {
		return
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		if len(schema.Properties) == 0 {
			return
		}
		nameMap := make(map[string]string)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			sourceNames, omit := schemaSourceFieldNames(field)
			if omit {
				continue
			}
			desired := fieldKeyName(field)
			for _, sourceName := range sourceNames {
				nameMap[sourceName] = desired
				if prop, ok := schema.Properties[sourceName]; ok {
					delete(schema.Properties, sourceName)
					schema.Properties[desired] = prop
					applyFieldNames(prop, field.Type)
					break
				}
			}
		}

		if len(schema.Required) > 0 {
			updated := make([]string, 0, len(schema.Required))
			for _, name := range schema.Required {
				if mapped, ok := nameMap[name]; ok {
					updated = append(updated, mapped)
				} else {
					updated = append(updated, name)
				}
			}
			schema.Required = dedupeStrings(updated)
		}

		if len(schema.PropertyOrder) > 0 {
			updated := make([]string, 0, len(schema.PropertyOrder))
			for _, name := range schema.PropertyOrder {
				if mapped, ok := nameMap[name]; ok {
					updated = append(updated, mapped)
				} else {
					updated = append(updated, name)
				}
			}
			schema.PropertyOrder = dedupeStrings(updated)
		}

		if len(schema.DependentRequired) > 0 {
			updated := make(map[string][]string, len(schema.DependentRequired))
			for name, deps := range schema.DependentRequired {
				if mapped, ok := nameMap[name]; ok {
					name = mapped
				}
				updatedDeps := make([]string, 0, len(deps))
				for _, dep := range deps {
					if mapped, ok := nameMap[dep]; ok {
						updatedDeps = append(updatedDeps, mapped)
					} else {
						updatedDeps = append(updatedDeps, dep)
					}
				}
				updated[name] = dedupeStrings(updatedDeps)
			}
			schema.DependentRequired = updated
		}

	case reflect.Slice, reflect.Array:
		applyFieldNames(schema.Items, t.Elem())

	case reflect.Map:
		if schema.AdditionalProperties != nil {
			applyFieldNames(schema.AdditionalProperties, t.Elem())
		}
	}
}

func injectDefaults(schema *jsonschema.Schema, value reflect.Value) {
	if schema == nil || !value.IsValid() {
		return
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		if len(schema.Properties) == 0 {
			return
		}
		t := value.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			name := fieldKeyName(field)
			if name == "" {
				continue
			}
			propSchema, ok := schema.Properties[name]
			if !ok {
				continue
			}

			fieldVal := value.Field(i)
			if fieldVal.Kind() == reflect.Pointer && fieldVal.IsNil() {
				if propSchema.Default == nil && schemaAllowsNull(propSchema) {
					propSchema.Default = json.RawMessage("null")
				}
				continue
			}

			if propSchema.Default == nil {
				if raw, ok := marshalDefault(propSchema, fieldVal); ok {
					propSchema.Default = raw
				}
			}

			injectDefaults(propSchema, fieldVal)
		}

	case reflect.Slice, reflect.Array, reflect.Map, reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if schema.Default == nil {
			if raw, ok := marshalDefault(schema, value); ok {
				schema.Default = raw
			}
		}
	}
}

func pruneRequiredWithDefaults(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	for _, prop := range schema.Properties {
		pruneRequiredWithDefaults(prop)
	}
	if schema.Items != nil {
		pruneRequiredWithDefaults(schema.Items)
	}
	if schema.AdditionalProperties != nil {
		pruneRequiredWithDefaults(schema.AdditionalProperties)
	}
	if len(schema.Required) == 0 {
		return
	}
	required := make([]string, 0, len(schema.Required))
	for _, name := range schema.Required {
		prop := schema.Properties[name]
		if prop != nil && prop.Default != nil {
			continue
		}
		required = append(required, name)
	}
	schema.Required = required
}

func marshalDefault(schema *jsonschema.Schema, value reflect.Value) (json.RawMessage, bool) {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil, false
	}

	if value.Type() == reflect.TypeOf(time.Duration(0)) {
		duration, ok := value.Interface().(time.Duration)
		if !ok {
			return nil, false
		}
		return json.RawMessage(fmt.Sprintf("%q", duration.String())), true
	}

	if isZeroValue(value) && !schemaAllowsZeroDefault(schema, value.Kind()) {
		return nil, false
	}

	raw, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, false
	}
	return raw, true
}

func schemaAllowsNull(schema *jsonschema.Schema) bool {
	if schema == nil {
		return false
	}
	if schema.Type == "null" {
		return true
	}
	for _, v := range schema.AnyOf {
		if schemaAllowsNull(v) {
			return true
		}
	}
	for _, v := range schema.OneOf {
		if schemaAllowsNull(v) {
			return true
		}
	}
	return false
}

func schemaAllowsZeroDefault(schema *jsonschema.Schema, kind reflect.Kind) bool {
	if schema == nil {
		return false
	}
	switch kind {
	case reflect.Bool:
		return true
	case reflect.Struct:
		return true
	}
	return false
}

func collectDisabledCoreSections(ext any, disabled map[string]struct{}) {
	disabler, ok := ext.(CoreSchemaDisabler)
	if !ok {
		return
	}
	for _, name := range disabler.DisabledCoreConfigSections() {
		name = strings.TrimSpace(name)
		if name != "" {
			disabled[name] = struct{}{}
		}
	}
}

func disableCoreSections(schema *jsonschema.Schema, disabled map[string]struct{}) {
	if schema == nil || len(disabled) == 0 {
		return
	}
	for name := range disabled {
		delete(schema.Properties, name)
	}
	if len(schema.Required) == 0 {
		return
	}
	filtered := make([]string, 0, len(schema.Required))
	for _, name := range schema.Required {
		if _, ok := disabled[name]; !ok {
			filtered = append(filtered, name)
		}
	}
	schema.Required = filtered
}

func mergeSchemas(base, ext *jsonschema.Schema) *jsonschema.Schema {
	if base == nil {
		return ext
	}
	if ext == nil {
		return base
	}
	merged := *base
	if merged.Properties == nil {
		merged.Properties = map[string]*jsonschema.Schema{}
	}
	for name, prop := range ext.Properties {
		if existing, ok := merged.Properties[name]; ok {
			merged.Properties[name] = mergeSchemas(existing, prop)
			continue
		}
		merged.Properties[name] = prop
	}
	merged.Required = dedupeStrings(append(append([]string(nil), base.Required...), ext.Required...))
	merged.PropertyOrder = dedupeStrings(append(append([]string(nil), base.PropertyOrder...), ext.PropertyOrder...))
	if len(ext.DependentRequired) > 0 {
		if merged.DependentRequired == nil {
			merged.DependentRequired = map[string][]string{}
		}
		for key, deps := range ext.DependentRequired {
			merged.DependentRequired[key] = dedupeStrings(append(append([]string(nil), merged.DependentRequired[key]...), deps...))
		}
	}
	return &merged
}

func fieldKeyName(field reflect.StructField) string {
	if tag := strings.TrimSpace(field.Tag.Get("mapstructure")); tag != "" && tag != "-" {
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			return name
		}
	}
	if tag := strings.TrimSpace(field.Tag.Get("yaml")); tag != "" && tag != "-" {
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			return name
		}
	}
	if tag := strings.TrimSpace(field.Tag.Get("json")); tag != "" && tag != "-" {
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			return name
		}
	}
	return lowerCamel(field.Name)
}

func schemaSourceFieldNames(field reflect.StructField) ([]string, bool) {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "-" {
		return nil, true
	}
	if tag != "" {
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			return uniqueStrings([]string{name, lowerCamel(field.Name), snakeCase(field.Name), field.Name}), false
		}
	}
	return uniqueStrings([]string{lowerCamel(field.Name), snakeCase(field.Name), field.Name}), false
}

func lowerCamel(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func snakeCase(name string) string {
	if name == "" {
		return ""
	}
	var out []rune
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				out = append(out, '_')
			}
			out = append(out, unicode.ToLower(r))
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func uniqueStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		filtered = append(filtered, value)
	}
	return dedupeStrings(filtered)
}

func isZeroValue(value reflect.Value) bool {
	return value.IsZero()
}
