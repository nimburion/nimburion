package configschema

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

	serviceName := "Service"
	if cfg, ok := baseDefaults.(*nimburioncfg.Config); ok && strings.TrimSpace(cfg.Service.Name) != "" {
		serviceName = cfg.Service.Name
	} else if cfg, ok := baseDefaults.(nimburioncfg.Config); ok && strings.TrimSpace(cfg.Service.Name) != "" {
		serviceName = cfg.Service.Name
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
			jsonName, omit := jsonFieldName(field)
			if omit {
				continue
			}
			desired := fieldKeyName(field)
			nameMap[jsonName] = desired
			if prop, ok := schema.Properties[jsonName]; ok {
				delete(schema.Properties, jsonName)
				schema.Properties[desired] = prop
				applyFieldNames(prop, field.Type)
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
	if len(schema.Required) == 0 || len(schema.Properties) == 0 {
		return
	}
	kept := make([]string, 0, len(schema.Required))
	for _, name := range schema.Required {
		prop := schema.Properties[name]
		if prop == nil || prop.Default == nil {
			kept = append(kept, name)
		}
	}
	schema.Required = kept
}

func marshalDefault(schema *jsonschema.Schema, value reflect.Value) (json.RawMessage, bool) {
	if !value.IsValid() {
		return nil, false
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}

	if value.Type() == reflect.TypeOf(time.Duration(0)) && schemaAllowsString(schema) {
		dur := value.Interface().(time.Duration)
		payload, err := json.Marshal(dur.String())
		if err != nil {
			return nil, false
		}
		return payload, true
	}

	payload, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, false
	}
	return payload, true
}

func schemaAllowsString(schema *jsonschema.Schema) bool {
	if schema == nil {
		return false
	}
	if schema.Type == "string" {
		return true
	}
	for _, t := range schema.Types {
		if t == "string" {
			return true
		}
	}
	return false
}

func fieldKeyName(field reflect.StructField) string {
	if tag, ok := tagName(field.Tag.Get("mapstructure")); ok {
		return tag
	}
	if tag, ok := tagName(field.Tag.Get("yaml")); ok {
		return tag
	}
	return toSnakeCase(field.Name)
}

func tagName(tag string) (string, bool) {
	if tag == "" {
		return "", false
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return "", false
	}
	return name, true
}

func toSnakeCase(value string) string {
	if value == "" {
		return value
	}
	var b strings.Builder
	b.Grow(len(value) + 8)

	for i, r := range value {
		if i > 0 && isWordBoundary(value, i, r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func isWordBoundary(value string, index int, r rune) bool {
	if !unicode.IsUpper(r) {
		return false
	}
	prev := rune(value[index-1])
	if unicode.IsUpper(prev) {
		if index+1 < len(value) {
			next := rune(value[index+1])
			return unicode.IsLower(next)
		}
		return false
	}
	return true
}

func jsonFieldName(field reflect.StructField) (string, bool) {
	if !field.IsExported() {
		return "", true
	}
	name := field.Name
	if tag, ok := field.Tag.Lookup("json"); ok {
		tagName, _, found := strings.Cut(tag, ",")
		if tagName == "-" && !found {
			return "", true
		}
		if tagName != "" {
			name = tagName
		}
	}
	return name, false
}

func mergeSchemas(primary, secondary *jsonschema.Schema) *jsonschema.Schema {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}
	return mergeSchemaNode(primary, secondary)
}

func mergeSchemaNode(primary, secondary *jsonschema.Schema) *jsonschema.Schema {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}

	merged := &jsonschema.Schema{}
	*merged = *primary

	merged.Properties = map[string]*jsonschema.Schema{}
	for key, value := range primary.Properties {
		merged.Properties[key] = value
	}
	for key, value := range secondary.Properties {
		if existing, exists := merged.Properties[key]; exists {
			merged.Properties[key] = mergeSchemaNode(existing, value)
			continue
		}
		merged.Properties[key] = value
	}

	if secondary.Type != "" {
		merged.Type = secondary.Type
	}
	if len(secondary.Types) > 0 {
		merged.Types = dedupeStrings(append([]string{}, secondary.Types...))
	}

	merged.Required = dedupeStrings(append(append([]string{}, primary.Required...), secondary.Required...))
	merged.PropertyOrder = dedupeStrings(append(append([]string{}, primary.PropertyOrder...), secondary.PropertyOrder...))
	merged.DependentRequired = mergeDependentRequired(primary.DependentRequired, secondary.DependentRequired)

	if secondary.AdditionalProperties != nil {
		if merged.AdditionalProperties != nil {
			merged.AdditionalProperties = mergeSchemaNode(merged.AdditionalProperties, secondary.AdditionalProperties)
		} else {
			merged.AdditionalProperties = secondary.AdditionalProperties
		}
	}

	if secondary.Items != nil {
		if merged.Items != nil {
			merged.Items = mergeSchemaNode(merged.Items, secondary.Items)
		} else {
			merged.Items = secondary.Items
		}
	}

	if secondary.Default != nil {
		merged.Default = secondary.Default
	}

	return merged
}

func mergeDependentRequired(primary, secondary map[string][]string) map[string][]string {
	if len(primary) == 0 && len(secondary) == 0 {
		return nil
	}
	out := make(map[string][]string, len(primary)+len(secondary))
	for key, values := range primary {
		out[key] = append([]string{}, values...)
	}
	for key, values := range secondary {
		out[key] = dedupeStrings(append(out[key], values...))
	}
	return out
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

func schemaAllowsNull(schema *jsonschema.Schema) bool {
	if schema == nil {
		return false
	}
	if schema.Type == "null" {
		return true
	}
	for _, t := range schema.Types {
		if t == "null" {
			return true
		}
	}
	return false
}

func collectDisabledCoreSections(extension any, disabled map[string]struct{}) {
	disabler, ok := extension.(CoreSchemaDisabler)
	if !ok {
		return
	}
	for _, section := range disabler.DisabledCoreConfigSections() {
		normalized := strings.ToLower(strings.TrimSpace(section))
		if normalized == "" {
			continue
		}
		disabled[normalized] = struct{}{}
	}
}

func disableCoreSections(schema *jsonschema.Schema, disabled map[string]struct{}) {
	if schema == nil || len(disabled) == 0 {
		return
	}
	for section := range disabled {
		delete(schema.Properties, section)
		delete(schema.DependentRequired, section)
	}
	schema.Required = filterDisabledSections(schema.Required, disabled)
	schema.PropertyOrder = filterDisabledSections(schema.PropertyOrder, disabled)

	if len(schema.DependentRequired) == 0 {
		return
	}
	updated := make(map[string][]string, len(schema.DependentRequired))
	for key, deps := range schema.DependentRequired {
		updated[key] = filterDisabledSections(deps, disabled)
	}
	schema.DependentRequired = updated
}

func filterDisabledSections(values []string, disabled map[string]struct{}) []string {
	if len(values) == 0 {
		return values
	}
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if _, blocked := disabled[strings.ToLower(strings.TrimSpace(value))]; blocked {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}
