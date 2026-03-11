package config

import (
	"github.com/spf13/viper"

	authconfig "github.com/nimburion/nimburion/pkg/auth/config"
	cacheconfig "github.com/nimburion/nimburion/pkg/cache/config"
	appconfig "github.com/nimburion/nimburion/pkg/core/app/config"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
	schemaconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
	corsconfig "github.com/nimburion/nimburion/pkg/http/cors/config"
	csrfconfig "github.com/nimburion/nimburion/pkg/http/csrf/config"
	httpsignature "github.com/nimburion/nimburion/pkg/http/httpsignature"
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

type defaultsContributor interface {
	ApplyDefaults(*viper.Viper)
}

type envContributor interface {
	BindEnv(*viper.Viper, string) error
}

func builtInConfigExtensions() []interface{} {
	return []interface{}{
		&appconfig.Extension{},
		&authconfig.Extension{},
		&cacheconfig.Extension{},
		&corsconfig.Extension{},
		&csrfconfig.Extension{},
		&emailconfig.Extension{},
		&eventbusconfig.Extension{},
		&schemaconfig.Extension{},
		&httpsignature.Extension{},
		&i18nconfig.Extension{},
		&openapiconfig.Extension{},
		&ratelimitconfig.Extension{},
		&serverconfig.Extension{},
		&securityheadersconfig.Extension{},
		&sseconfig.Extension{},
		&jobsconfig.Extension{},
		&observabilityconfig.Extension{},
		&persistenceconfig.Extension{},
		&objectconfig.Extension{},
		&searchconfig.Extension{},
		&schedulerconfig.Extension{},
		&sessionconfig.Extension{},
	}
}

func applyConfigDefaults(v *viper.Viper, target interface{}) error {
	if contributor, ok := target.(defaultsContributor); ok {
		contributor.ApplyDefaults(v)
		return nil
	}
	return applyExtensionDefaults(v, target)
}

func bindConfigEnv(v *viper.Viper, envPrefix string, target interface{}) error {
	if contributor, ok := target.(envContributor); ok {
		return contributor.BindEnv(v, envPrefix)
	}
	return bindExtensionEnv(v, target)
}

func remapCoreExtension(core *Config, extension interface{}) error {
	switch ext := extension.(type) {
	case *appconfig.Extension:
		ext.App = core.App
	case *authconfig.Extension:
		ext.Auth = core.Auth
	case *cacheconfig.Extension:
		ext.Cache = core.Cache
	case *corsconfig.Extension:
		ext.CORS = core.CORS
	case *csrfconfig.Extension:
		ext.CSRF = core.CSRF
	case *emailconfig.Extension:
		ext.Email = core.Email
	case *eventbusconfig.Extension:
		ext.EventBus = core.EventBus
	case *schemaconfig.Extension:
		ext.Validation = core.Validation
	case *httpsignature.Extension:
		ext.Security.HTTPSignature = core.Security.HTTPSignature
	case *i18nconfig.Extension:
		ext.I18n = core.I18n
	case *openapiconfig.Extension:
		ext.Swagger = core.Swagger
	case *ratelimitconfig.Extension:
		ext.RateLimit = core.RateLimit
	case *serverconfig.Extension:
		ext.HTTP = core.HTTP
		ext.Management = core.Management
	case *securityheadersconfig.Extension:
		ext.SecurityHeaders = core.SecurityHeaders
	case *sseconfig.Extension:
		ext.SSE = core.SSE
	case *jobsconfig.Extension:
		ext.Jobs = core.Jobs
	case *observabilityconfig.Extension:
		ext.Observability = core.Observability
	case *persistenceconfig.Extension:
		ext.Database = core.Database
	case *objectconfig.Extension:
		ext.ObjectStorage = core.ObjectStorage
	case *searchconfig.Extension:
		ext.Search = core.Search
	case *schedulerconfig.Extension:
		ext.Scheduler = core.Scheduler
	case *sessionconfig.Extension:
		ext.Session = core.Session
	}
	return nil
}
