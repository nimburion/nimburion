package config

import (
	authconfig "github.com/nimburion/nimburion/pkg/auth/config"
	cacheconfig "github.com/nimburion/nimburion/pkg/cache/config"
	appconfig "github.com/nimburion/nimburion/pkg/core/app/config"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemavalidationconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	corsconfig "github.com/nimburion/nimburion/pkg/http/cors/config"
	csrfconfig "github.com/nimburion/nimburion/pkg/http/csrf/config"
	httpsignatureconfig "github.com/nimburion/nimburion/pkg/http/httpsignature/config"
	i18nconfig "github.com/nimburion/nimburion/pkg/http/i18n/config"
	openapiconfig "github.com/nimburion/nimburion/pkg/http/openapi/config"
	ratelimitconfig "github.com/nimburion/nimburion/pkg/http/ratelimit/config"
	securityheadersconfig "github.com/nimburion/nimburion/pkg/http/securityheaders/config"
	serverconfig "github.com/nimburion/nimburion/pkg/http/server/config"
	sseconfig "github.com/nimburion/nimburion/pkg/http/sse/config"
	jobsconfig "github.com/nimburion/nimburion/pkg/jobs/config"
	observabilityconfig "github.com/nimburion/nimburion/pkg/observability/config"
	persistenceconfig "github.com/nimburion/nimburion/pkg/persistence/config"
	objectconfig "github.com/nimburion/nimburion/pkg/persistence/object/config"
	searchconfig "github.com/nimburion/nimburion/pkg/persistence/search/config"
	schedulerconfig "github.com/nimburion/nimburion/pkg/scheduler/config"
	sessionconfig "github.com/nimburion/nimburion/pkg/session/config"
)

// Config is the root configuration structure for the microservice framework
type Config struct {
	App             appconfig.AppConfig
	HTTP            serverconfig.HTTPConfig
	Management      serverconfig.ManagementConfig
	CORS            corsconfig.Config
	SecurityHeaders securityheadersconfig.Config       `mapstructure:"security_headers"`
	Security        httpsignatureconfig.SecurityConfig `mapstructure:"security"`
	I18n            i18nconfig.Config                  `mapstructure:"i18n"`
	Session         sessionconfig.Config               `mapstructure:"session"`
	CSRF            csrfconfig.Config                  `mapstructure:"csrf"`
	SSE             sseconfig.Config                   `mapstructure:"sse"`
	Email           emailconfig.Config                 `mapstructure:"email"`
	Auth            authconfig.Config
	Database        persistenceconfig.DatabaseConfig
	Cache           cacheconfig.Config
	ObjectStorage   objectconfig.Config `mapstructure:"object_storage"`
	Search          searchconfig.Config
	EventBus        eventbusconfig.Config  `mapstructure:"eventbus"`
	Jobs            jobsconfig.Config      `mapstructure:"jobs"`
	Scheduler       schedulerconfig.Config `mapstructure:"scheduler"`
	Validation      schemavalidationconfig.ValidationConfig
	Observability   observabilityconfig.Config
	Swagger         openapiconfig.SwaggerConfig
	RateLimit       ratelimitconfig.Config
}
