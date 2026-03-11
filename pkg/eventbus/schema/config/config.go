package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// KafkaValidationConfig configures local schema validation for Kafka events.
type KafkaValidationConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	Mode           string            `mapstructure:"mode"`
	DescriptorPath string            `mapstructure:"descriptor_path"`
	DefaultPolicy  string            `mapstructure:"default_policy"`
	Subjects       map[string]string `mapstructure:"subjects"`
}

// ValidationConfig contributes the validation root section owned by the eventbus schema family.
type ValidationConfig struct {
	Kafka KafkaValidationConfig `mapstructure:"kafka"`
}

// Extension contributes the validation config section as family-owned config surface.
type Extension struct {
	Validation ValidationConfig `mapstructure:"validation"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"validation"} }

// ApplyDefaults registers default event schema validation configuration values.
func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("validation.kafka.enabled", false)
	v.SetDefault("validation.kafka.mode", "enforce")
	v.SetDefault("validation.kafka.descriptor_path", "")
	v.SetDefault("validation.kafka.default_policy", "BACKWARD")
	v.SetDefault("validation.kafka.subjects", map[string]string{})
}

// BindEnv binds event schema validation configuration keys to environment variables.
func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"validation.kafka.enabled", "VALIDATION_KAFKA_ENABLED",
		"validation.kafka.mode", "VALIDATION_KAFKA_MODE",
		"validation.kafka.descriptor_path", "VALIDATION_KAFKA_DESCRIPTOR_PATH",
		"validation.kafka.default_policy", "VALIDATION_KAFKA_DEFAULT_POLICY",
	)
}

// Validate checks that event schema validation configuration is coherent.
func (e Extension) Validate() error {
	validModes := []string{"warn", "enforce"}
	mode := strings.ToLower(strings.TrimSpace(e.Validation.Kafka.Mode))
	if mode == "" {
		mode = "enforce"
	}
	if !contains(validModes, mode) {
		return fmt.Errorf("invalid validation.kafka.mode: %s (must be one of: %v)", e.Validation.Kafka.Mode, validModes)
	}
	validCompatibility := []string{"backward", "full"}
	defaultPolicy := strings.ToLower(strings.TrimSpace(e.Validation.Kafka.DefaultPolicy))
	if defaultPolicy == "" {
		defaultPolicy = "backward"
	}
	if !contains(validCompatibility, defaultPolicy) {
		return fmt.Errorf("invalid validation.kafka.default_policy: %s (must be one of: %v)", e.Validation.Kafka.DefaultPolicy, []string{"BACKWARD", "FULL"})
	}
	if e.Validation.Kafka.Enabled && strings.TrimSpace(e.Validation.Kafka.DescriptorPath) == "" {
		return errors.New("validation.kafka.descriptor_path is required when validation.kafka.enabled is true")
	}
	for subject, policy := range e.Validation.Kafka.Subjects {
		if strings.TrimSpace(subject) == "" {
			return errors.New("validation.kafka.subjects contains an empty subject")
		}
		if !contains(validCompatibility, strings.ToLower(strings.TrimSpace(policy))) {
			return fmt.Errorf("invalid validation.kafka.subjects[%s]: %s (must be one of: %v)", subject, policy, []string{"BACKWARD", "FULL"})
		}
	}
	return nil
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	if len(values)%2 != 0 {
		return fmt.Errorf("bindEnvPairs requires even number of values, got %d", len(values))
	}
	for len(values) > 0 {
		key, suffix := values[0], values[1]
		if err := v.BindEnv(key, prefixedEnv(prefix, suffix)); err != nil {
			return err
		}
		values = values[2:]
	}
	return nil
}

func prefixedEnv(prefix, suffix string) string {
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.TrimSpace(prefix) + "_" + suffix
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
