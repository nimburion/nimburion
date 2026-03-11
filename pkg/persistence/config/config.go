package config

import (
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/spf13/viper"
)

const (
	DatabaseTypePostgres = "postgres"
	DatabaseTypeMySQL    = "mysql"
	DatabaseTypeMongoDB  = "mongodb"
	DatabaseTypeDynamoDB = "dynamodb"
)

// DatabaseConfig configures persistence backends and shared connection settings.
type DatabaseConfig struct {
	Type            string        `mapstructure:"type"`
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	QueryTimeout    time.Duration `mapstructure:"query_timeout"`
	DatabaseName    string        `mapstructure:"database_name"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	Region          string        `mapstructure:"region"`
	Endpoint        string        `mapstructure:"endpoint"`
	AccessKeyID     string        `mapstructure:"access_key_id"`
	SecretAccessKey string        `mapstructure:"secret_access_key"`
	SessionToken    string        `mapstructure:"session_token"`
}

// Extension contributes the database config section as persistence-owned config surface.
type Extension struct {
	Database DatabaseConfig `mapstructure:"database"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"database"} }

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", 5*time.Minute)
	v.SetDefault("database.conn_max_idle_time", 2*time.Minute)
	v.SetDefault("database.query_timeout", 10*time.Second)
	v.SetDefault("database.connect_timeout", 5*time.Second)
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"database.type", "DB_TYPE",
		"database.url", "DB_URL",
		"database.max_open_conns", "DB_MAX_OPEN_CONNS",
		"database.max_idle_conns", "DB_MAX_IDLE_CONNS",
		"database.conn_max_lifetime", "DB_CONN_MAX_LIFETIME",
		"database.conn_max_idle_time", "DB_CONN_MAX_IDLE_TIME",
		"database.query_timeout", "DB_QUERY_TIMEOUT",
		"database.database_name", "DB_DATABASE_NAME",
		"database.connect_timeout", "DB_CONNECT_TIMEOUT",
		"database.region", "DB_REGION",
		"database.endpoint", "DB_ENDPOINT",
		"database.access_key_id", "DB_ACCESS_KEY_ID",
		"database.secret_access_key", "DB_SECRET_ACCESS_KEY",
		"database.session_token", "DB_SESSION_TOKEN",
	)
}

func (e Extension) Validate() error {
	if e.Database.Type == "" {
		return nil
	}
	validTypes := []string{DatabaseTypePostgres, DatabaseTypeMySQL, DatabaseTypeMongoDB, DatabaseTypeDynamoDB}
	if !contains(validTypes, e.Database.Type) {
		return validationErrorf("validation.database.type.invalid", "invalid database.type: %s (must be one of: %v)", e.Database.Type, validTypes)
	}
	if e.Database.Type != DatabaseTypeDynamoDB && strings.TrimSpace(e.Database.URL) == "" {
		return validationError("validation.database.url.required", "database.url is required when database.type is specified")
	}
	if e.Database.Type == DatabaseTypeMongoDB && strings.TrimSpace(e.Database.DatabaseName) == "" {
		return validationError("validation.database.database_name.required", "database.database_name is required when database.type is mongodb")
	}
	if e.Database.Type == DatabaseTypeDynamoDB && strings.TrimSpace(e.Database.Region) == "" {
		return validationError("validation.database.region.required", "database.region is required when database.type is dynamodb")
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
