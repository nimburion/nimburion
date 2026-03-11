package config

import (
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/spf13/viper"
)

// Config configures object storage backends.
type Config struct {
	Enabled bool     `mapstructure:"enabled"`
	Type    string   `mapstructure:"type"`
	S3      S3Config `mapstructure:"s3"`
}

// S3Config configures S3-compatible object storage.
type S3Config struct {
	Bucket           string        `mapstructure:"bucket"`
	Region           string        `mapstructure:"region"`
	Endpoint         string        `mapstructure:"endpoint"`
	AccessKeyID      string        `mapstructure:"access_key_id"`
	SecretAccessKey  string        `mapstructure:"secret_access_key"`
	SessionToken     string        `mapstructure:"session_token"`
	UsePathStyle     bool          `mapstructure:"use_path_style"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	PresignExpiry    time.Duration `mapstructure:"presign_expiry"`
}

// Extension contributes the object_storage config section as family-owned config surface.
type Extension struct {
	ObjectStorage Config `mapstructure:"object_storage"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"object_storage"} }

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("object_storage.enabled", false)
	v.SetDefault("object_storage.type", "")
	v.SetDefault("object_storage.s3.bucket", "")
	v.SetDefault("object_storage.s3.region", "")
	v.SetDefault("object_storage.s3.endpoint", "")
	v.SetDefault("object_storage.s3.access_key_id", "")
	v.SetDefault("object_storage.s3.secret_access_key", "")
	v.SetDefault("object_storage.s3.session_token", "")
	v.SetDefault("object_storage.s3.use_path_style", false)
	v.SetDefault("object_storage.s3.operation_timeout", 30*time.Second)
	v.SetDefault("object_storage.s3.presign_expiry", 15*time.Minute)
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"object_storage.enabled", "OBJECT_STORAGE_ENABLED",
		"object_storage.type", "OBJECT_STORAGE_TYPE",
		"object_storage.s3.bucket", "OBJECT_STORAGE_S3_BUCKET",
		"object_storage.s3.region", "OBJECT_STORAGE_S3_REGION",
		"object_storage.s3.endpoint", "OBJECT_STORAGE_S3_ENDPOINT",
		"object_storage.s3.access_key_id", "OBJECT_STORAGE_S3_ACCESS_KEY_ID",
		"object_storage.s3.secret_access_key", "OBJECT_STORAGE_S3_SECRET_ACCESS_KEY",
		"object_storage.s3.session_token", "OBJECT_STORAGE_S3_SESSION_TOKEN",
		"object_storage.s3.use_path_style", "OBJECT_STORAGE_S3_USE_PATH_STYLE",
		"object_storage.s3.operation_timeout", "OBJECT_STORAGE_S3_OPERATION_TIMEOUT",
		"object_storage.s3.presign_expiry", "OBJECT_STORAGE_S3_PRESIGN_EXPIRY",
	)
}

func (e Extension) Validate() error {
	if !e.ObjectStorage.Enabled {
		return nil
	}
	storageType := strings.ToLower(strings.TrimSpace(e.ObjectStorage.Type))
	if storageType == "" {
		storageType = "s3"
	}
	validTypes := []string{"s3"}
	if !contains(validTypes, storageType) {
		return validationErrorf("validation.object_storage.type.invalid", "invalid object_storage.type: %s (must be one of: %v)", e.ObjectStorage.Type, validTypes)
	}
	if strings.TrimSpace(e.ObjectStorage.S3.Bucket) == "" {
		return validationError("validation.object_storage.s3.bucket.required", "object_storage.s3.bucket is required when object_storage.enabled is true and type=s3")
	}
	if strings.TrimSpace(e.ObjectStorage.S3.Region) == "" {
		return validationError("validation.object_storage.s3.region.required", "object_storage.s3.region is required when object_storage.enabled is true and type=s3")
	}
	if e.ObjectStorage.S3.OperationTimeout <= 0 {
		return validationError("validation.object_storage.s3.operation_timeout.invalid", "object_storage.s3.operation_timeout must be greater than zero when object_storage.enabled is true and type=s3")
	}
	if e.ObjectStorage.S3.PresignExpiry <= 0 {
		return validationError("validation.object_storage.s3.presign_expiry.invalid", "object_storage.s3.presign_expiry must be greater than zero when object_storage.enabled is true and type=s3")
	}
	return nil
}

func validationError(code, message string) error {
	return coreerrors.NewValidationWithCode(code, message, nil, nil)
}

func validationErrorf(code, format string, args ...any) error {
	return validationError(code, fmt.Sprintf(format, args...))
}

func bindEnvPairs(v *viper.Viper, prefix string, values ...string) error {
	for index := 0; index < len(values); index += 2 {
		if err := v.BindEnv(values[index], prefixedEnv(prefix, values[index+1])); err != nil {
			return err
		}
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
