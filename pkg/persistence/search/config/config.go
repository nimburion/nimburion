package config

import (
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/spf13/viper"
)

// Config configures search backends such as OpenSearch.
type Config struct {
	Type             string        `mapstructure:"type"`
	Driver           string        `mapstructure:"driver"`
	URL              string        `mapstructure:"url"`
	URLs             []string      `mapstructure:"urls"`
	Username         string        `mapstructure:"username"`
	Password         string        `mapstructure:"password"`
	APIKey           string        `mapstructure:"api_key"`
	AWSAuthEnabled   bool          `mapstructure:"aws_auth_enabled"`
	AWSRegion        string        `mapstructure:"aws_region"`
	AWSService       string        `mapstructure:"aws_service"`
	AWSAccessKeyID   string        `mapstructure:"aws_access_key_id"`
	AWSSecretKey     string        `mapstructure:"aws_secret_access_key"`
	AWSSessionToken  string        `mapstructure:"aws_session_token"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// Extension contributes the search config section as family-owned config surface.
type Extension struct {
	Search Config `mapstructure:"search"`
}

// DisabledCoreConfigSections disables the legacy monolithic root section when schema composition uses this family extension.
func (Extension) DisabledCoreConfigSections() []string { return []string{"search"} }

func (Extension) ApplyDefaults(v *viper.Viper) {
	v.SetDefault("search.type", "")
	v.SetDefault("search.driver", "http")
	v.SetDefault("search.url", "")
	v.SetDefault("search.urls", []string{})
	v.SetDefault("search.username", "")
	v.SetDefault("search.password", "")
	v.SetDefault("search.api_key", "")
	v.SetDefault("search.aws_auth_enabled", false)
	v.SetDefault("search.aws_region", "")
	v.SetDefault("search.aws_service", "es")
	v.SetDefault("search.aws_access_key_id", "")
	v.SetDefault("search.aws_secret_access_key", "")
	v.SetDefault("search.aws_session_token", "")
	v.SetDefault("search.max_conns", 10)
	v.SetDefault("search.operation_timeout", 30*time.Second)
}

func (Extension) BindEnv(v *viper.Viper, prefix string) error {
	return bindEnvPairs(v, prefix,
		"search.type", "SEARCH_TYPE",
		"search.driver", "SEARCH_DRIVER",
		"search.url", "SEARCH_URL",
		"search.urls", "SEARCH_URLS",
		"search.username", "SEARCH_USERNAME",
		"search.password", "SEARCH_PASSWORD",
		"search.api_key", "SEARCH_API_KEY",
		"search.aws_auth_enabled", "SEARCH_AWS_AUTH_ENABLED",
		"search.aws_region", "SEARCH_AWS_REGION",
		"search.aws_service", "SEARCH_AWS_SERVICE",
		"search.aws_access_key_id", "SEARCH_AWS_ACCESS_KEY_ID",
		"search.aws_secret_access_key", "SEARCH_AWS_SECRET_ACCESS_KEY",
		"search.aws_session_token", "SEARCH_AWS_SESSION_TOKEN",
		"search.max_conns", "SEARCH_MAX_CONNS",
		"search.operation_timeout", "SEARCH_OPERATION_TIMEOUT",
	)
}

func (e Extension) Validate() error {
	if strings.TrimSpace(e.Search.Type) == "" {
		return nil
	}
	searchType := strings.ToLower(strings.TrimSpace(e.Search.Type))
	validTypes := []string{"opensearch", "elasticsearch"}
	if !contains(validTypes, searchType) {
		return validationErrorf("validation.search.type.invalid", "invalid search.type: %s (must be one of: %v)", e.Search.Type, validTypes)
	}
	driver := strings.ToLower(strings.TrimSpace(e.Search.Driver))
	if driver == "" {
		driver = "http"
	}
	validDrivers := []string{"http", "opensearch-sdk", "elasticsearch-sdk"}
	if !contains(validDrivers, driver) {
		return validationErrorf("validation.search.driver.invalid", "invalid search.driver: %s (must be one of: %v)", e.Search.Driver, validDrivers)
	}
	if driver == "opensearch-sdk" && searchType != "opensearch" {
		return validationError("validation.search.driver.opensearch_sdk.type", "search.driver=opensearch-sdk requires search.type=opensearch")
	}
	if driver == "elasticsearch-sdk" && searchType != "elasticsearch" {
		return validationError("validation.search.driver.elasticsearch_sdk.type", "search.driver=elasticsearch-sdk requires search.type=elasticsearch")
	}
	if strings.TrimSpace(e.Search.URL) == "" && len(e.Search.URLs) == 0 {
		return validationError("validation.search.url.required", "search.url or search.urls is required when search.type is specified")
	}
	if e.Search.AWSAuthEnabled {
		if strings.TrimSpace(e.Search.AWSRegion) == "" {
			return validationError("validation.search.aws_region.required", "search.aws_region is required when search.aws_auth_enabled is true")
		}
		if strings.TrimSpace(e.Search.AWSService) == "" {
			return validationError("validation.search.aws_service.required", "search.aws_service is required when search.aws_auth_enabled is true")
		}
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
