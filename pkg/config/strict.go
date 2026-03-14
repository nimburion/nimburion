package config

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

type configShape struct {
	children map[string]*configShape
	elem     *configShape
}

func validateRuntimeSettings(settings map[string]interface{}, extensions ...interface{}) error {
	shape, err := buildRuntimeConfigShape(extensions...)
	if err != nil {
		return err
	}
	if err := detectNonConfigDocument(settings, shape); err != nil {
		return err
	}
	return validateSettingsAgainstShape(settings, shape, "")
}

func validateRuntimeConfigFile(configFile string, extensions ...interface{}) error {
	if strings.TrimSpace(configFile) == "" {
		return nil
	}

	raw := viper.New()
	raw.SetConfigFile(configFile)
	if err := raw.ReadInConfig(); err != nil {
		return err
	}

	return validateRuntimeSettings(raw.AllSettings(), extensions...)
}

func buildRuntimeConfigShape(extensions ...interface{}) (*configShape, error) {
	root, err := buildShapeFromType(reflect.TypeOf(Config{}))
	if err != nil {
		return nil, err
	}
	for _, extension := range extensions {
		if extension == nil {
			continue
		}
		extType := reflect.TypeOf(extension)
		if extType.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("config extension must be a pointer to a struct")
		}
		shape, err := buildShapeFromTarget(extension)
		if err != nil {
			return nil, err
		}
		mergeShapes(root, shape)
	}
	return root, nil
}

func buildShapeFromTarget(target interface{}) (*configShape, error) {
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return nil, fmt.Errorf("config target must be a non-nil pointer")
	}
	return buildShapeFromType(value.Elem().Type())
}

func buildShapeFromType(typeOf reflect.Type) (*configShape, error) {
	for typeOf.Kind() == reflect.Ptr {
		typeOf = typeOf.Elem()
	}

	node := &configShape{
		children: map[string]*configShape{},
	}

	switch typeOf.Kind() {
	case reflect.Struct:
		if isTimeDuration(typeOf) {
			return node, nil
		}
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			if field.PkgPath != "" {
				continue
			}
			mapKey, skip := parseMapstructureTag(field.Tag.Get("mapstructure"))
			if skip {
				continue
			}
			if mapKey == "" {
				mapKey = toSnakeCase(field.Name)
			}
			child, err := buildShapeFromType(field.Type)
			if err != nil {
				return nil, err
			}
			node.children[mapKey] = child
		}
	case reflect.Map:
		elem, err := buildShapeFromType(typeOf.Elem())
		if err != nil {
			return nil, err
		}
		node.elem = elem
	case reflect.Slice, reflect.Array:
		elem, err := buildShapeFromType(typeOf.Elem())
		if err != nil {
			return nil, err
		}
		node.elem = elem
	}

	return node, nil
}

func validateSettingsAgainstShape(value interface{}, shape *configShape, path string) error {
	switch typed := normalizeConfigValue(value).(type) {
	case map[string]interface{}:
		if shape == nil {
			return nil
		}
		if shape.elem != nil && len(shape.children) == 0 {
			var errs []error
			for key, nested := range typed {
				nextPath := joinConfigPath(path, key)
				if err := validateSettingsAgainstShape(nested, shape.elem, nextPath); err != nil {
					errs = append(errs, err)
				}
			}
			if len(errs) > 0 {
				return errors.Join(errs...)
			}
			return nil
		}

		var errs []error
		for key, nested := range typed {
			child := shape.children[key]
			if child == nil {
				errs = append(errs, fmt.Errorf("unknown config key %q", joinConfigPath(path, key)))
				continue
			}
			if err := validateSettingsAgainstShape(nested, child, joinConfigPath(path, key)); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
	case []interface{}:
		if shape == nil || shape.elem == nil {
			return nil
		}
		var errs []error
		for index, nested := range typed {
			nextPath := fmt.Sprintf("%s[%d]", path, index)
			if err := validateSettingsAgainstShape(nested, shape.elem, nextPath); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
	}

	return nil
}

func normalizeConfigValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			normalized[key] = normalizeConfigValue(nested)
		}
		return normalized
	case map[interface{}]interface{}:
		normalized := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			normalized[fmt.Sprint(key)] = normalizeConfigValue(nested)
		}
		return normalized
	case []interface{}:
		normalized := make([]interface{}, len(typed))
		for index, nested := range typed {
			normalized[index] = normalizeConfigValue(nested)
		}
		return normalized
	default:
		return value
	}
}

func detectNonConfigDocument(settings map[string]interface{}, shape *configShape) error {
	if len(settings) == 0 || shape == nil {
		return nil
	}

	knownRoots := make(map[string]struct{}, len(shape.children))
	for key := range shape.children {
		knownRoots[key] = struct{}{}
	}

	schemaMarkers := []string{
		"$schema",
		"$defs",
		"additionalproperties",
		"allOf",
		"anyOf",
		"definitions",
		"oneOf",
		"properties",
		"required",
		"title",
		"type",
	}

	markerCount := 0
	knownCount := 0
	for key := range settings {
		normalizedKey := strings.ToLower(key)
		if _, ok := knownRoots[normalizedKey]; ok {
			knownCount++
		}
		if containsString(schemaMarkers, normalizedKey) {
			markerCount++
		}
	}

	if knownCount == 0 && markerCount >= 2 {
		return fmt.Errorf("input does not look like a runtime config document: found schema-like top-level keys %s", formatSortedKeys(settings))
	}

	return nil
}

func mergeShapes(dst, src *configShape) {
	if dst == nil || src == nil {
		return
	}
	if dst.children == nil {
		dst.children = map[string]*configShape{}
	}
	for key, child := range src.children {
		existing := dst.children[key]
		if existing == nil {
			dst.children[key] = child
			continue
		}
		mergeShapes(existing, child)
	}
	if dst.elem == nil {
		dst.elem = src.elem
		return
	}
	mergeShapes(dst.elem, src.elem)
}

func joinConfigPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func formatSortedKeys(settings map[string]interface{}) string {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func isTimeDuration(typeOf reflect.Type) bool {
	return typeOf.PkgPath() == "time" && typeOf.Name() == "Duration"
}
