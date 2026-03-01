package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Loader defines the interface for loading configuration
type Loader interface {
	Load() (*Config, error)
	Validate(*Config) error
}

// ViperLoader implements Loader using Viper for configuration management
type ViperLoader struct {
	configFile         string
	envPrefix          string
	serviceNameDefault string
}

// NewViperLoader creates a new ViperLoader
// configFile: path to configuration file (optional, can be empty)
// envPrefix: prefix for environment variables (e.g., "APP")
func NewViperLoader(configFile, envPrefix string) *ViperLoader {
	return &ViperLoader{
		configFile: configFile,
		envPrefix:  envPrefix,
	}
}

// WithServiceNameDefault sets the default service.name used when no config/env override is provided.
func (l *ViperLoader) WithServiceNameDefault(serviceName string) *ViperLoader {
	if l == nil {
		return l
	}
	l.serviceNameDefault = strings.TrimSpace(serviceName)
	return l
}

// Load loads configuration with precedence: ENV > file > defaults
func (l *ViperLoader) Load() (*Config, error) {
	v := viper.New()

	// Start with defaults
	defaults := DefaultConfig()
	l.setDefaults(v, defaults)

	// Read config file if provided
	if l.configFile != "" {
		v.SetConfigFile(l.configFile)
		if err := v.ReadInConfig(); err != nil {
			// Only return error if file was explicitly specified but couldn't be read
			return nil, fmt.Errorf("failed to read config file %s: %w", l.configFile, err)
		}
	}

	// Environment variables override file config through explicit bindings.
	v.SetEnvPrefix(l.envPrefix)

	// Map legacy env names to standard abbreviated keys when needed.
	l.bindLegacyEnvVars()

	// Bind all environment variables explicitly for nested structs
	if err := l.bindEnvVars(v); err != nil {
		return nil, fmt.Errorf("failed to bind environment variables: %w", err)
	}

	// Unmarshal into a new config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := l.Validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// bindEnvVars explicitly binds environment variables for nested structs.
func (l *ViperLoader) bindEnvVars(v *viper.Viper) error {
	var bindErr error
	bind := func(key string, envs ...string) {
		if bindErr != nil {
			return
		}
		args := append([]string{key}, envs...)
		bindErr = v.BindEnv(args...)
	}

	// Router
	bind("router_type", l.prefixedEnv("ROUTER_TYPE"))
	bind("service.name", l.prefixedEnv("SERVICE_NAME"))
	bind("service.environment", l.prefixedEnv("SERVICE_ENVIRONMENT"), l.prefixedEnv("ENVIRONMENT"))

	// HTTP
	bind("http.port", l.prefixedEnv("HTTP_PORT"))
	bind("http.read_timeout", l.prefixedEnv("HTTP_READ_TIMEOUT"))
	bind("http.write_timeout", l.prefixedEnv("HTTP_WRITE_TIMEOUT"))
	bind("http.idle_timeout", l.prefixedEnv("HTTP_IDLE_TIMEOUT"))
	bind("http.max_request_size", l.prefixedEnv("HTTP_MAX_REQUEST_SIZE"))

	// Management
	bind("management.enabled", l.prefixedEnv("MGMT_ENABLED"))
	bind("management.port", l.prefixedEnv("MGMT_PORT"))
	bind("management.read_timeout", l.prefixedEnv("MGMT_READ_TIMEOUT"))
	bind("management.write_timeout", l.prefixedEnv("MGMT_WRITE_TIMEOUT"))
	bind("management.auth_enabled", l.prefixedEnv("MGMT_AUTH_ENABLED"))
	bind("management.mtls_enabled", l.prefixedEnv("MGMT_MTLS_ENABLED"))
	bind("management.tls_cert_file", l.prefixedEnv("MGMT_TLS_CERT_FILE"))
	bind("management.tls_key_file", l.prefixedEnv("MGMT_TLS_KEY_FILE"))
	bind("management.tls_ca_file", l.prefixedEnv("MGMT_TLS_CA_FILE"))

	// CORS
	bind("cors.enabled", l.prefixedEnv("CORS_ENABLED"))
	bind("cors.allow_all_origins", l.prefixedEnv("CORS_ALLOW_ALL_ORIGINS"))
	bind("cors.allow_origins", l.prefixedEnv("CORS_ALLOW_ORIGINS"))
	bind("cors.allow_methods", l.prefixedEnv("CORS_ALLOW_METHODS"))
	bind("cors.allow_private_network", l.prefixedEnv("CORS_ALLOW_PRIVATE_NETWORK"))
	bind("cors.allow_headers", l.prefixedEnv("CORS_ALLOW_HEADERS"))
	bind("cors.expose_headers", l.prefixedEnv("CORS_EXPOSE_HEADERS"))
	bind("cors.allow_credentials", l.prefixedEnv("CORS_ALLOW_CREDENTIALS"))
	bind("cors.max_age", l.prefixedEnv("CORS_MAX_AGE"))
	bind("cors.allow_wildcard", l.prefixedEnv("CORS_ALLOW_WILDCARD"))
	bind("cors.allow_browser_extensions", l.prefixedEnv("CORS_ALLOW_BROWSER_EXTENSIONS"))
	bind("cors.custom_schemas", l.prefixedEnv("CORS_CUSTOM_SCHEMAS"))
	bind("cors.allow_websockets", l.prefixedEnv("CORS_ALLOW_WEBSOCKETS"))
	bind("cors.allow_files", l.prefixedEnv("CORS_ALLOW_FILES"))
	bind("cors.options_response_status_code", l.prefixedEnv("CORS_OPTIONS_RESPONSE_STATUS_CODE"))

	// Security headers
	bind("security_headers.enabled", l.prefixedEnv("SECURITY_HEADERS_ENABLED"))
	bind("security_headers.is_development", l.prefixedEnv("SECURITY_HEADERS_IS_DEVELOPMENT"))
	bind("security_headers.allowed_hosts", l.prefixedEnv("SECURITY_HEADERS_ALLOWED_HOSTS"))
	bind("security_headers.ssl_redirect", l.prefixedEnv("SECURITY_HEADERS_SSL_REDIRECT"))
	bind("security_headers.ssl_temporary_redirect", l.prefixedEnv("SECURITY_HEADERS_SSL_TEMPORARY_REDIRECT"))
	bind("security_headers.ssl_host", l.prefixedEnv("SECURITY_HEADERS_SSL_HOST"))
	bind("security_headers.dont_redirect_ipv4_hostnames", l.prefixedEnv("SECURITY_HEADERS_DONT_REDIRECT_IPV4_HOSTNAMES"))
	bind("security_headers.sts_seconds", l.prefixedEnv("SECURITY_HEADERS_STS_SECONDS"))
	bind("security_headers.sts_include_subdomains", l.prefixedEnv("SECURITY_HEADERS_STS_INCLUDE_SUBDOMAINS"))
	bind("security_headers.sts_preload", l.prefixedEnv("SECURITY_HEADERS_STS_PRELOAD"))
	bind("security_headers.custom_frame_options", l.prefixedEnv("SECURITY_HEADERS_CUSTOM_FRAME_OPTIONS"))
	bind("security_headers.content_type_nosniff", l.prefixedEnv("SECURITY_HEADERS_CONTENT_TYPE_NOSNIFF"))
	bind("security_headers.content_security_policy", l.prefixedEnv("SECURITY_HEADERS_CONTENT_SECURITY_POLICY"))
	bind("security_headers.referrer_policy", l.prefixedEnv("SECURITY_HEADERS_REFERRER_POLICY"))
	bind("security_headers.permissions_policy", l.prefixedEnv("SECURITY_HEADERS_PERMISSIONS_POLICY"))
	bind("security_headers.ie_no_open", l.prefixedEnv("SECURITY_HEADERS_IE_NO_OPEN"))
	bind("security_headers.x_dns_prefetch_control", l.prefixedEnv("SECURITY_HEADERS_X_DNS_PREFETCH_CONTROL"))
	bind("security_headers.cross_origin_opener_policy", l.prefixedEnv("SECURITY_HEADERS_CROSS_ORIGIN_OPENER_POLICY"))
	bind("security_headers.cross_origin_resource_policy", l.prefixedEnv("SECURITY_HEADERS_CROSS_ORIGIN_RESOURCE_POLICY"))
	bind("security_headers.cross_origin_embedder_policy", l.prefixedEnv("SECURITY_HEADERS_CROSS_ORIGIN_EMBEDDER_POLICY"))

	// Security (middleware)
	bind("security.http_signature.enabled", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_ENABLED"))
	bind("security.http_signature.key_id_header", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_KEY_ID_HEADER"))
	bind("security.http_signature.timestamp_header", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_TIMESTAMP_HEADER"))
	bind("security.http_signature.nonce_header", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_NONCE_HEADER"))
	bind("security.http_signature.signature_header", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_SIGNATURE_HEADER"))
	bind("security.http_signature.max_clock_skew", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_MAX_CLOCK_SKEW"))
	bind("security.http_signature.nonce_ttl", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_NONCE_TTL"))
	bind("security.http_signature.require_nonce", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_REQUIRE_NONCE"))
	bind("security.http_signature.excluded_path_prefixes", l.prefixedEnv("SECURITY_HTTP_SIGNATURE_EXCLUDED_PATH_PREFIXES"))

	// I18n
	bind("i18n.enabled", l.prefixedEnv("I18N_ENABLED"))
	bind("i18n.default_locale", l.prefixedEnv("I18N_DEFAULT_LOCALE"))
	bind("i18n.supported_locales", l.prefixedEnv("I18N_SUPPORTED_LOCALES"))
	bind("i18n.query_param", l.prefixedEnv("I18N_QUERY_PARAM"))
	bind("i18n.header_name", l.prefixedEnv("I18N_HEADER_NAME"))
	bind("i18n.fallback_mode", l.prefixedEnv("I18N_FALLBACK_MODE"))
	bind("i18n.catalog_path", l.prefixedEnv("I18N_CATALOG_PATH"))

	// Session
	bind("session.enabled", l.prefixedEnv("SESSION_ENABLED"))
	bind("session.store", l.prefixedEnv("SESSION_STORE"))
	bind("session.ttl", l.prefixedEnv("SESSION_TTL"))
	bind("session.idle_timeout", l.prefixedEnv("SESSION_IDLE_TIMEOUT"))
	bind("session.cookie_name", l.prefixedEnv("SESSION_COOKIE_NAME"))
	bind("session.cookie_path", l.prefixedEnv("SESSION_COOKIE_PATH"))
	bind("session.cookie_domain", l.prefixedEnv("SESSION_COOKIE_DOMAIN"))
	bind("session.cookie_secure", l.prefixedEnv("SESSION_COOKIE_SECURE"))
	bind("session.cookie_http_only", l.prefixedEnv("SESSION_COOKIE_HTTP_ONLY"))
	bind("session.cookie_same_site", l.prefixedEnv("SESSION_COOKIE_SAME_SITE"))
	bind("session.auto_create", l.prefixedEnv("SESSION_AUTO_CREATE"))
	bind("session.redis.url", l.prefixedEnv("SESSION_REDIS_URL"))
	bind("session.redis.max_conns", l.prefixedEnv("SESSION_REDIS_MAX_CONNS"))
	bind("session.redis.operation_timeout", l.prefixedEnv("SESSION_REDIS_OPERATION_TIMEOUT"))
	bind("session.redis.prefix", l.prefixedEnv("SESSION_REDIS_PREFIX"))
	bind("session.memcached.addresses", l.prefixedEnv("SESSION_MEMCACHED_ADDRESSES"))
	bind("session.memcached.timeout", l.prefixedEnv("SESSION_MEMCACHED_TIMEOUT"))
	bind("session.memcached.prefix", l.prefixedEnv("SESSION_MEMCACHED_PREFIX"))

	// CSRF
	bind("csrf.enabled", l.prefixedEnv("CSRF_ENABLED"))
	bind("csrf.header_name", l.prefixedEnv("CSRF_HEADER_NAME"))
	bind("csrf.cookie_name", l.prefixedEnv("CSRF_COOKIE_NAME"))
	bind("csrf.cookie_path", l.prefixedEnv("CSRF_COOKIE_PATH"))
	bind("csrf.cookie_domain", l.prefixedEnv("CSRF_COOKIE_DOMAIN"))
	bind("csrf.cookie_secure", l.prefixedEnv("CSRF_COOKIE_SECURE"))
	bind("csrf.cookie_same_site", l.prefixedEnv("CSRF_COOKIE_SAME_SITE"))
	bind("csrf.cookie_ttl", l.prefixedEnv("CSRF_COOKIE_TTL"))
	bind("csrf.exempt_methods", l.prefixedEnv("CSRF_EXEMPT_METHODS"))
	bind("csrf.exempt_paths", l.prefixedEnv("CSRF_EXEMPT_PATHS"))

	// SSE
	bind("sse.enabled", l.prefixedEnv("SSE_ENABLED"))
	bind("sse.endpoint", l.prefixedEnv("SSE_ENDPOINT"))
	bind("sse.store", l.prefixedEnv("SSE_STORE"))
	bind("sse.bus", l.prefixedEnv("SSE_BUS"))
	bind("sse.replay_limit", l.prefixedEnv("SSE_REPLAY_LIMIT"))
	bind("sse.client_buffer", l.prefixedEnv("SSE_CLIENT_BUFFER"))
	bind("sse.max_connections", l.prefixedEnv("SSE_MAX_CONNECTIONS"))
	bind("sse.heartbeat_interval", l.prefixedEnv("SSE_HEARTBEAT_INTERVAL"))
	bind("sse.default_retry_ms", l.prefixedEnv("SSE_DEFAULT_RETRY_MS"))
	bind("sse.drop_on_backpressure", l.prefixedEnv("SSE_DROP_ON_BACKPRESSURE"))
	bind("sse.channel_query_param", l.prefixedEnv("SSE_CHANNEL_QUERY_PARAM"))
	bind("sse.tenant_query_param", l.prefixedEnv("SSE_TENANT_QUERY_PARAM"))
	bind("sse.subject_query_param", l.prefixedEnv("SSE_SUBJECT_QUERY_PARAM"))
	bind("sse.last_event_id_query_param", l.prefixedEnv("SSE_LAST_EVENT_ID_QUERY_PARAM"))
	bind("sse.redis.url", l.prefixedEnv("SSE_REDIS_URL"))
	bind("sse.redis.max_conns", l.prefixedEnv("SSE_REDIS_MAX_CONNS"))
	bind("sse.redis.operation_timeout", l.prefixedEnv("SSE_REDIS_OPERATION_TIMEOUT"))
	bind("sse.redis.history_prefix", l.prefixedEnv("SSE_REDIS_HISTORY_PREFIX"))
	bind("sse.redis.pubsub_prefix", l.prefixedEnv("SSE_REDIS_PUBSUB_PREFIX"))
	bind("sse.eventbus.topic_prefix", l.prefixedEnv("SSE_EVENTBUS_TOPIC_PREFIX"))
	bind("sse.eventbus.operation_timeout", l.prefixedEnv("SSE_EVENTBUS_OPERATION_TIMEOUT"))

	// Email
	bind("email.enabled", l.prefixedEnv("EMAIL_ENABLED"))
	bind("email.provider", l.prefixedEnv("EMAIL_PROVIDER"))
	bind("email.smtp.host", l.prefixedEnv("EMAIL_SMTP_HOST"))
	bind("email.smtp.port", l.prefixedEnv("EMAIL_SMTP_PORT"))
	bind("email.smtp.username", l.prefixedEnv("EMAIL_SMTP_USERNAME"))
	bind("email.smtp.password", l.prefixedEnv("EMAIL_SMTP_PASSWORD"))
	bind("email.smtp.from", l.prefixedEnv("EMAIL_SMTP_FROM"))
	bind("email.smtp.enable_tls", l.prefixedEnv("EMAIL_SMTP_ENABLE_TLS"))
	bind("email.smtp.insecure_skip_verify", l.prefixedEnv("EMAIL_SMTP_INSECURE_SKIP_VERIFY"))
	bind("email.smtp.operation_timeout", l.prefixedEnv("EMAIL_SMTP_OPERATION_TIMEOUT"))
	bind("email.ses.region", l.prefixedEnv("EMAIL_SES_REGION"))
	bind("email.ses.endpoint", l.prefixedEnv("EMAIL_SES_ENDPOINT"))
	bind("email.ses.access_key_id", l.prefixedEnv("EMAIL_SES_ACCESS_KEY_ID"))
	bind("email.ses.secret_access_key", l.prefixedEnv("EMAIL_SES_SECRET_ACCESS_KEY"))
	bind("email.ses.session_token", l.prefixedEnv("EMAIL_SES_SESSION_TOKEN"))
	bind("email.ses.from", l.prefixedEnv("EMAIL_SES_FROM"))
	bind("email.ses.operation_timeout", l.prefixedEnv("EMAIL_SES_OPERATION_TIMEOUT"))
	bind("email.sendgrid.token", l.prefixedEnv("EMAIL_SENDGRID_TOKEN"))
	bind("email.sendgrid.from", l.prefixedEnv("EMAIL_SENDGRID_FROM"))
	bind("email.sendgrid.base_url", l.prefixedEnv("EMAIL_SENDGRID_BASE_URL"))
	bind("email.sendgrid.operation_timeout", l.prefixedEnv("EMAIL_SENDGRID_OPERATION_TIMEOUT"))
	bind("email.mailgun.token", l.prefixedEnv("EMAIL_MAILGUN_TOKEN"))
	bind("email.mailgun.domain", l.prefixedEnv("EMAIL_MAILGUN_DOMAIN"))
	bind("email.mailgun.from", l.prefixedEnv("EMAIL_MAILGUN_FROM"))
	bind("email.mailgun.base_url", l.prefixedEnv("EMAIL_MAILGUN_BASE_URL"))
	bind("email.mailgun.operation_timeout", l.prefixedEnv("EMAIL_MAILGUN_OPERATION_TIMEOUT"))
	bind("email.mailchimp.token", l.prefixedEnv("EMAIL_MAILCHIMP_TOKEN"))
	bind("email.mailchimp.from", l.prefixedEnv("EMAIL_MAILCHIMP_FROM"))
	bind("email.mailchimp.base_url", l.prefixedEnv("EMAIL_MAILCHIMP_BASE_URL"))
	bind("email.mailchimp.operation_timeout", l.prefixedEnv("EMAIL_MAILCHIMP_OPERATION_TIMEOUT"))
	bind("email.mailersend.token", l.prefixedEnv("EMAIL_MAILERSEND_TOKEN"))
	bind("email.mailersend.from", l.prefixedEnv("EMAIL_MAILERSEND_FROM"))
	bind("email.mailersend.base_url", l.prefixedEnv("EMAIL_MAILERSEND_BASE_URL"))
	bind("email.mailersend.operation_timeout", l.prefixedEnv("EMAIL_MAILERSEND_OPERATION_TIMEOUT"))
	bind("email.postmark.server_token", l.prefixedEnv("EMAIL_POSTMARK_SERVER_TOKEN"))
	bind("email.postmark.from", l.prefixedEnv("EMAIL_POSTMARK_FROM"))
	bind("email.postmark.base_url", l.prefixedEnv("EMAIL_POSTMARK_BASE_URL"))
	bind("email.postmark.operation_timeout", l.prefixedEnv("EMAIL_POSTMARK_OPERATION_TIMEOUT"))
	bind("email.mailtrap.token", l.prefixedEnv("EMAIL_MAILTRAP_TOKEN"))
	bind("email.mailtrap.from", l.prefixedEnv("EMAIL_MAILTRAP_FROM"))
	bind("email.mailtrap.base_url", l.prefixedEnv("EMAIL_MAILTRAP_BASE_URL"))
	bind("email.mailtrap.operation_timeout", l.prefixedEnv("EMAIL_MAILTRAP_OPERATION_TIMEOUT"))
	bind("email.smtp2go.token", l.prefixedEnv("EMAIL_SMTP2GO_TOKEN"))
	bind("email.smtp2go.from", l.prefixedEnv("EMAIL_SMTP2GO_FROM"))
	bind("email.smtp2go.base_url", l.prefixedEnv("EMAIL_SMTP2GO_BASE_URL"))
	bind("email.smtp2go.operation_timeout", l.prefixedEnv("EMAIL_SMTP2GO_OPERATION_TIMEOUT"))
	bind("email.sendpulse.token", l.prefixedEnv("EMAIL_SENDPULSE_TOKEN"))
	bind("email.sendpulse.from", l.prefixedEnv("EMAIL_SENDPULSE_FROM"))
	bind("email.sendpulse.base_url", l.prefixedEnv("EMAIL_SENDPULSE_BASE_URL"))
	bind("email.sendpulse.operation_timeout", l.prefixedEnv("EMAIL_SENDPULSE_OPERATION_TIMEOUT"))
	bind("email.brevo.token", l.prefixedEnv("EMAIL_BREVO_TOKEN"))
	bind("email.brevo.from", l.prefixedEnv("EMAIL_BREVO_FROM"))
	bind("email.brevo.base_url", l.prefixedEnv("EMAIL_BREVO_BASE_URL"))
	bind("email.brevo.operation_timeout", l.prefixedEnv("EMAIL_BREVO_OPERATION_TIMEOUT"))
	bind("email.mailjet.api_key", l.prefixedEnv("EMAIL_MAILJET_API_KEY"))
	bind("email.mailjet.api_secret", l.prefixedEnv("EMAIL_MAILJET_API_SECRET"))
	bind("email.mailjet.from", l.prefixedEnv("EMAIL_MAILJET_FROM"))
	bind("email.mailjet.base_url", l.prefixedEnv("EMAIL_MAILJET_BASE_URL"))
	bind("email.mailjet.operation_timeout", l.prefixedEnv("EMAIL_MAILJET_OPERATION_TIMEOUT"))

	// Auth
	bind("auth.enabled", l.prefixedEnv("AUTH_ENABLED"))
	bind("auth.issuer", l.prefixedEnv("AUTH_ISSUER"))
	bind("auth.jwks_url", l.prefixedEnv("AUTH_JWKS_URL"))
	bind("auth.jwks_cache_ttl", l.prefixedEnv("AUTH_JWKS_CACHE_TTL"))
	bind("auth.audience", l.prefixedEnv("AUTH_AUDIENCE"))

	// Database
	bind("database.type", l.prefixedEnv("DB_TYPE"))
	bind("database.url", l.prefixedEnv("DB_URL"))
	bind("database.max_open_conns", l.prefixedEnv("DB_MAX_OPEN_CONNS"))
	bind("database.max_idle_conns", l.prefixedEnv("DB_MAX_IDLE_CONNS"))
	bind("database.conn_max_lifetime", l.prefixedEnv("DB_CONN_MAX_LIFETIME"))
	bind("database.conn_max_idle_time", l.prefixedEnv("DB_CONN_MAX_IDLE_TIME"))
	bind("database.query_timeout", l.prefixedEnv("DB_QUERY_TIMEOUT"))
	bind("database.database_name", l.prefixedEnv("DB_DATABASE_NAME"))
	bind("database.connect_timeout", l.prefixedEnv("DB_CONNECT_TIMEOUT"))
	bind("database.region", l.prefixedEnv("DB_REGION"))
	bind("database.endpoint", l.prefixedEnv("DB_ENDPOINT"))
	bind("database.access_key_id", l.prefixedEnv("DB_ACCESS_KEY_ID"))
	bind("database.secret_access_key", l.prefixedEnv("DB_SECRET_ACCESS_KEY"))
	bind("database.session_token", l.prefixedEnv("DB_SESSION_TOKEN"))

	// Cache
	bind("cache.type", l.prefixedEnv("CACHE_TYPE"))
	bind("cache.url", l.prefixedEnv("CACHE_URL"))
	bind("cache.max_conns", l.prefixedEnv("CACHE_MAX_CONNS"))
	bind("cache.operation_timeout", l.prefixedEnv("CACHE_OPERATION_TIMEOUT"))

	// Object storage
	bind("object_storage.enabled", l.prefixedEnv("OBJECT_STORAGE_ENABLED"))
	bind("object_storage.type", l.prefixedEnv("OBJECT_STORAGE_TYPE"))
	bind("object_storage.s3.bucket", l.prefixedEnv("OBJECT_STORAGE_S3_BUCKET"))
	bind("object_storage.s3.region", l.prefixedEnv("OBJECT_STORAGE_S3_REGION"))
	bind("object_storage.s3.endpoint", l.prefixedEnv("OBJECT_STORAGE_S3_ENDPOINT"))
	bind("object_storage.s3.access_key_id", l.prefixedEnv("OBJECT_STORAGE_S3_ACCESS_KEY_ID"))
	bind("object_storage.s3.secret_access_key", l.prefixedEnv("OBJECT_STORAGE_S3_SECRET_ACCESS_KEY"))
	bind("object_storage.s3.session_token", l.prefixedEnv("OBJECT_STORAGE_S3_SESSION_TOKEN"))
	bind("object_storage.s3.use_path_style", l.prefixedEnv("OBJECT_STORAGE_S3_USE_PATH_STYLE"))
	bind("object_storage.s3.operation_timeout", l.prefixedEnv("OBJECT_STORAGE_S3_OPERATION_TIMEOUT"))
	bind("object_storage.s3.presign_expiry", l.prefixedEnv("OBJECT_STORAGE_S3_PRESIGN_EXPIRY"))

	// Search
	bind("search.type", l.prefixedEnv("SEARCH_TYPE"))
	bind("search.driver", l.prefixedEnv("SEARCH_DRIVER"))
	bind("search.url", l.prefixedEnv("SEARCH_URL"))
	bind("search.urls", l.prefixedEnv("SEARCH_URLS"))
	bind("search.username", l.prefixedEnv("SEARCH_USERNAME"))
	bind("search.password", l.prefixedEnv("SEARCH_PASSWORD"))
	bind("search.api_key", l.prefixedEnv("SEARCH_API_KEY"))
	bind("search.aws_auth_enabled", l.prefixedEnv("SEARCH_AWS_AUTH_ENABLED"))
	bind("search.aws_region", l.prefixedEnv("SEARCH_AWS_REGION"))
	bind("search.aws_service", l.prefixedEnv("SEARCH_AWS_SERVICE"))
	bind("search.aws_access_key_id", l.prefixedEnv("SEARCH_AWS_ACCESS_KEY_ID"))
	bind("search.aws_secret_access_key", l.prefixedEnv("SEARCH_AWS_SECRET_ACCESS_KEY"))
	bind("search.aws_session_token", l.prefixedEnv("SEARCH_AWS_SESSION_TOKEN"))
	bind("search.max_conns", l.prefixedEnv("SEARCH_MAX_CONNS"))
	bind("search.operation_timeout", l.prefixedEnv("SEARCH_OPERATION_TIMEOUT"))

	// EventBus
	bind("eventbus.type", l.prefixedEnv("EVENTBUS_TYPE"))
	bind("eventbus.brokers", l.prefixedEnv("EVENTBUS_BROKERS"))
	bind("eventbus.serializer", l.prefixedEnv("EVENTBUS_SERIALIZER"))
	bind("eventbus.operation_timeout", l.prefixedEnv("EVENTBUS_OPERATION_TIMEOUT"))
	bind("eventbus.group_id", l.prefixedEnv("EVENTBUS_GROUP_ID"))
	bind("eventbus.url", l.prefixedEnv("EVENTBUS_URL"))
	bind("eventbus.exchange", l.prefixedEnv("EVENTBUS_EXCHANGE"))
	bind("eventbus.exchange_type", l.prefixedEnv("EVENTBUS_EXCHANGE_TYPE"))
	bind("eventbus.queue_name", l.prefixedEnv("EVENTBUS_QUEUE_NAME"))
	bind("eventbus.routing_key", l.prefixedEnv("EVENTBUS_ROUTING_KEY"))
	bind("eventbus.consumer_tag", l.prefixedEnv("EVENTBUS_CONSUMER_TAG"))
	bind("eventbus.region", l.prefixedEnv("EVENTBUS_REGION"))
	bind("eventbus.queue_url", l.prefixedEnv("EVENTBUS_QUEUE_URL"))
	bind("eventbus.endpoint", l.prefixedEnv("EVENTBUS_ENDPOINT"))
	bind("eventbus.access_key_id", l.prefixedEnv("EVENTBUS_ACCESS_KEY_ID"))
	bind("eventbus.secret_access_key", l.prefixedEnv("EVENTBUS_SECRET_ACCESS_KEY"))
	bind("eventbus.session_token", l.prefixedEnv("EVENTBUS_SESSION_TOKEN"))
	bind("eventbus.wait_time_seconds", l.prefixedEnv("EVENTBUS_WAIT_TIME_SECONDS"))
	bind("eventbus.max_messages", l.prefixedEnv("EVENTBUS_MAX_MESSAGES"))
	bind("eventbus.visibility_timeout", l.prefixedEnv("EVENTBUS_VISIBILITY_TIMEOUT"))

	// Jobs
	bind("jobs.backend", l.prefixedEnv("JOBS_BACKEND"))
	bind("jobs.default_queue", l.prefixedEnv("JOBS_DEFAULT_QUEUE"))
	bind("jobs.worker.concurrency", l.prefixedEnv("JOBS_WORKER_CONCURRENCY"))
	bind("jobs.worker.lease_ttl", l.prefixedEnv("JOBS_WORKER_LEASE_TTL"))
	bind("jobs.worker.reserve_timeout", l.prefixedEnv("JOBS_WORKER_RESERVE_TIMEOUT"))
	bind("jobs.worker.stop_timeout", l.prefixedEnv("JOBS_WORKER_STOP_TIMEOUT"))
	bind("jobs.retry.max_attempts", l.prefixedEnv("JOBS_RETRY_MAX_ATTEMPTS"))
	bind("jobs.retry.initial_backoff", l.prefixedEnv("JOBS_RETRY_INITIAL_BACKOFF"))
	bind("jobs.retry.max_backoff", l.prefixedEnv("JOBS_RETRY_MAX_BACKOFF"))
	bind("jobs.retry.attempt_timeout", l.prefixedEnv("JOBS_RETRY_ATTEMPT_TIMEOUT"))
	bind("jobs.dlq.enabled", l.prefixedEnv("JOBS_DLQ_ENABLED"))
	bind("jobs.dlq.queue_suffix", l.prefixedEnv("JOBS_DLQ_QUEUE_SUFFIX"))
	bind("jobs.redis.url", l.prefixedEnv("JOBS_REDIS_URL"))
	bind("jobs.redis.prefix", l.prefixedEnv("JOBS_REDIS_PREFIX"))
	bind("jobs.redis.operation_timeout", l.prefixedEnv("JOBS_REDIS_OPERATION_TIMEOUT"))

	// Scheduler
	bind("scheduler.enabled", l.prefixedEnv("SCHEDULER_ENABLED"))
	bind("scheduler.timezone", l.prefixedEnv("SCHEDULER_TIMEZONE"))
	bind("scheduler.lock_provider", l.prefixedEnv("SCHEDULER_LOCK_PROVIDER"))
	bind("scheduler.lock_ttl", l.prefixedEnv("SCHEDULER_LOCK_TTL"))
	bind("scheduler.dispatch_timeout", l.prefixedEnv("SCHEDULER_DISPATCH_TIMEOUT"))
	bind("scheduler.redis.url", l.prefixedEnv("SCHEDULER_REDIS_URL"))
	bind("scheduler.redis.prefix", l.prefixedEnv("SCHEDULER_REDIS_PREFIX"))
	bind("scheduler.redis.operation_timeout", l.prefixedEnv("SCHEDULER_REDIS_OPERATION_TIMEOUT"))
	bind("scheduler.postgres.url", l.prefixedEnv("SCHEDULER_POSTGRES_URL"))
	bind("scheduler.postgres.table", l.prefixedEnv("SCHEDULER_POSTGRES_TABLE"))
	bind("scheduler.postgres.operation_timeout", l.prefixedEnv("SCHEDULER_POSTGRES_OPERATION_TIMEOUT"))

	// Validation
	bind("validation.kafka.enabled", l.prefixedEnv("VALIDATION_KAFKA_ENABLED"))
	bind("validation.kafka.mode", l.prefixedEnv("VALIDATION_KAFKA_MODE"))
	bind("validation.kafka.descriptor_path", l.prefixedEnv("VALIDATION_KAFKA_DESCRIPTOR_PATH"))
	bind("validation.kafka.default_policy", l.prefixedEnv("VALIDATION_KAFKA_DEFAULT_POLICY"))

	// Rate limit
	bind("rate_limit.enabled", l.prefixedEnv("RATE_LIMIT_ENABLED"))
	bind("rate_limit.type", l.prefixedEnv("RATE_LIMIT_TYPE"))
	bind("rate_limit.requests_per_second", l.prefixedEnv("RATE_LIMIT_REQUESTS_PER_SECOND"))
	bind("rate_limit.burst", l.prefixedEnv("RATE_LIMIT_BURST"))
	bind("rate_limit.window", l.prefixedEnv("RATE_LIMIT_WINDOW"))
	bind("rate_limit.redis.url", l.prefixedEnv("RATE_LIMIT_REDIS_URL"))
	bind("rate_limit.redis.max_conns", l.prefixedEnv("RATE_LIMIT_REDIS_MAX_CONNS"))
	bind("rate_limit.redis.operation_timeout", l.prefixedEnv("RATE_LIMIT_REDIS_OPERATION_TIMEOUT"))
	bind("rate_limit.redis.prefix", l.prefixedEnv("RATE_LIMIT_REDIS_PREFIX"))

	// Observability
	bind("observability.log_level", l.prefixedEnv("OBSERVABILITY_LOG_LEVEL"))
	bind("observability.log_format", l.prefixedEnv("OBSERVABILITY_LOG_FORMAT"))
	bind("observability.service_name", l.prefixedEnv("OBSERVABILITY_SERVICE_NAME"))
	bind("observability.tracing_enabled", l.prefixedEnv("OBSERVABILITY_TRACING_ENABLED"))
	bind("observability.tracing_sample_rate", l.prefixedEnv("OBSERVABILITY_TRACING_SAMPLE_RATE"))
	bind("observability.tracing_endpoint", l.prefixedEnv("OBSERVABILITY_TRACING_ENDPOINT"))
	bind("observability.async_logging.enabled", l.prefixedEnv("OBSERVABILITY_ASYNC_LOGGING_ENABLED"))
	bind("observability.async_logging.queue_size", l.prefixedEnv("OBSERVABILITY_ASYNC_LOGGING_QUEUE_SIZE"))
	bind("observability.async_logging.worker_count", l.prefixedEnv("OBSERVABILITY_ASYNC_LOGGING_WORKER_COUNT"))
	bind("observability.async_logging.drop_when_full", l.prefixedEnv("OBSERVABILITY_ASYNC_LOGGING_DROP_WHEN_FULL"))
	bind("observability.request_logging.enabled", l.prefixedEnv("OBSERVABILITY_REQUEST_LOGGING_ENABLED"))
	bind("observability.request_logging.log_start", l.prefixedEnv("OBSERVABILITY_REQUEST_LOGGING_LOG_START"))
	bind("observability.request_logging.output", l.prefixedEnv("OBSERVABILITY_REQUEST_LOGGING_OUTPUT"))
	bind("observability.request_logging.fields", l.prefixedEnv("OBSERVABILITY_REQUEST_LOGGING_FIELDS"))
	bind("observability.request_tracing.enabled", l.prefixedEnv("OBSERVABILITY_REQUEST_TRACING_ENABLED"))
	bind("observability.request_timeout.enabled", l.prefixedEnv("OBSERVABILITY_REQUEST_TIMEOUT_ENABLED"))
	bind("observability.request_timeout.default", l.prefixedEnv("OBSERVABILITY_REQUEST_TIMEOUT_DEFAULT"))

	// Swagger
	bind("swagger.enabled", l.prefixedEnv("SWAGGER_ENABLED"))
	bind("swagger.spec_path", l.prefixedEnv("SWAGGER_SPEC_PATH"))

	return bindErr
}

// bindLegacyEnvVars maps legacy env vars to current abbreviated names when abbreviated vars are absent.
func (l *ViperLoader) bindLegacyEnvVars() {
	aliases := []struct {
		abbrevSuffix string
		legacySuffix string
	}{
		{"MGMT_PORT", "MANAGEMENT_PORT"},
		{"MGMT_READ_TIMEOUT", "MANAGEMENT_READ_TIMEOUT"},
		{"MGMT_WRITE_TIMEOUT", "MANAGEMENT_WRITE_TIMEOUT"},
		{"MGMT_AUTH_ENABLED", "MANAGEMENT_AUTH_ENABLED"},
		{"MGMT_MTLS_ENABLED", "MANAGEMENT_MTLS_ENABLED"},
		{"DB_TYPE", "DATABASE_TYPE"},
		{"DB_URL", "DATABASE_URL"},
		{"DB_MAX_OPEN_CONNS", "DATABASE_MAX_OPEN_CONNS"},
		{"DB_MAX_IDLE_CONNS", "DATABASE_MAX_IDLE_CONNS"},
		{"DB_CONN_MAX_LIFETIME", "DATABASE_CONN_MAX_LIFETIME"},
		{"DB_QUERY_TIMEOUT", "DATABASE_QUERY_TIMEOUT"},
		{"DB_DATABASE_NAME", "DATABASE_DATABASE_NAME"},
		{"DB_CONNECT_TIMEOUT", "DATABASE_CONNECT_TIMEOUT"},
		{"DB_REGION", "DATABASE_REGION"},
		{"DB_ENDPOINT", "DATABASE_ENDPOINT"},
		{"DB_ACCESS_KEY_ID", "DATABASE_ACCESS_KEY_ID"},
		{"DB_SECRET_ACCESS_KEY", "DATABASE_SECRET_ACCESS_KEY"},
		{"DB_SESSION_TOKEN", "DATABASE_SESSION_TOKEN"},
	}

	for _, alias := range aliases {
		abbrevEnv := l.prefixedEnv(alias.abbrevSuffix)
		if _, hasAbbrev := os.LookupEnv(abbrevEnv); hasAbbrev {
			continue
		}
		if legacyValue, hasLegacy := os.LookupEnv(l.prefixedEnv(alias.legacySuffix)); hasLegacy {
			_ = os.Setenv(abbrevEnv, legacyValue)
		}
	}
}

func (l *ViperLoader) prefixedEnv(suffix string) string {
	prefix := strings.TrimSpace(l.envPrefix)
	if prefix == "" {
		prefix = "APP"
	}
	return fmt.Sprintf("%s_%s", strings.ToUpper(prefix), suffix)
}

func (l *ViperLoader) defaultServiceName(fallback string) string {
	if l != nil {
		if configured := strings.TrimSpace(l.serviceNameDefault); configured != "" {
			return configured
		}
	}
	return strings.TrimSpace(fallback)
}

// setDefaults sets default values in Viper from the default config
func (l *ViperLoader) setDefaults(v *viper.Viper, cfg *Config) {
	// Router defaults
	v.SetDefault("router_type", cfg.RouterType)
	v.SetDefault("service.name", l.defaultServiceName(cfg.Service.Name))
	v.SetDefault("service.environment", cfg.Service.Environment)

	// HTTP defaults
	v.SetDefault("http.port", cfg.HTTP.Port)
	v.SetDefault("http.read_timeout", cfg.HTTP.ReadTimeout)
	v.SetDefault("http.write_timeout", cfg.HTTP.WriteTimeout)
	v.SetDefault("http.idle_timeout", cfg.HTTP.IdleTimeout)
	v.SetDefault("http.max_request_size", cfg.HTTP.MaxRequestSize)

	// Management defaults
	v.SetDefault("management.enabled", cfg.Management.Enabled)
	v.SetDefault("management.port", cfg.Management.Port)
	v.SetDefault("management.read_timeout", cfg.Management.ReadTimeout)
	v.SetDefault("management.write_timeout", cfg.Management.WriteTimeout)
	v.SetDefault("management.auth_enabled", cfg.Management.AuthEnabled)
	v.SetDefault("management.mtls_enabled", cfg.Management.MTLSEnabled)
	v.SetDefault("management.tls_cert_file", cfg.Management.TLSCertFile)
	v.SetDefault("management.tls_key_file", cfg.Management.TLSKeyFile)
	v.SetDefault("management.tls_ca_file", cfg.Management.TLSCAFile)

	// CORS defaults
	v.SetDefault("cors.enabled", cfg.CORS.Enabled)
	v.SetDefault("cors.allow_all_origins", cfg.CORS.AllowAllOrigins)
	v.SetDefault("cors.allow_origins", cfg.CORS.AllowOrigins)
	v.SetDefault("cors.allow_methods", cfg.CORS.AllowMethods)
	v.SetDefault("cors.allow_private_network", cfg.CORS.AllowPrivateNetwork)
	v.SetDefault("cors.allow_headers", cfg.CORS.AllowHeaders)
	v.SetDefault("cors.expose_headers", cfg.CORS.ExposeHeaders)
	v.SetDefault("cors.allow_credentials", cfg.CORS.AllowCredentials)
	v.SetDefault("cors.max_age", cfg.CORS.MaxAge)
	v.SetDefault("cors.allow_wildcard", cfg.CORS.AllowWildcard)
	v.SetDefault("cors.allow_browser_extensions", cfg.CORS.AllowBrowserExtensions)
	v.SetDefault("cors.custom_schemas", cfg.CORS.CustomSchemas)
	v.SetDefault("cors.allow_websockets", cfg.CORS.AllowWebSockets)
	v.SetDefault("cors.allow_files", cfg.CORS.AllowFiles)
	v.SetDefault("cors.options_response_status_code", cfg.CORS.OptionsResponseStatusCode)

	// Security headers defaults
	v.SetDefault("security_headers.enabled", cfg.SecurityHeaders.Enabled)
	v.SetDefault("security_headers.is_development", cfg.SecurityHeaders.IsDevelopment)
	v.SetDefault("security_headers.allowed_hosts", cfg.SecurityHeaders.AllowedHosts)
	v.SetDefault("security_headers.ssl_redirect", cfg.SecurityHeaders.SSLRedirect)
	v.SetDefault("security_headers.ssl_temporary_redirect", cfg.SecurityHeaders.SSLTemporaryRedirect)
	v.SetDefault("security_headers.ssl_host", cfg.SecurityHeaders.SSLHost)
	v.SetDefault("security_headers.ssl_proxy_headers", cfg.SecurityHeaders.SSLProxyHeaders)
	v.SetDefault("security_headers.dont_redirect_ipv4_hostnames", cfg.SecurityHeaders.DontRedirectIPV4Hostnames)
	v.SetDefault("security_headers.sts_seconds", cfg.SecurityHeaders.STSSeconds)
	v.SetDefault("security_headers.sts_include_subdomains", cfg.SecurityHeaders.STSIncludeSubdomains)
	v.SetDefault("security_headers.sts_preload", cfg.SecurityHeaders.STSPreload)
	v.SetDefault("security_headers.custom_frame_options", cfg.SecurityHeaders.CustomFrameOptions)
	v.SetDefault("security_headers.content_type_nosniff", cfg.SecurityHeaders.ContentTypeNosniff)
	v.SetDefault("security_headers.content_security_policy", cfg.SecurityHeaders.ContentSecurityPolicy)
	v.SetDefault("security_headers.referrer_policy", cfg.SecurityHeaders.ReferrerPolicy)
	v.SetDefault("security_headers.permissions_policy", cfg.SecurityHeaders.PermissionsPolicy)
	v.SetDefault("security_headers.ie_no_open", cfg.SecurityHeaders.IENoOpen)
	v.SetDefault("security_headers.x_dns_prefetch_control", cfg.SecurityHeaders.XDNSPrefetchControl)
	v.SetDefault("security_headers.cross_origin_opener_policy", cfg.SecurityHeaders.CrossOriginOpenerPolicy)
	v.SetDefault("security_headers.cross_origin_resource_policy", cfg.SecurityHeaders.CrossOriginResourcePolicy)
	v.SetDefault("security_headers.cross_origin_embedder_policy", cfg.SecurityHeaders.CrossOriginEmbedderPolicy)
	v.SetDefault("security_headers.custom_headers", cfg.SecurityHeaders.CustomHeaders)

	// Security defaults
	v.SetDefault("security.http_signature.enabled", cfg.Security.HTTPSignature.Enabled)
	v.SetDefault("security.http_signature.key_id_header", cfg.Security.HTTPSignature.KeyIDHeader)
	v.SetDefault("security.http_signature.timestamp_header", cfg.Security.HTTPSignature.TimestampHeader)
	v.SetDefault("security.http_signature.nonce_header", cfg.Security.HTTPSignature.NonceHeader)
	v.SetDefault("security.http_signature.signature_header", cfg.Security.HTTPSignature.SignatureHeader)
	v.SetDefault("security.http_signature.max_clock_skew", cfg.Security.HTTPSignature.MaxClockSkew)
	v.SetDefault("security.http_signature.nonce_ttl", cfg.Security.HTTPSignature.NonceTTL)
	v.SetDefault("security.http_signature.require_nonce", cfg.Security.HTTPSignature.RequireNonce)
	v.SetDefault("security.http_signature.excluded_path_prefixes", cfg.Security.HTTPSignature.ExcludedPathPrefixes)
	v.SetDefault("security.http_signature.static_keys", cfg.Security.HTTPSignature.StaticKeys)

	// I18n defaults
	v.SetDefault("i18n.enabled", cfg.I18n.Enabled)
	v.SetDefault("i18n.default_locale", cfg.I18n.DefaultLocale)
	v.SetDefault("i18n.supported_locales", cfg.I18n.SupportedLocales)
	v.SetDefault("i18n.query_param", cfg.I18n.QueryParam)
	v.SetDefault("i18n.header_name", cfg.I18n.HeaderName)
	v.SetDefault("i18n.fallback_mode", cfg.I18n.FallbackMode)
	v.SetDefault("i18n.catalog_path", cfg.I18n.CatalogPath)

	// Session defaults
	v.SetDefault("session.enabled", cfg.Session.Enabled)
	v.SetDefault("session.store", cfg.Session.Store)
	v.SetDefault("session.ttl", cfg.Session.TTL)
	v.SetDefault("session.idle_timeout", cfg.Session.IdleTimeout)
	v.SetDefault("session.cookie_name", cfg.Session.CookieName)
	v.SetDefault("session.cookie_path", cfg.Session.CookiePath)
	v.SetDefault("session.cookie_domain", cfg.Session.CookieDomain)
	v.SetDefault("session.cookie_secure", cfg.Session.CookieSecure)
	v.SetDefault("session.cookie_http_only", cfg.Session.CookieHTTPOnly)
	v.SetDefault("session.cookie_same_site", cfg.Session.CookieSameSite)
	v.SetDefault("session.auto_create", cfg.Session.AutoCreate)
	v.SetDefault("session.redis.max_conns", cfg.Session.Redis.MaxConns)
	v.SetDefault("session.redis.operation_timeout", cfg.Session.Redis.OperationTimeout)
	v.SetDefault("session.redis.prefix", cfg.Session.Redis.Prefix)
	v.SetDefault("session.memcached.addresses", cfg.Session.Memcached.Addresses)
	v.SetDefault("session.memcached.timeout", cfg.Session.Memcached.Timeout)
	v.SetDefault("session.memcached.prefix", cfg.Session.Memcached.Prefix)

	// CSRF defaults
	v.SetDefault("csrf.enabled", cfg.CSRF.Enabled)
	v.SetDefault("csrf.header_name", cfg.CSRF.HeaderName)
	v.SetDefault("csrf.cookie_name", cfg.CSRF.CookieName)
	v.SetDefault("csrf.cookie_path", cfg.CSRF.CookiePath)
	v.SetDefault("csrf.cookie_domain", cfg.CSRF.CookieDomain)
	v.SetDefault("csrf.cookie_secure", cfg.CSRF.CookieSecure)
	v.SetDefault("csrf.cookie_same_site", cfg.CSRF.CookieSameSite)
	v.SetDefault("csrf.cookie_ttl", cfg.CSRF.CookieTTL)
	v.SetDefault("csrf.exempt_methods", cfg.CSRF.ExemptMethods)
	v.SetDefault("csrf.exempt_paths", cfg.CSRF.ExemptPaths)

	// SSE defaults
	v.SetDefault("sse.enabled", cfg.SSE.Enabled)
	v.SetDefault("sse.endpoint", cfg.SSE.Endpoint)
	v.SetDefault("sse.store", cfg.SSE.Store)
	v.SetDefault("sse.bus", cfg.SSE.Bus)
	v.SetDefault("sse.replay_limit", cfg.SSE.ReplayLimit)
	v.SetDefault("sse.client_buffer", cfg.SSE.ClientBuffer)
	v.SetDefault("sse.max_connections", cfg.SSE.MaxConnections)
	v.SetDefault("sse.heartbeat_interval", cfg.SSE.HeartbeatInterval)
	v.SetDefault("sse.default_retry_ms", cfg.SSE.DefaultRetryMS)
	v.SetDefault("sse.drop_on_backpressure", cfg.SSE.DropOnBackpressure)
	v.SetDefault("sse.channel_query_param", cfg.SSE.ChannelQueryParam)
	v.SetDefault("sse.tenant_query_param", cfg.SSE.TenantQueryParam)
	v.SetDefault("sse.subject_query_param", cfg.SSE.SubjectQueryParam)
	v.SetDefault("sse.last_event_id_query_param", cfg.SSE.LastEventIDQueryParam)
	v.SetDefault("sse.redis.max_conns", cfg.SSE.Redis.MaxConns)
	v.SetDefault("sse.redis.operation_timeout", cfg.SSE.Redis.OperationTimeout)
	v.SetDefault("sse.redis.history_prefix", cfg.SSE.Redis.HistoryPrefix)
	v.SetDefault("sse.redis.pubsub_prefix", cfg.SSE.Redis.PubSubPrefix)
	v.SetDefault("sse.eventbus.topic_prefix", cfg.SSE.EventBus.TopicPrefix)
	v.SetDefault("sse.eventbus.operation_timeout", cfg.SSE.EventBus.OperationTimeout)

	// Email defaults
	v.SetDefault("email.enabled", cfg.Email.Enabled)
	v.SetDefault("email.provider", cfg.Email.Provider)
	v.SetDefault("email.smtp.port", cfg.Email.SMTP.Port)
	v.SetDefault("email.smtp.enable_tls", cfg.Email.SMTP.EnableTLS)
	v.SetDefault("email.smtp.insecure_skip_verify", cfg.Email.SMTP.InsecureSkipVerify)
	v.SetDefault("email.smtp.operation_timeout", cfg.Email.SMTP.OperationTimeout)
	v.SetDefault("email.ses.operation_timeout", cfg.Email.SES.OperationTimeout)
	v.SetDefault("email.sendgrid.base_url", cfg.Email.SendGrid.BaseURL)
	v.SetDefault("email.sendgrid.operation_timeout", cfg.Email.SendGrid.OperationTimeout)
	v.SetDefault("email.mailgun.base_url", cfg.Email.Mailgun.BaseURL)
	v.SetDefault("email.mailgun.operation_timeout", cfg.Email.Mailgun.OperationTimeout)
	v.SetDefault("email.mailchimp.base_url", cfg.Email.Mailchimp.BaseURL)
	v.SetDefault("email.mailchimp.operation_timeout", cfg.Email.Mailchimp.OperationTimeout)
	v.SetDefault("email.mailersend.base_url", cfg.Email.MailerSend.BaseURL)
	v.SetDefault("email.mailersend.operation_timeout", cfg.Email.MailerSend.OperationTimeout)
	v.SetDefault("email.postmark.base_url", cfg.Email.Postmark.BaseURL)
	v.SetDefault("email.postmark.operation_timeout", cfg.Email.Postmark.OperationTimeout)
	v.SetDefault("email.mailtrap.base_url", cfg.Email.Mailtrap.BaseURL)
	v.SetDefault("email.mailtrap.operation_timeout", cfg.Email.Mailtrap.OperationTimeout)
	v.SetDefault("email.smtp2go.base_url", cfg.Email.SMTP2GO.BaseURL)
	v.SetDefault("email.smtp2go.operation_timeout", cfg.Email.SMTP2GO.OperationTimeout)
	v.SetDefault("email.sendpulse.base_url", cfg.Email.SendPulse.BaseURL)
	v.SetDefault("email.sendpulse.operation_timeout", cfg.Email.SendPulse.OperationTimeout)
	v.SetDefault("email.brevo.base_url", cfg.Email.Brevo.BaseURL)
	v.SetDefault("email.brevo.operation_timeout", cfg.Email.Brevo.OperationTimeout)
	v.SetDefault("email.mailjet.base_url", cfg.Email.Mailjet.BaseURL)
	v.SetDefault("email.mailjet.operation_timeout", cfg.Email.Mailjet.OperationTimeout)

	// Auth defaults
	v.SetDefault("auth.enabled", cfg.Auth.Enabled)
	v.SetDefault("auth.jwks_cache_ttl", cfg.Auth.JWKSCacheTTL)
	for claimName, aliases := range cfg.Auth.Claims.Mappings {
		v.SetDefault(fmt.Sprintf("auth.claims.mappings.%s", claimName), aliases)
	}

	// Database defaults
	v.SetDefault("database.max_open_conns", cfg.Database.MaxOpenConns)
	v.SetDefault("database.max_idle_conns", cfg.Database.MaxIdleConns)
	v.SetDefault("database.conn_max_lifetime", cfg.Database.ConnMaxLifetime)
	v.SetDefault("database.conn_max_idle_time", cfg.Database.ConnMaxIdleTime)
	v.SetDefault("database.query_timeout", cfg.Database.QueryTimeout)
	v.SetDefault("database.connect_timeout", cfg.Database.ConnectTimeout)

	// Cache defaults
	v.SetDefault("cache.max_conns", cfg.Cache.MaxConns)
	v.SetDefault("cache.operation_timeout", cfg.Cache.OperationTimeout)

	// Object storage defaults
	v.SetDefault("object_storage.enabled", cfg.ObjectStorage.Enabled)
	v.SetDefault("object_storage.type", cfg.ObjectStorage.Type)
	v.SetDefault("object_storage.s3.bucket", cfg.ObjectStorage.S3.Bucket)
	v.SetDefault("object_storage.s3.region", cfg.ObjectStorage.S3.Region)
	v.SetDefault("object_storage.s3.endpoint", cfg.ObjectStorage.S3.Endpoint)
	v.SetDefault("object_storage.s3.access_key_id", cfg.ObjectStorage.S3.AccessKeyID)
	v.SetDefault("object_storage.s3.secret_access_key", cfg.ObjectStorage.S3.SecretAccessKey)
	v.SetDefault("object_storage.s3.session_token", cfg.ObjectStorage.S3.SessionToken)
	v.SetDefault("object_storage.s3.use_path_style", cfg.ObjectStorage.S3.UsePathStyle)
	v.SetDefault("object_storage.s3.operation_timeout", cfg.ObjectStorage.S3.OperationTimeout)
	v.SetDefault("object_storage.s3.presign_expiry", cfg.ObjectStorage.S3.PresignExpiry)

	// Search defaults
	v.SetDefault("search.driver", cfg.Search.Driver)
	v.SetDefault("search.max_conns", cfg.Search.MaxConns)
	v.SetDefault("search.operation_timeout", cfg.Search.OperationTimeout)
	v.SetDefault("search.aws_service", cfg.Search.AWSService)

	// EventBus defaults
	v.SetDefault("eventbus.serializer", cfg.EventBus.Serializer)
	v.SetDefault("eventbus.operation_timeout", cfg.EventBus.OperationTimeout)
	v.SetDefault("eventbus.group_id", cfg.EventBus.GroupID)
	v.SetDefault("eventbus.exchange", cfg.EventBus.Exchange)
	v.SetDefault("eventbus.exchange_type", cfg.EventBus.ExchangeType)
	v.SetDefault("eventbus.wait_time_seconds", cfg.EventBus.WaitTimeSeconds)
	v.SetDefault("eventbus.max_messages", cfg.EventBus.MaxMessages)

	// Jobs defaults
	v.SetDefault("jobs.backend", cfg.Jobs.Backend)
	v.SetDefault("jobs.default_queue", cfg.Jobs.DefaultQueue)
	v.SetDefault("jobs.worker.concurrency", cfg.Jobs.Worker.Concurrency)
	v.SetDefault("jobs.worker.lease_ttl", cfg.Jobs.Worker.LeaseTTL)
	v.SetDefault("jobs.worker.reserve_timeout", cfg.Jobs.Worker.ReserveTimeout)
	v.SetDefault("jobs.worker.stop_timeout", cfg.Jobs.Worker.StopTimeout)
	v.SetDefault("jobs.retry.max_attempts", cfg.Jobs.Retry.MaxAttempts)
	v.SetDefault("jobs.retry.initial_backoff", cfg.Jobs.Retry.InitialBackoff)
	v.SetDefault("jobs.retry.max_backoff", cfg.Jobs.Retry.MaxBackoff)
	v.SetDefault("jobs.retry.attempt_timeout", cfg.Jobs.Retry.AttemptTimeout)
	v.SetDefault("jobs.dlq.enabled", cfg.Jobs.DLQ.Enabled)
	v.SetDefault("jobs.dlq.queue_suffix", cfg.Jobs.DLQ.QueueSuffix)
	v.SetDefault("jobs.redis.prefix", cfg.Jobs.Redis.Prefix)
	v.SetDefault("jobs.redis.operation_timeout", cfg.Jobs.Redis.OperationTimeout)

	// Scheduler defaults
	v.SetDefault("scheduler.enabled", cfg.Scheduler.Enabled)
	v.SetDefault("scheduler.timezone", cfg.Scheduler.Timezone)
	v.SetDefault("scheduler.lock_provider", cfg.Scheduler.LockProvider)
	v.SetDefault("scheduler.lock_ttl", cfg.Scheduler.LockTTL)
	v.SetDefault("scheduler.dispatch_timeout", cfg.Scheduler.DispatchTimeout)
	v.SetDefault("scheduler.redis.prefix", cfg.Scheduler.Redis.Prefix)
	v.SetDefault("scheduler.redis.operation_timeout", cfg.Scheduler.Redis.OperationTimeout)
	v.SetDefault("scheduler.postgres.table", cfg.Scheduler.Postgres.Table)
	v.SetDefault("scheduler.postgres.operation_timeout", cfg.Scheduler.Postgres.OperationTimeout)

	// Validation defaults
	v.SetDefault("validation.kafka.enabled", cfg.Validation.Kafka.Enabled)
	v.SetDefault("validation.kafka.mode", cfg.Validation.Kafka.Mode)
	v.SetDefault("validation.kafka.descriptor_path", cfg.Validation.Kafka.DescriptorPath)
	v.SetDefault("validation.kafka.default_policy", cfg.Validation.Kafka.DefaultPolicy)
	v.SetDefault("validation.kafka.subjects", cfg.Validation.Kafka.Subjects)

	// Observability defaults
	v.SetDefault("observability.log_level", cfg.Observability.LogLevel)
	v.SetDefault("observability.log_format", cfg.Observability.LogFormat)
	v.SetDefault("observability.service_name", cfg.Observability.ServiceName)
	v.SetDefault("observability.tracing_enabled", cfg.Observability.TracingEnabled)
	v.SetDefault("observability.tracing_sample_rate", cfg.Observability.TracingSampleRate)
	v.SetDefault("observability.async_logging.enabled", cfg.Observability.AsyncLogging.Enabled)
	v.SetDefault("observability.async_logging.queue_size", cfg.Observability.AsyncLogging.QueueSize)
	v.SetDefault("observability.async_logging.worker_count", cfg.Observability.AsyncLogging.WorkerCount)
	v.SetDefault("observability.async_logging.drop_when_full", cfg.Observability.AsyncLogging.DropWhenFull)
	v.SetDefault("observability.request_logging.enabled", cfg.Observability.RequestLogging.Enabled)
	v.SetDefault("observability.request_logging.log_start", cfg.Observability.RequestLogging.LogStart)
	v.SetDefault("observability.request_logging.output", cfg.Observability.RequestLogging.Output)
	v.SetDefault("observability.request_logging.fields", cfg.Observability.RequestLogging.Fields)
	v.SetDefault("observability.request_logging.excluded_path_prefixes", cfg.Observability.RequestLogging.ExcludedPathPrefixes)
	v.SetDefault("observability.request_logging.path_policies", cfg.Observability.RequestLogging.PathPolicies)
	v.SetDefault("observability.request_tracing.enabled", cfg.Observability.RequestTracing.Enabled)
	v.SetDefault("observability.request_tracing.excluded_path_prefixes", cfg.Observability.RequestTracing.ExcludedPathPrefixes)
	v.SetDefault("observability.request_tracing.path_policies", cfg.Observability.RequestTracing.PathPolicies)
	v.SetDefault("observability.request_timeout.enabled", cfg.Observability.RequestTimeout.Enabled)
	v.SetDefault("observability.request_timeout.default", cfg.Observability.RequestTimeout.Default)
	v.SetDefault("observability.request_timeout.excluded_path_prefixes", cfg.Observability.RequestTimeout.ExcludedPathPrefixes)
	v.SetDefault("observability.request_timeout.path_policies", cfg.Observability.RequestTimeout.PathPolicies)

	// Rate limit defaults
	v.SetDefault("rate_limit.enabled", cfg.RateLimit.Enabled)
	v.SetDefault("rate_limit.type", cfg.RateLimit.Type)
	v.SetDefault("rate_limit.requests_per_second", cfg.RateLimit.RequestsPerSecond)
	v.SetDefault("rate_limit.burst", cfg.RateLimit.Burst)
	v.SetDefault("rate_limit.window", cfg.RateLimit.Window)
	v.SetDefault("rate_limit.redis.max_conns", cfg.RateLimit.Redis.MaxConns)
	v.SetDefault("rate_limit.redis.operation_timeout", cfg.RateLimit.Redis.OperationTimeout)
	v.SetDefault("rate_limit.redis.prefix", cfg.RateLimit.Redis.Prefix)

	// Swagger defaults
	v.SetDefault("swagger.enabled", cfg.Swagger.Enabled)
	v.SetDefault("swagger.spec_path", cfg.Swagger.SpecPath)
}

// Validate validates the configuration and returns detailed errors
func (l *ViperLoader) Validate(cfg *Config) error {
	var errs []error

	// Normalize CORS AllowOrigins (remove empty strings)
	cfg.CORS.AllowOrigins = normalizeStringSlice(cfg.CORS.AllowOrigins)
	cfg.Observability.RequestLogging.Fields = normalizeStringSlice(cfg.Observability.RequestLogging.Fields)

	// Validate Auth configuration
	validRouterTypes := []string{"nethttp", "gin", "gorilla"}
	if !contains(validRouterTypes, strings.ToLower(cfg.RouterType)) {
		errs = append(errs, fmt.Errorf("invalid router_type: %s (must be one of: %v)", cfg.RouterType, validRouterTypes))
	}

	// Validate Auth configuration
	if cfg.Auth.Enabled {
		if cfg.Auth.Issuer == "" {
			errs = append(errs, errors.New("auth.issuer is required when auth is enabled"))
		}
		if cfg.Auth.JWKSUrl == "" {
			errs = append(errs, errors.New("auth.jwks_url is required when auth is enabled"))
		}
		if cfg.Auth.Audience == "" {
			errs = append(errs, errors.New("auth.audience is required when auth is enabled"))
		}
		if len(cfg.Auth.Claims.Rules) == 0 {
			errs = append(errs, errors.New("auth.claims.rules must contain at least one rule when auth is enabled"))
		}
		validSources := []string{"route", "header", "query"}
		validOperators := []string{"required", "equals", "one_of", "regex"}
		for index, rule := range cfg.Auth.Claims.Rules {
			if strings.TrimSpace(rule.Claim) == "" {
				errs = append(errs, fmt.Errorf("auth.claims.rules[%d].claim is required", index))
			}
			operator := strings.ToLower(strings.TrimSpace(rule.Operator))
			if operator == "" {
				operator = "required"
			}
			if !contains(validOperators, operator) {
				errs = append(errs, fmt.Errorf("auth.claims.rules[%d].operator must be one of %v", index, validOperators))
				continue
			}
			switch operator {
			case "equals":
				source := strings.ToLower(strings.TrimSpace(rule.Source))
				if !contains(validSources, source) {
					errs = append(errs, fmt.Errorf("auth.claims.rules[%d].source must be one of %v when operator=equals", index, validSources))
				}
				if strings.TrimSpace(rule.Key) == "" {
					errs = append(errs, fmt.Errorf("auth.claims.rules[%d].key is required when operator=equals", index))
				}
			case "one_of":
				if len(rule.Values) == 0 {
					errs = append(errs, fmt.Errorf("auth.claims.rules[%d].values must contain at least one value when operator=one_of", index))
				}
			case "regex":
				if len(rule.Values) != 1 || strings.TrimSpace(rule.Values[0]) == "" {
					errs = append(errs, fmt.Errorf("auth.claims.rules[%d].values must contain exactly one regex pattern when operator=regex", index))
				}
			}
		}
		for claimName, aliases := range cfg.Auth.Claims.Mappings {
			if strings.TrimSpace(claimName) == "" {
				errs = append(errs, errors.New("auth.claims.mappings contains an empty claim name"))
				continue
			}
			for aliasIndex, alias := range aliases {
				if strings.TrimSpace(alias) == "" {
					errs = append(errs, fmt.Errorf("auth.claims.mappings.%s[%d] cannot be empty", claimName, aliasIndex))
				}
			}
		}
	}
	if cfg.Observability.AsyncLogging.Enabled {
		if cfg.Observability.AsyncLogging.QueueSize <= 0 {
			errs = append(errs, errors.New("observability.async_logging.queue_size must be greater than 0 when async logging is enabled"))
		}
		if cfg.Observability.AsyncLogging.WorkerCount <= 0 {
			errs = append(errs, errors.New("observability.async_logging.worker_count must be greater than 0 when async logging is enabled"))
		}
	}
	validRequestLoggingModes := []string{"off", "minimal", "full"}
	validRequestLoggingOutputs := []string{"logger", "stdout", "stderr"}
	validRequestLoggingFields := []string{
		"request_id", "method", "path", "status", "duration_ms", "error",
		"remote_addr", "remote_port", "request_method", "request_uri", "uri",
		"args", "query_string", "request_time", "time_local", "host",
		"server_protocol", "scheme", "http_referer", "http_user_agent",
		"x_forwarded_for", "remote_user", "request_length",
	}
	requestLoggingOutput := strings.ToLower(strings.TrimSpace(cfg.Observability.RequestLogging.Output))
	if requestLoggingOutput == "" {
		requestLoggingOutput = "logger"
	}
	if !contains(validRequestLoggingOutputs, requestLoggingOutput) {
		errs = append(errs, fmt.Errorf("observability.request_logging.output must be one of %v", validRequestLoggingOutputs))
	}
	for index, field := range cfg.Observability.RequestLogging.Fields {
		normalizedField := strings.ToLower(strings.TrimSpace(field))
		if !contains(validRequestLoggingFields, normalizedField) {
			errs = append(errs, fmt.Errorf("observability.request_logging.fields[%d] must be one of %v", index, validRequestLoggingFields))
		}
	}
	for index, policy := range cfg.Observability.RequestLogging.PathPolicies {
		if strings.TrimSpace(policy.PathPrefix) == "" {
			errs = append(errs, fmt.Errorf("observability.request_logging.path_policies[%d].path_prefix is required", index))
		}
		if !contains(validRequestLoggingModes, strings.ToLower(strings.TrimSpace(policy.Mode))) {
			errs = append(errs, fmt.Errorf("observability.request_logging.path_policies[%d].mode must be one of %v", index, validRequestLoggingModes))
		}
	}
	validRequestTracingModes := []string{"off", "minimal", "full"}
	for index, policy := range cfg.Observability.RequestTracing.PathPolicies {
		if strings.TrimSpace(policy.PathPrefix) == "" {
			errs = append(errs, fmt.Errorf("observability.request_tracing.path_policies[%d].path_prefix is required", index))
		}
		if !contains(validRequestTracingModes, strings.ToLower(strings.TrimSpace(policy.Mode))) {
			errs = append(errs, fmt.Errorf("observability.request_tracing.path_policies[%d].mode must be one of %v", index, validRequestTracingModes))
		}
	}
	if cfg.Observability.RequestTimeout.Enabled && cfg.Observability.RequestTimeout.Default <= 0 {
		errs = append(errs, errors.New("observability.request_timeout.default must be greater than zero when request timeout is enabled"))
	}
	validRequestTimeoutModes := []string{"off", "on"}
	for index, policy := range cfg.Observability.RequestTimeout.PathPolicies {
		if strings.TrimSpace(policy.PathPrefix) == "" {
			errs = append(errs, fmt.Errorf("observability.request_timeout.path_policies[%d].path_prefix is required", index))
		}
		if !contains(validRequestTimeoutModes, strings.ToLower(strings.TrimSpace(policy.Mode))) {
			errs = append(errs, fmt.Errorf("observability.request_timeout.path_policies[%d].mode must be one of %v", index, validRequestTimeoutModes))
		}
	}
	if cfg.Management.AuthEnabled && !cfg.Auth.Enabled {
		errs = append(errs, errors.New("auth.enabled must be true when management.auth_enabled is true"))
	}
	if cfg.Management.MTLSEnabled {
		if cfg.Management.TLSCertFile == "" {
			errs = append(errs, errors.New("management.tls_cert_file is required when management.mtls_enabled is true"))
		}
		if cfg.Management.TLSKeyFile == "" {
			errs = append(errs, errors.New("management.tls_key_file is required when management.mtls_enabled is true"))
		}
		if cfg.Management.TLSCAFile == "" {
			errs = append(errs, errors.New("management.tls_ca_file is required when management.mtls_enabled is true"))
		}
	}
	if cfg.CORS.Enabled {
		if len(cfg.CORS.AllowMethods) == 0 {
			errs = append(errs, errors.New("cors.allow_methods must contain at least one method when cors is enabled"))
		}
		if cfg.CORS.AllowCredentials && cfg.CORS.AllowAllOrigins {
			errs = append(errs, errors.New("cors.allow_credentials cannot be true when cors.allow_all_origins is true"))
		}
		if cfg.CORS.AllowAllOrigins && len(cfg.CORS.AllowOrigins) > 0 {
			errs = append(errs, errors.New("cors.allow_all_origins and cors.allow_origins cannot both be set"))
		}
		if cfg.CORS.MaxAge < 0 {
			errs = append(errs, errors.New("cors.max_age cannot be negative"))
		}
		if cfg.CORS.OptionsResponseStatusCode < 200 || cfg.CORS.OptionsResponseStatusCode > 299 {
			errs = append(errs, errors.New("cors.options_response_status_code must be between 200 and 299"))
		}
		for index, origin := range cfg.CORS.AllowOrigins {
			trimmed := strings.TrimSpace(origin)
			if trimmed == "" {
				errs = append(errs, fmt.Errorf("cors.allow_origins[%d] cannot be empty", index))
				continue
			}
			if strings.Contains(trimmed, "*") {
				if trimmed == "*" {
					continue
				}
				if !cfg.CORS.AllowWildcard {
					errs = append(errs, fmt.Errorf("cors.allow_origins[%d] contains wildcard but cors.allow_wildcard is false", index))
					continue
				}
				if strings.Count(trimmed, "*") > 1 {
					errs = append(errs, fmt.Errorf("cors.allow_origins[%d] can contain only one '*' wildcard", index))
				}
			}
		}
		for index, method := range cfg.CORS.AllowMethods {
			if strings.TrimSpace(method) == "" {
				errs = append(errs, fmt.Errorf("cors.allow_methods[%d] cannot be empty", index))
			}
		}
		for index, schema := range cfg.CORS.CustomSchemas {
			if strings.TrimSpace(schema) == "" {
				errs = append(errs, fmt.Errorf("cors.custom_schemas[%d] cannot be empty", index))
			}
		}
	}
	if cfg.SecurityHeaders.STSSeconds < 0 {
		errs = append(errs, errors.New("security_headers.sts_seconds cannot be negative"))
	}
	if cfg.Security.HTTPSignature.Enabled {
		if strings.TrimSpace(cfg.Security.HTTPSignature.KeyIDHeader) == "" {
			errs = append(errs, errors.New("security.http_signature.key_id_header is required when security.http_signature.enabled is true"))
		}
		if strings.TrimSpace(cfg.Security.HTTPSignature.TimestampHeader) == "" {
			errs = append(errs, errors.New("security.http_signature.timestamp_header is required when security.http_signature.enabled is true"))
		}
		if strings.TrimSpace(cfg.Security.HTTPSignature.SignatureHeader) == "" {
			errs = append(errs, errors.New("security.http_signature.signature_header is required when security.http_signature.enabled is true"))
		}
		if cfg.Security.HTTPSignature.RequireNonce && strings.TrimSpace(cfg.Security.HTTPSignature.NonceHeader) == "" {
			errs = append(errs, errors.New("security.http_signature.nonce_header is required when security.http_signature.require_nonce is true"))
		}
		if cfg.Security.HTTPSignature.MaxClockSkew <= 0 {
			errs = append(errs, errors.New("security.http_signature.max_clock_skew must be greater than zero when security.http_signature.enabled is true"))
		}
		if cfg.Security.HTTPSignature.NonceTTL <= 0 {
			errs = append(errs, errors.New("security.http_signature.nonce_ttl must be greater than zero when security.http_signature.enabled is true"))
		}
		if len(cfg.Security.HTTPSignature.StaticKeys) == 0 {
			errs = append(errs, errors.New("security.http_signature.static_keys must contain at least one key when security.http_signature.enabled is true"))
		}
		for keyID, secret := range cfg.Security.HTTPSignature.StaticKeys {
			if strings.TrimSpace(keyID) == "" {
				errs = append(errs, errors.New("security.http_signature.static_keys contains an empty key id"))
				continue
			}
			if strings.TrimSpace(secret) == "" {
				errs = append(errs, fmt.Errorf("security.http_signature.static_keys.%s cannot be empty", keyID))
			}
		}
	}
	if cfg.I18n.Enabled {
		if strings.TrimSpace(cfg.I18n.DefaultLocale) == "" {
			errs = append(errs, errors.New("i18n.default_locale is required when i18n is enabled"))
		}
		if len(cfg.I18n.SupportedLocales) == 0 {
			errs = append(errs, errors.New("i18n.supported_locales must contain at least one locale when i18n is enabled"))
		}
		normalizedSupported := make([]string, 0, len(cfg.I18n.SupportedLocales))
		for index, locale := range cfg.I18n.SupportedLocales {
			trimmed := strings.TrimSpace(locale)
			if trimmed == "" {
				errs = append(errs, fmt.Errorf("i18n.supported_locales[%d] cannot be empty", index))
				continue
			}
			normalizedSupported = append(normalizedSupported, strings.ToLower(trimmed))
		}
		if strings.TrimSpace(cfg.I18n.QueryParam) == "" {
			errs = append(errs, errors.New("i18n.query_param is required when i18n is enabled"))
		}
		if strings.TrimSpace(cfg.I18n.HeaderName) == "" {
			errs = append(errs, errors.New("i18n.header_name is required when i18n is enabled"))
		}
		validFallbackModes := []string{"base", "default"}
		if !contains(validFallbackModes, strings.ToLower(strings.TrimSpace(cfg.I18n.FallbackMode))) {
			errs = append(errs, fmt.Errorf("invalid i18n.fallback_mode: %s (must be one of: %v)", cfg.I18n.FallbackMode, validFallbackModes))
		}
		defaultLocale := strings.ToLower(strings.TrimSpace(cfg.I18n.DefaultLocale))
		if defaultLocale != "" && len(normalizedSupported) > 0 && !contains(normalizedSupported, defaultLocale) {
			errs = append(errs, errors.New("i18n.default_locale must be included in i18n.supported_locales"))
		}
	}
	validSameSiteValues := []string{"lax", "strict", "none"}
	if cfg.Session.Enabled {
		validSessionStores := []string{"inmemory", "redis", "memcached"}
		store := strings.ToLower(strings.TrimSpace(cfg.Session.Store))
		if !contains(validSessionStores, store) {
			errs = append(errs, fmt.Errorf("invalid session.store: %s (must be one of: %v)", cfg.Session.Store, validSessionStores))
		}
		if cfg.Session.TTL <= 0 {
			errs = append(errs, errors.New("session.ttl must be greater than zero when session is enabled"))
		}
		if cfg.Session.IdleTimeout <= 0 {
			errs = append(errs, errors.New("session.idle_timeout must be greater than zero when session is enabled"))
		}
		if !contains(validSameSiteValues, strings.ToLower(strings.TrimSpace(cfg.Session.CookieSameSite))) {
			errs = append(errs, fmt.Errorf("invalid session.cookie_same_site: %s (must be one of: %v)", cfg.Session.CookieSameSite, validSameSiteValues))
		}
		switch store {
		case "redis":
			if strings.TrimSpace(cfg.Session.Redis.URL) == "" {
				errs = append(errs, errors.New("session.redis.url is required when session.store=redis"))
			}
		case "memcached":
			if len(cfg.Session.Memcached.Addresses) == 0 {
				errs = append(errs, errors.New("session.memcached.addresses must contain at least one endpoint when session.store=memcached"))
			}
		}
	}
	if cfg.CSRF.Enabled {
		if !cfg.Session.Enabled {
			errs = append(errs, errors.New("session.enabled must be true when csrf.enabled is true"))
		}
		if strings.TrimSpace(cfg.CSRF.HeaderName) == "" {
			errs = append(errs, errors.New("csrf.header_name is required when csrf is enabled"))
		}
		if strings.TrimSpace(cfg.CSRF.CookieName) == "" {
			errs = append(errs, errors.New("csrf.cookie_name is required when csrf is enabled"))
		}
		if cfg.CSRF.CookieTTL <= 0 {
			errs = append(errs, errors.New("csrf.cookie_ttl must be greater than zero when csrf is enabled"))
		}
		if !contains(validSameSiteValues, strings.ToLower(strings.TrimSpace(cfg.CSRF.CookieSameSite))) {
			errs = append(errs, fmt.Errorf("invalid csrf.cookie_same_site: %s (must be one of: %v)", cfg.CSRF.CookieSameSite, validSameSiteValues))
		}
	}
	if cfg.SSE.Enabled {
		validSSEStores := []string{"inmemory", "redis"}
		store := strings.ToLower(strings.TrimSpace(cfg.SSE.Store))
		if !contains(validSSEStores, store) {
			errs = append(errs, fmt.Errorf("invalid sse.store: %s (must be one of: %v)", cfg.SSE.Store, validSSEStores))
		}
		validSSEBuses := []string{"none", "inmemory", "redis", "eventbus"}
		bus := strings.ToLower(strings.TrimSpace(cfg.SSE.Bus))
		if !contains(validSSEBuses, bus) {
			errs = append(errs, fmt.Errorf("invalid sse.bus: %s (must be one of: %v)", cfg.SSE.Bus, validSSEBuses))
		}
		if strings.TrimSpace(cfg.SSE.Endpoint) == "" || !strings.HasPrefix(strings.TrimSpace(cfg.SSE.Endpoint), "/") {
			errs = append(errs, errors.New("sse.endpoint must be a non-empty absolute path"))
		}
		if cfg.SSE.ReplayLimit <= 0 {
			errs = append(errs, errors.New("sse.replay_limit must be greater than zero when sse is enabled"))
		}
		if cfg.SSE.ClientBuffer <= 0 {
			errs = append(errs, errors.New("sse.client_buffer must be greater than zero when sse is enabled"))
		}
		if cfg.SSE.MaxConnections <= 0 {
			errs = append(errs, errors.New("sse.max_connections must be greater than zero when sse is enabled"))
		}
		if cfg.SSE.HeartbeatInterval <= 0 {
			errs = append(errs, errors.New("sse.heartbeat_interval must be greater than zero when sse is enabled"))
		}
		if cfg.SSE.DefaultRetryMS <= 0 {
			errs = append(errs, errors.New("sse.default_retry_ms must be greater than zero when sse is enabled"))
		}
		if store == "redis" || bus == "redis" {
			if strings.TrimSpace(cfg.SSE.Redis.URL) == "" {
				errs = append(errs, errors.New("sse.redis.url is required when sse.store=redis or sse.bus=redis"))
			}
		}
		if bus == "eventbus" && strings.TrimSpace(cfg.EventBus.Type) == "" {
			errs = append(errs, errors.New("eventbus.type is required when sse.bus=eventbus"))
		}
	}
	if cfg.Email.Enabled {
		provider := strings.ToLower(strings.TrimSpace(cfg.Email.Provider))
		validProviders := []string{
			"smtp", "ses", "sendgrid", "mailgun", "mailchimp",
			"mailersend", "postmark", "mailtrap", "smtp2go",
			"sendpulse", "brevo", "mailjet",
		}
		if !contains(validProviders, provider) {
			errs = append(errs, fmt.Errorf("invalid email.provider: %s (must be one of: %v)", cfg.Email.Provider, validProviders))
		}
		switch provider {
		case "smtp":
			if strings.TrimSpace(cfg.Email.SMTP.Host) == "" {
				errs = append(errs, errors.New("email.smtp.host is required when email.provider=smtp"))
			}
		case "ses":
			if strings.TrimSpace(cfg.Email.SES.Region) == "" {
				errs = append(errs, errors.New("email.ses.region is required when email.provider=ses"))
			}
		case "sendgrid":
			if strings.TrimSpace(cfg.Email.SendGrid.Token) == "" {
				errs = append(errs, errors.New("email.sendgrid.token is required when email.provider=sendgrid"))
			}
		case "mailgun":
			if strings.TrimSpace(cfg.Email.Mailgun.Token) == "" {
				errs = append(errs, errors.New("email.mailgun.token is required when email.provider=mailgun"))
			}
			if strings.TrimSpace(cfg.Email.Mailgun.Domain) == "" {
				errs = append(errs, errors.New("email.mailgun.domain is required when email.provider=mailgun"))
			}
		case "mailchimp":
			if strings.TrimSpace(cfg.Email.Mailchimp.Token) == "" {
				errs = append(errs, errors.New("email.mailchimp.token is required when email.provider=mailchimp"))
			}
		case "mailersend":
			if strings.TrimSpace(cfg.Email.MailerSend.Token) == "" {
				errs = append(errs, errors.New("email.mailersend.token is required when email.provider=mailersend"))
			}
		case "postmark":
			if strings.TrimSpace(cfg.Email.Postmark.ServerToken) == "" {
				errs = append(errs, errors.New("email.postmark.server_token is required when email.provider=postmark"))
			}
		case "mailtrap":
			if strings.TrimSpace(cfg.Email.Mailtrap.Token) == "" {
				errs = append(errs, errors.New("email.mailtrap.token is required when email.provider=mailtrap"))
			}
		case "smtp2go":
			if strings.TrimSpace(cfg.Email.SMTP2GO.Token) == "" {
				errs = append(errs, errors.New("email.smtp2go.token is required when email.provider=smtp2go"))
			}
		case "sendpulse":
			if strings.TrimSpace(cfg.Email.SendPulse.Token) == "" {
				errs = append(errs, errors.New("email.sendpulse.token is required when email.provider=sendpulse"))
			}
		case "brevo":
			if strings.TrimSpace(cfg.Email.Brevo.Token) == "" {
				errs = append(errs, errors.New("email.brevo.token is required when email.provider=brevo"))
			}
		case "mailjet":
			if strings.TrimSpace(cfg.Email.Mailjet.APIKey) == "" {
				errs = append(errs, errors.New("email.mailjet.api_key is required when email.provider=mailjet"))
			}
			if strings.TrimSpace(cfg.Email.Mailjet.APISecret) == "" {
				errs = append(errs, errors.New("email.mailjet.api_secret is required when email.provider=mailjet"))
			}
		}
	}

	if cfg.RateLimit.Enabled {
		validRateTypes := []string{"local", "redis"}
		if !contains(validRateTypes, strings.ToLower(cfg.RateLimit.Type)) {
			errs = append(errs, fmt.Errorf("invalid rate_limit.type: %s (must be one of: %v)", cfg.RateLimit.Type, validRateTypes))
		}
		if strings.ToLower(cfg.RateLimit.Type) == "redis" && cfg.RateLimit.Redis.URL == "" {
			errs = append(errs, errors.New("rate_limit.redis.url is required when rate_limit.type=redis"))
		}
		if cfg.RateLimit.RequestsPerSecond <= 0 {
			errs = append(errs, errors.New("rate_limit.requests_per_second must be greater than zero when rate limiting is enabled"))
		}
		if cfg.RateLimit.Burst < 0 {
			errs = append(errs, errors.New("rate_limit.burst cannot be negative"))
		}
		if cfg.RateLimit.Window <= 0 {
			errs = append(errs, errors.New("rate_limit.window must be greater than zero"))
		}
	}

	// Validate Database configuration
	if cfg.Database.Type != "" {
		validTypes := []string{DatabaseTypePostgres, DatabaseTypeMySQL, DatabaseTypeMongoDB, DatabaseTypeDynamoDB}
		if !contains(validTypes, cfg.Database.Type) {
			errs = append(errs, fmt.Errorf("invalid database.type: %s (must be one of: %v)", cfg.Database.Type, validTypes))
		}
		if cfg.Database.Type != DatabaseTypeDynamoDB && cfg.Database.URL == "" {
			errs = append(errs, errors.New("database.url is required when database.type is specified"))
		}
		if cfg.Database.Type == DatabaseTypeMongoDB && cfg.Database.DatabaseName == "" {
			errs = append(errs, errors.New("database.database_name is required when database.type is mongodb"))
		}
		if cfg.Database.Type == DatabaseTypeDynamoDB && cfg.Database.Region == "" {
			errs = append(errs, errors.New("database.region is required when database.type is dynamodb"))
		}
	}

	// Validate Cache configuration
	if cfg.Cache.Type != "" {
		validTypes := []string{"redis", "inmemory"}
		if !contains(validTypes, cfg.Cache.Type) {
			errs = append(errs, fmt.Errorf("invalid cache.type: %s (must be one of: %v)", cfg.Cache.Type, validTypes))
		}
		if cfg.Cache.Type == "redis" && cfg.Cache.URL == "" {
			errs = append(errs, errors.New("cache.url is required when cache.type is redis"))
		}
	}

	// Validate Object Storage configuration
	if cfg.ObjectStorage.Enabled {
		storageType := strings.ToLower(strings.TrimSpace(cfg.ObjectStorage.Type))
		if storageType == "" {
			storageType = "s3"
		}
		validTypes := []string{"s3"}
		if !contains(validTypes, storageType) {
			errs = append(errs, fmt.Errorf("invalid object_storage.type: %s (must be one of: %v)", cfg.ObjectStorage.Type, validTypes))
		}
		if storageType == "s3" {
			if strings.TrimSpace(cfg.ObjectStorage.S3.Bucket) == "" {
				errs = append(errs, errors.New("object_storage.s3.bucket is required when object_storage.enabled is true and type=s3"))
			}
			if strings.TrimSpace(cfg.ObjectStorage.S3.Region) == "" {
				errs = append(errs, errors.New("object_storage.s3.region is required when object_storage.enabled is true and type=s3"))
			}
			if cfg.ObjectStorage.S3.OperationTimeout <= 0 {
				errs = append(errs, errors.New("object_storage.s3.operation_timeout must be greater than zero when object_storage.enabled is true and type=s3"))
			}
			if cfg.ObjectStorage.S3.PresignExpiry <= 0 {
				errs = append(errs, errors.New("object_storage.s3.presign_expiry must be greater than zero when object_storage.enabled is true and type=s3"))
			}
		}
	}

	// Validate Search configuration
	if cfg.Search.Type != "" {
		searchType := strings.ToLower(cfg.Search.Type)
		validTypes := []string{"opensearch", "elasticsearch"}
		if !contains(validTypes, searchType) {
			errs = append(errs, fmt.Errorf("invalid search.type: %s (must be one of: %v)", cfg.Search.Type, validTypes))
		}
		driver := strings.ToLower(strings.TrimSpace(cfg.Search.Driver))
		if driver == "" {
			driver = "http"
		}
		validDrivers := []string{"http", "opensearch-sdk", "elasticsearch-sdk"}
		if !contains(validDrivers, driver) {
			errs = append(errs, fmt.Errorf("invalid search.driver: %s (must be one of: %v)", cfg.Search.Driver, validDrivers))
		}
		if driver == "opensearch-sdk" && searchType != "opensearch" {
			errs = append(errs, errors.New("search.driver=opensearch-sdk requires search.type=opensearch"))
		}
		if driver == "elasticsearch-sdk" && searchType != "elasticsearch" {
			errs = append(errs, errors.New("search.driver=elasticsearch-sdk requires search.type=elasticsearch"))
		}
		if strings.TrimSpace(cfg.Search.URL) == "" && len(cfg.Search.URLs) == 0 {
			errs = append(errs, errors.New("search.url or search.urls is required when search.type is specified"))
		}
		if cfg.Search.AWSAuthEnabled {
			if strings.TrimSpace(cfg.Search.AWSRegion) == "" {
				errs = append(errs, errors.New("search.aws_region is required when search.aws_auth_enabled is true"))
			}
			if strings.TrimSpace(cfg.Search.AWSService) == "" {
				errs = append(errs, errors.New("search.aws_service is required when search.aws_auth_enabled is true"))
			}
		}
	}

	// Validate EventBus configuration
	if cfg.EventBus.Type != "" {
		validTypes := []string{EventBusTypeKafka, EventBusTypeRabbitMQ, EventBusTypeSQS}
		if !contains(validTypes, cfg.EventBus.Type) {
			errs = append(errs, fmt.Errorf("invalid eventbus.type: %s (must be one of: %v)", cfg.EventBus.Type, validTypes))
		}
		if cfg.EventBus.Type == EventBusTypeKafka && len(cfg.EventBus.Brokers) == 0 {
			errs = append(errs, errors.New("eventbus.brokers is required when eventbus.type is specified"))
		}
		if cfg.EventBus.Type == EventBusTypeRabbitMQ && cfg.EventBus.URL == "" && len(cfg.EventBus.Brokers) == 0 {
			errs = append(errs, errors.New("eventbus.url (or eventbus.brokers[0]) is required when eventbus.type is rabbitmq"))
		}
		if cfg.EventBus.Type == EventBusTypeSQS {
			if cfg.EventBus.Region == "" {
				errs = append(errs, errors.New("eventbus.region is required when eventbus.type is sqs"))
			}
			if cfg.EventBus.QueueURL == "" {
				errs = append(errs, errors.New("eventbus.queue_url is required when eventbus.type is sqs"))
			}
		}

		validSerializers := []string{"json", "protobuf", "avro"}
		if !contains(validSerializers, cfg.EventBus.Serializer) {
			errs = append(errs, fmt.Errorf("invalid eventbus.serializer: %s (must be one of: %v)", cfg.EventBus.Serializer, validSerializers))
		}
	}

	// Validate Jobs configuration
	jobsBackend := strings.ToLower(strings.TrimSpace(cfg.Jobs.Backend))
	if jobsBackend == "" {
		jobsBackend = JobsBackendEventBus
	}
	validJobsBackends := []string{JobsBackendEventBus, JobsBackendRedis}
	if !contains(validJobsBackends, jobsBackend) {
		errs = append(errs, fmt.Errorf("invalid jobs.backend: %s (must be one of: %v)", cfg.Jobs.Backend, validJobsBackends))
	}
	if cfg.Jobs.Worker.Concurrency <= 0 {
		errs = append(errs, errors.New("jobs.worker.concurrency must be greater than zero"))
	}
	if cfg.Jobs.Worker.LeaseTTL <= 0 {
		errs = append(errs, errors.New("jobs.worker.lease_ttl must be greater than zero"))
	}
	if cfg.Jobs.Worker.ReserveTimeout <= 0 {
		errs = append(errs, errors.New("jobs.worker.reserve_timeout must be greater than zero"))
	}
	if cfg.Jobs.Worker.StopTimeout <= 0 {
		errs = append(errs, errors.New("jobs.worker.stop_timeout must be greater than zero"))
	}
	if cfg.Jobs.Retry.MaxAttempts <= 0 {
		errs = append(errs, errors.New("jobs.retry.max_attempts must be greater than zero"))
	}
	if cfg.Jobs.Retry.InitialBackoff <= 0 {
		errs = append(errs, errors.New("jobs.retry.initial_backoff must be greater than zero"))
	}
	if cfg.Jobs.Retry.MaxBackoff <= 0 {
		errs = append(errs, errors.New("jobs.retry.max_backoff must be greater than zero"))
	}
	if cfg.Jobs.Retry.MaxBackoff < cfg.Jobs.Retry.InitialBackoff {
		errs = append(errs, errors.New("jobs.retry.max_backoff must be greater than or equal to jobs.retry.initial_backoff"))
	}
	if cfg.Jobs.Retry.AttemptTimeout <= 0 {
		errs = append(errs, errors.New("jobs.retry.attempt_timeout must be greater than zero"))
	}
	if cfg.Jobs.DLQ.Enabled && strings.TrimSpace(cfg.Jobs.DLQ.QueueSuffix) == "" {
		errs = append(errs, errors.New("jobs.dlq.queue_suffix is required when jobs.dlq.enabled is true"))
	}
	if jobsBackend == JobsBackendRedis {
		if strings.TrimSpace(cfg.Jobs.Redis.URL) == "" {
			errs = append(errs, errors.New("jobs.redis.url is required when jobs.backend is redis"))
		}
		if strings.TrimSpace(cfg.Jobs.Redis.Prefix) == "" {
			errs = append(errs, errors.New("jobs.redis.prefix is required when jobs.backend is redis"))
		}
		if cfg.Jobs.Redis.OperationTimeout <= 0 {
			errs = append(errs, errors.New("jobs.redis.operation_timeout must be greater than zero when jobs.backend is redis"))
		}
	}

	// Validate Scheduler configuration
	if cfg.Scheduler.Enabled {
		lockProvider := strings.ToLower(strings.TrimSpace(cfg.Scheduler.LockProvider))
		if lockProvider == "" {
			lockProvider = SchedulerLockProviderRedis
		}
		validLockProviders := []string{SchedulerLockProviderRedis, SchedulerLockProviderPostgres}
		if !contains(validLockProviders, lockProvider) {
			errs = append(errs, fmt.Errorf("invalid scheduler.lock_provider: %s (must be one of: %v)", cfg.Scheduler.LockProvider, validLockProviders))
		}
		if cfg.Scheduler.LockTTL <= 0 {
			errs = append(errs, errors.New("scheduler.lock_ttl must be greater than zero"))
		}
		if cfg.Scheduler.DispatchTimeout <= 0 {
			errs = append(errs, errors.New("scheduler.dispatch_timeout must be greater than zero"))
		}
		timezone := strings.TrimSpace(cfg.Scheduler.Timezone)
		if timezone == "" {
			timezone = "UTC"
		}
		if _, err := time.LoadLocation(timezone); err != nil {
			errs = append(errs, fmt.Errorf("invalid scheduler.timezone: %w", err))
		}

		switch lockProvider {
		case SchedulerLockProviderRedis:
			redisURL := strings.TrimSpace(cfg.Scheduler.Redis.URL)
			if redisURL == "" {
				redisURL = strings.TrimSpace(cfg.Cache.URL)
			}
			if redisURL == "" {
				errs = append(errs, errors.New("scheduler.redis.url (or cache.url) is required when scheduler.lock_provider is redis"))
			}
			if strings.TrimSpace(cfg.Scheduler.Redis.Prefix) == "" {
				errs = append(errs, errors.New("scheduler.redis.prefix is required when scheduler.lock_provider is redis"))
			}
			if cfg.Scheduler.Redis.OperationTimeout <= 0 {
				errs = append(errs, errors.New("scheduler.redis.operation_timeout must be greater than zero when scheduler.lock_provider is redis"))
			}
		case SchedulerLockProviderPostgres:
			postgresURL := strings.TrimSpace(cfg.Scheduler.Postgres.URL)
			if postgresURL == "" {
				postgresURL = strings.TrimSpace(cfg.Database.URL)
			}
			if postgresURL == "" {
				errs = append(errs, errors.New("scheduler.postgres.url (or database.url) is required when scheduler.lock_provider is postgres"))
			}
			if strings.TrimSpace(cfg.Scheduler.Postgres.Table) == "" {
				errs = append(errs, errors.New("scheduler.postgres.table is required when scheduler.lock_provider is postgres"))
			}
			if cfg.Scheduler.Postgres.OperationTimeout <= 0 {
				errs = append(errs, errors.New("scheduler.postgres.operation_timeout must be greater than zero when scheduler.lock_provider is postgres"))
			}
		}

		taskNames := map[string]struct{}{}
		for idx, task := range cfg.Scheduler.Tasks {
			name := strings.TrimSpace(task.Name)
			if name == "" {
				errs = append(errs, fmt.Errorf("scheduler.tasks[%d].name is required", idx))
				continue
			}
			if _, exists := taskNames[name]; exists {
				errs = append(errs, fmt.Errorf("scheduler.tasks contains duplicate name %q", name))
			}
			taskNames[name] = struct{}{}
			if strings.TrimSpace(task.Cron) == "" {
				errs = append(errs, fmt.Errorf("scheduler.tasks[%s].cron is required", name))
			}
			if strings.TrimSpace(task.Queue) == "" {
				errs = append(errs, fmt.Errorf("scheduler.tasks[%s].queue is required", name))
			}
			if strings.TrimSpace(task.JobName) == "" {
				errs = append(errs, fmt.Errorf("scheduler.tasks[%s].job_name is required", name))
			}
			misfire := strings.ToLower(strings.TrimSpace(task.MisfirePolicy))
			if misfire != "" && misfire != "skip" && misfire != "fire_once" {
				errs = append(errs, fmt.Errorf("scheduler.tasks[%s].misfire_policy must be one of: skip, fire_once", name))
			}
		}
	}

	validValidationModes := []string{"warn", "enforce"}
	kafkaValidationMode := strings.ToLower(strings.TrimSpace(cfg.Validation.Kafka.Mode))
	if kafkaValidationMode == "" {
		kafkaValidationMode = "enforce"
	}
	if !contains(validValidationModes, kafkaValidationMode) {
		errs = append(errs, fmt.Errorf("invalid validation.kafka.mode: %s (must be one of: %v)", cfg.Validation.Kafka.Mode, validValidationModes))
	}
	validCompatibility := []string{"backward", "full"}
	defaultPolicy := strings.ToLower(strings.TrimSpace(cfg.Validation.Kafka.DefaultPolicy))
	if defaultPolicy == "" {
		defaultPolicy = "backward"
	}
	if !contains(validCompatibility, defaultPolicy) {
		errs = append(errs, fmt.Errorf("invalid validation.kafka.default_policy: %s (must be one of: %v)", cfg.Validation.Kafka.DefaultPolicy, []string{"BACKWARD", "FULL"}))
	}
	if cfg.Validation.Kafka.Enabled && strings.TrimSpace(cfg.Validation.Kafka.DescriptorPath) == "" {
		errs = append(errs, errors.New("validation.kafka.descriptor_path is required when validation.kafka.enabled is true"))
	}
	for subject, policy := range cfg.Validation.Kafka.Subjects {
		if strings.TrimSpace(subject) == "" {
			errs = append(errs, errors.New("validation.kafka.subjects contains an empty subject"))
			continue
		}
		if !contains(validCompatibility, strings.ToLower(strings.TrimSpace(policy))) {
			errs = append(errs, fmt.Errorf("invalid validation.kafka.subjects[%s]: %s (must be one of: %v)", subject, policy, []string{"BACKWARD", "FULL"}))
		}
	}

	// Validate Observability configuration
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, cfg.Observability.LogLevel) {
		errs = append(errs, fmt.Errorf("invalid observability.log_level: %s (must be one of: %v)", cfg.Observability.LogLevel, validLogLevels))
	}

	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, cfg.Observability.LogFormat) {
		errs = append(errs, fmt.Errorf("invalid observability.log_format: %s (must be one of: %v)", cfg.Observability.LogFormat, validLogFormats))
	}

	if cfg.Observability.TracingEnabled && cfg.Observability.TracingEndpoint == "" {
		errs = append(errs, errors.New("observability.tracing_endpoint is required when tracing is enabled"))
	}

	// Validate port numbers
	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid http.port: %d (must be between 1 and 65535)", cfg.HTTP.Port))
	}
	if cfg.HTTP.MaxRequestSize < 0 {
		errs = append(errs, errors.New("http.max_request_size cannot be negative"))
	}
	if cfg.Management.Enabled {
		if cfg.Management.Port <= 0 || cfg.Management.Port > 65535 {
			errs = append(errs, fmt.Errorf("invalid management.port: %d (must be between 1 and 65535)", cfg.Management.Port))
		}
		if cfg.HTTP.Port == cfg.Management.Port {
			errs = append(errs, errors.New("http.port and management.port must be different"))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// normalizeStringSlice removes empty strings and trims whitespace
func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
