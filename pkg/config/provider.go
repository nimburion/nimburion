package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type extensionValidator interface {
	Validate() error
}

type ConfigProvider struct {
	loader *ViperLoader
	v      *viper.Viper
	flags  *pflag.FlagSet
}

func NewConfigProvider(configFile, envPrefix string) *ConfigProvider {
	return &ConfigProvider{
		loader: NewViperLoader(configFile, envPrefix),
		v:      viper.New(),
	}
}

func (p *ConfigProvider) WithFlags(flags *pflag.FlagSet) *ConfigProvider {
	p.flags = flags
	return p
}

func (p *ConfigProvider) WithServiceNameDefault(serviceName string) *ConfigProvider {
	if p == nil || p.loader == nil {
		return p
	}
	p.loader.WithServiceNameDefault(serviceName)
	return p
}

// ConfigFile returns the path to the config file that was loaded, or empty string if none.
func (p *ConfigProvider) ConfigFile() string {
	if p.loader == nil {
		return ""
	}
	return p.loader.configFile
}

func (p *ConfigProvider) Load(core *Config, extensions ...interface{}) error {
	_, err := p.load(core, false, extensions...)
	return err
}

// LoadWithSecrets loads core + extension config including secrets merge.
// Returns the raw secrets map used for redaction (nil when no secrets file was loaded).
func (p *ConfigProvider) LoadWithSecrets(core *Config, extensions ...interface{}) (map[string]interface{}, error) {
	return p.load(core, true, extensions...)
}

// AllSettings returns the effective merged settings currently held by the provider.
func (p *ConfigProvider) AllSettings() map[string]interface{} {
	if p == nil || p.v == nil {
		return map[string]interface{}{}
	}
	return p.v.AllSettings()
}

func (p *ConfigProvider) load(core *Config, withSecrets bool, extensions ...interface{}) (map[string]interface{}, error) {
	p.v = viper.New()

	defaults := DefaultConfig()
	p.loader.setDefaults(p.v, defaults)

	for _, extension := range extensions {
		if err := applyExtensionDefaults(p.v, extension); err != nil {
			return nil, err
		}
	}

	if p.loader.configFile != "" {
		p.v.SetConfigFile(p.loader.configFile)
		if err := p.v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", p.loader.configFile, err)
		}
	}

	var secrets map[string]interface{}
	if withSecrets {
		secretsFile, _, err := p.loader.discoverSecretsFile()
		if err != nil {
			return nil, err
		}
		if secretsFile != "" {
			secretsViper := viper.New()
			secretsViper.SetConfigFile(secretsFile)
			if err := secretsViper.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("failed to read secrets file %s: %w", secretsFile, err)
			}
			secrets = secretsViper.AllSettings()
			if err := p.v.MergeConfigMap(secrets); err != nil {
				return nil, fmt.Errorf("failed to merge secrets: %w", err)
			}
		}
	}

	p.v.SetEnvPrefix(p.loader.envPrefix)
	p.loader.bindLegacyEnvVars()
	p.loader.bindEnvVars(p.v)

	for _, extension := range extensions {
		if err := bindExtensionEnv(p.v, extension); err != nil {
			return nil, err
		}
	}

	if p.flags != nil {
		for _, extension := range extensions {
			if err := applyExtensionFlags(p.v, p.flags, extension); err != nil {
				return nil, err
			}
		}
	}

	if core == nil {
		core = &Config{}
	}
	if err := p.v.Unmarshal(core); err != nil {
		return nil, fmt.Errorf("failed to unmarshal core config: %w", err)
	}
	if err := p.loader.Validate(core); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	for _, extension := range extensions {
		if err := p.v.Unmarshal(extension); err != nil {
			return nil, fmt.Errorf("failed to unmarshal extension config: %w", err)
		}
		if validator, ok := extension.(extensionValidator); ok {
			if err := validator.Validate(); err != nil {
				return nil, fmt.Errorf("extension config validation failed: %w", err)
			}
		}
	}

	return secrets, nil
}

func RegisterFlagsFromStruct(flags *pflag.FlagSet, target interface{}) error {
	fields, err := collectConfigFields(target)
	if err != nil {
		return err
	}

	for _, field := range fields {
		if field.Flag == "" {
			continue
		}
		usage := field.Usage
		if usage == "" {
			usage = "configuration override"
		}
		defaultValue, err := parseStringByType(field.Default, field.Type)
		if err != nil {
			return err
		}

		switch field.Type.Kind() {
		case reflect.String:
			flags.String(field.Flag, defaultValue.(string), usage)
		case reflect.Bool:
			flags.Bool(field.Flag, defaultValue.(bool), usage)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Type.PkgPath() == "time" && field.Type.Name() == "Duration" {
				flags.Duration(field.Flag, defaultValue.(time.Duration), usage)
			} else {
				flags.Int64(field.Flag, defaultValue.(int64), usage)
			}
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				flags.StringSlice(field.Flag, defaultValue.([]string), usage)
			}
		}
	}

	return nil
}

