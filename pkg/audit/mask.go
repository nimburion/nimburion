package audit

import (
	"reflect"
	"strings"
)

type Classification string
type Redaction string

const (
	ClassificationPublic    Classification = "public"
	ClassificationSensitive Classification = "sensitive"
	ClassificationSecret    Classification = "secret"

	RedactionNone Redaction = "none"
	RedactionMask Redaction = "mask"
	RedactionFull Redaction = "full"
)

const MaskedValue = "***"

func RedactSettings(settings, mask map[string]interface{}) map[string]interface{} {
	if len(settings) == 0 || len(mask) == 0 {
		return settings
	}
	out := make(map[string]interface{}, len(settings))
	for key, value := range settings {
		childMask, ok := mask[key]
		if !ok {
			out[key] = value
			continue
		}
		out[key] = redactValue(value, childMask)
	}
	return out
}

func redactValue(value, mask interface{}) interface{} {
	maskMap, maskIsMap := mask.(map[string]interface{})
	if maskIsMap {
		valueMap, valueIsMap := value.(map[string]interface{})
		if !valueIsMap {
			if ShouldRedact(mask) {
				return MaskedValue
			}
			return value
		}
		out := make(map[string]interface{}, len(valueMap))
		for key, item := range valueMap {
			childMask, ok := maskMap[key]
			if !ok {
				out[key] = item
				continue
			}
			out[key] = redactValue(item, childMask)
		}
		return out
	}
	if ShouldRedact(mask) {
		return MaskedValue
	}
	return value
}

func ShouldRedact(mask interface{}) bool {
	if mask == nil {
		return false
	}
	switch value := mask.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case bool:
		return value
	case int:
		return value != 0
	case int8:
		return value != 0
	case int16:
		return value != 0
	case int32:
		return value != 0
	case int64:
		return value != 0
	case uint:
		return value != 0
	case uint8:
		return value != 0
	case uint16:
		return value != 0
	case uint32:
		return value != 0
	case uint64:
		return value != 0
	case float32:
		return value != 0
	case float64:
		return value != 0
	case []interface{}:
		return len(value) > 0
	case map[string]interface{}:
		return len(value) > 0
	default:
		return !reflect.ValueOf(mask).IsZero()
	}
}
