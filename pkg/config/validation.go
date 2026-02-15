package config

import (
	"fmt"
	"reflect"
	"strings"
)

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Auth.Enabled {
		if c.Auth.Issuer == "" {
			return fmt.Errorf("auth.issuer is required when auth is enabled")
		}
		if c.Auth.JWKSUrl == "" {
			return fmt.Errorf("auth.jwks_url is required when auth is enabled")
		}
		if c.Auth.Audience == "" {
			return fmt.Errorf("auth.audience is required when auth is enabled")
		}
	}

	if c.Management.AuthEnabled && !c.Auth.Enabled {
		return fmt.Errorf("management.auth_enabled requires auth.enabled to be true")
	}

	if c.Management.MTLSEnabled {
		if c.Management.TLSCertFile == "" {
			return fmt.Errorf("management.tls_cert_file is required when mtls is enabled")
		}
		if c.Management.TLSKeyFile == "" {
			return fmt.Errorf("management.tls_key_file is required when mtls is enabled")
		}
		if c.Management.TLSCAFile == "" {
			return fmt.Errorf("management.tls_ca_file is required when mtls is enabled")
		}
	}

	if c.Database.Type != "" {
		if c.Database.URL == "" && c.Database.Type != DatabaseTypeDynamoDB {
			return fmt.Errorf("database.url is required when database.type is set")
		}
		if c.Database.Type == DatabaseTypeMongoDB && c.Database.DatabaseName == "" {
			return fmt.Errorf("database.database_name is required for MongoDB")
		}
		if c.Database.Type == DatabaseTypeDynamoDB && c.Database.Region == "" {
			return fmt.Errorf("database.region is required for DynamoDB")
		}
	}

	if c.Cache.Type == "redis" && c.Cache.URL == "" {
		return fmt.Errorf("cache.url is required when cache.type is redis")
	}

	if c.EventBus.Type != "" {
		if c.EventBus.Type == EventBusTypeKafka && len(c.EventBus.Brokers) == 0 {
			return fmt.Errorf("eventbus.brokers is required for Kafka")
		}
		if c.EventBus.Type == EventBusTypeRabbitMQ && c.EventBus.URL == "" {
			return fmt.Errorf("eventbus.url is required for RabbitMQ")
		}
		if c.EventBus.Type == EventBusTypeSQS {
			if c.EventBus.Region == "" {
				return fmt.Errorf("eventbus.region is required for SQS")
			}
			if c.EventBus.QueueURL == "" {
				return fmt.Errorf("eventbus.queue_url is required for SQS")
			}
		}
	}

	return nil
}

// String returns the full configuration as a formatted string
func (c *Config) String() string {
	return formatStruct(reflect.ValueOf(c).Elem(), "")
}

// Redacted returns the configuration with secrets masked.
// Pass the secrets Config returned by LoadWithSecrets() to mask those values.
func (c *Config) Redacted(secrets *Config) string {
	if secrets == nil {
		return c.String()
	}
	return formatStructWithMask(reflect.ValueOf(c).Elem(), reflect.ValueOf(secrets).Elem(), "")
}

func formatStruct(v reflect.Value, prefix string) string {
	var sb strings.Builder
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if !value.CanInterface() {
			continue
		}

		fieldName := field.Name
		if tag := field.Tag.Get("mapstructure"); tag != "" && tag != "-" {
			fieldName = tag
		}

		switch value.Kind() {
		case reflect.Struct:
			sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, fieldName))
			sb.WriteString(formatStruct(value, prefix+"  "))
		case reflect.Slice:
			if value.Len() == 0 {
				sb.WriteString(fmt.Sprintf("%s%s: []\n", prefix, fieldName))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, fieldName))
				for j := 0; j < value.Len(); j++ {
					elem := value.Index(j)
					sb.WriteString(fmt.Sprintf("%s  - %v\n", prefix, elem.Interface()))
				}
			}
		case reflect.Map:
			if value.Len() == 0 {
				sb.WriteString(fmt.Sprintf("%s%s: {}\n", prefix, fieldName))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, fieldName))
				for _, key := range value.MapKeys() {
					mapValue := value.MapIndex(key)
					sb.WriteString(fmt.Sprintf("%s  %v: %v\n", prefix, key.Interface(), mapValue.Interface()))
				}
			}
		default:
			sb.WriteString(fmt.Sprintf("%s%s: %v\n", prefix, fieldName, value.Interface()))
		}
	}

	return sb.String()
}

func formatStructWithMask(v, mask reflect.Value, prefix string) string {
	var sb strings.Builder
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		maskValue := mask.Field(i)

		if !value.CanInterface() {
			continue
		}

		fieldName := field.Name
		if tag := field.Tag.Get("mapstructure"); tag != "" && tag != "-" {
			fieldName = tag
		}

		switch value.Kind() {
		case reflect.Struct:
			sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, fieldName))
			sb.WriteString(formatStructWithMask(value, maskValue, prefix+"  "))
		case reflect.Slice:
			if value.Len() == 0 {
				sb.WriteString(fmt.Sprintf("%s%s: []\n", prefix, fieldName))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, fieldName))
				for j := 0; j < value.Len(); j++ {
					elem := value.Index(j)
					sb.WriteString(fmt.Sprintf("%s  - %v\n", prefix, elem.Interface()))
				}
			}
		case reflect.Map:
			if value.Len() == 0 {
				sb.WriteString(fmt.Sprintf("%s%s: {}\n", prefix, fieldName))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, fieldName))
				for _, key := range value.MapKeys() {
					mapValue := value.MapIndex(key)
					sb.WriteString(fmt.Sprintf("%s  %v: %v\n", prefix, key.Interface(), mapValue.Interface()))
				}
			}
		default:
			displayValue := value.Interface()
			// Check if this field has a non-zero value in secrets
			if shouldRedact(maskValue) {
				displayValue = "***"
			}
			sb.WriteString(fmt.Sprintf("%s%s: %v\n", prefix, fieldName, displayValue))
		}
	}

	return sb.String()
}

func shouldRedact(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}

	switch v.Kind() {
	case reflect.String:
		return v.String() != ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return v.Float() != 0
	case reflect.Bool:
		return v.Bool()
	case reflect.Slice, reflect.Map:
		return v.Len() > 0
	default:
		return false
	}
}