type configField struct {
	Key     string
	Env     []string
	Flag    string
	Default string
	Usage   string
	Type    reflect.Type
}

func applyExtensionDefaults(v *viper.Viper, target interface{}) error {
	fields, err := collectConfigFields(target)
	if err != nil {
		return err
	}
	for _, field := range fields {
		if field.Default == "" {
			continue
		}
		defaultValue, err := parseStringByType(field.Default, field.Type)
		if err != nil {
			return err
		}
		v.SetDefault(field.Key, defaultValue)
	}
	return nil
}

func bindExtensionEnv(v *viper.Viper, target interface{}) error {
	fields, err := collectConfigFields(target)
	if err != nil {
		return err
	}
	for _, field := range fields {
		if len(field.Env) == 0 {
			continue
		}
		args := append([]string{field.Key}, field.Env...)
		if err := v.BindEnv(args...); err != nil {
			return err
		}
	}
	return nil
}

func applyExtensionFlags(v *viper.Viper, flags *pflag.FlagSet, target interface{}) error {
	fields, err := collectConfigFields(target)
	if err != nil {
		return err
	}
	for _, field := range fields {
		if field.Flag == "" {
			continue
		}
		flag := flags.Lookup(field.Flag)
		if flag == nil || !flag.Changed {
			continue
		}

		parsed, err := parseStringByType(flag.Value.String(), field.Type)
		if err != nil {
			return fmt.Errorf("invalid value for --%s: %w", field.Flag, err)
		}
		v.Set(field.Key, parsed)
	}
	return nil
}

func collectConfigFields(target interface{}) ([]configField, error) {
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return nil, fmt.Errorf("config target must be a non-nil pointer")
	}
	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("config target must point to a struct")
	}

	fields := make([]configField, 0, elem.NumField())
	collectFieldsRecursive(elem.Type(), "", &fields)
	return fields, nil
}

func collectFieldsRecursive(structType reflect.Type, prefix string, out *[]configField) {
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
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

		fullKey := mapKey
		if prefix != "" {
			fullKey = prefix + "." + mapKey
		}

		fieldType := field.Type
		isDuration := fieldType.PkgPath() == "time" && fieldType.Name() == "Duration"
		if fieldType.Kind() == reflect.Struct && !isDuration {
			collectFieldsRecursive(fieldType, fullKey, out)
			continue
		}

		entry := configField{
			Key:     fullKey,
			Env:     parseListTag(field.Tag.Get("env")),
			Flag:    strings.TrimSpace(field.Tag.Get("flag")),
			Default: strings.TrimSpace(field.Tag.Get("default")),
			Usage:   strings.TrimSpace(field.Tag.Get("flag_usage")),
			Type:    fieldType,
		}
		*out = append(*out, entry)
	}
}

func parseStringByType(value string, fieldType reflect.Type) (interface{}, error) {
	trimmed := strings.TrimSpace(value)
	switch fieldType.Kind() {
	case reflect.String:
		return trimmed, nil
	case reflect.Bool:
		if trimmed == "" {
			return false, nil
		}
		parsed, err := strconv.ParseBool(trimmed)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fieldType.PkgPath() == "time" && fieldType.Name() == "Duration" {
			if trimmed == "" {
				return time.Duration(0), nil
			}
			parsed, err := time.ParseDuration(trimmed)
			if err != nil {
				return nil, err
			}
			return parsed, nil
		}
		if trimmed == "" {
			return int64(0), nil
		}
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case reflect.Slice:
		if fieldType.Elem().Kind() == reflect.String {
			return parseStringSlice(trimmed), nil
		}
	}
	return nil, fmt.Errorf("unsupported field type %s", fieldType.String())
}

func parseStringSlice(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}

	normalized := strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	normalized = strings.NewReplacer(",", " ", ";", " ", "\n", " ", "\t", " ").Replace(normalized)
	parts := strings.Fields(normalized)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func parseMapstructureTag(tag string) (string, bool) {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return "", false
	}
	parts := strings.Split(trimmed, ",")
	key := strings.TrimSpace(parts[0])
	if key == "-" {
		return "", true
	}
	return key, false
}

func parseListTag(tag string) []string {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return nil
	}
	rawParts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func toSnakeCase(input string) string {
	if input == "" {
		return input
	}
	var out strings.Builder
	out.Grow(len(input) + 8)
	for index, runeValue := range input {
		if index > 0 && isWordBoundary(input, index, runeValue) {
			out.WriteByte('_')
		}
		out.WriteRune(unicode.ToLower(runeValue))
	}
	return out.String()
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
