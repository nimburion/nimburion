package config

import "time"

// Database type constants
const (
	// DatabaseTypePostgres represents PostgreSQL database
	DatabaseTypePostgres = "postgres"
	// DatabaseTypeMySQL represents MySQL database
	DatabaseTypeMySQL = "mysql"
	// DatabaseTypeMongoDB represents MongoDB database
	DatabaseTypeMongoDB = "mongodb"
	// DatabaseTypeDynamoDB represents AWS DynamoDB
	DatabaseTypeDynamoDB = "dynamodb"
)

// Event bus type constants
const (
	// EventBusTypeKafka represents Apache Kafka event bus
	EventBusTypeKafka = "kafka"
	// EventBusTypeRabbitMQ represents RabbitMQ event bus
	EventBusTypeRabbitMQ = "rabbitmq"
	// EventBusTypeSQS represents AWS SQS event bus
	EventBusTypeSQS = "sqs"
)

// Jobs backend type constants
const (
	// JobsBackendEventBus uses event bus for job queue
	JobsBackendEventBus = "eventbus"
	// JobsBackendRedis uses Redis for job queue
	JobsBackendRedis = "redis"
)

// Scheduler lock provider constants
const (
	// SchedulerLockProviderRedis uses Redis for distributed locks
	SchedulerLockProviderRedis = "redis"
	// SchedulerLockProviderPostgres uses PostgreSQL for distributed locks
	SchedulerLockProviderPostgres = "postgres"
)

// Config is the root configuration structure for the microservice framework
type Config struct {
	RouterType      string `mapstructure:"router_type"`
	Service         ServiceConfig
	HTTP            HTTPConfig
	Management      ManagementConfig
	CORS            CORSConfig
	SecurityHeaders SecurityHeadersConfig `mapstructure:"security_headers"`
	Security        SecurityConfig        `mapstructure:"security"`
	I18n            I18nConfig            `mapstructure:"i18n"`
	Session         SessionConfig         `mapstructure:"session"`
	CSRF            CSRFConfig            `mapstructure:"csrf"`
	SSE             SSEConfig             `mapstructure:"sse"`
	Email           EmailConfig           `mapstructure:"email"`
	Auth            AuthConfig
	Database        DatabaseConfig
	Cache           CacheConfig
	ObjectStorage   ObjectStorageConfig `mapstructure:"object_storage"`
	Search          SearchConfig
	EventBus        EventBusConfig  `mapstructure:"eventbus"`
	Jobs            JobsConfig      `mapstructure:"jobs"`
	Scheduler       SchedulerConfig `mapstructure:"scheduler"`
	Validation      ValidationConfig
	Observability   ObservabilityConfig
	Swagger         SwaggerConfig
	RateLimit       RateLimitConfig
}

// ServiceConfig configures service identity metadata.
type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

// HTTPConfig configures the public API server
type HTTPConfig struct {
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	MaxRequestSize int64         `mapstructure:"max_request_size"`
}

// ManagementConfig configures the management server
type ManagementConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	AuthEnabled  bool          `mapstructure:"auth_enabled"`
	MTLSEnabled  bool          `mapstructure:"mtls_enabled"`
	TLSCertFile  string        `mapstructure:"tls_cert_file"`
	TLSKeyFile   string        `mapstructure:"tls_key_file"`
	TLSCAFile    string        `mapstructure:"tls_ca_file"`
}

// CORSConfig configures CORS middleware for browser-based clients.
type CORSConfig struct {
	Enabled                   bool          `mapstructure:"enabled"`
	AllowAllOrigins           bool          `mapstructure:"allow_all_origins"`
	AllowOrigins              []string      `mapstructure:"allow_origins"`
	AllowMethods              []string      `mapstructure:"allow_methods"`
	AllowPrivateNetwork       bool          `mapstructure:"allow_private_network"`
	AllowHeaders              []string      `mapstructure:"allow_headers"`
	ExposeHeaders             []string      `mapstructure:"expose_headers"`
	AllowCredentials          bool          `mapstructure:"allow_credentials"`
	MaxAge                    time.Duration `mapstructure:"max_age"`
	AllowWildcard             bool          `mapstructure:"allow_wildcard"`
	AllowBrowserExtensions    bool          `mapstructure:"allow_browser_extensions"`
	CustomSchemas             []string      `mapstructure:"custom_schemas"`
	AllowWebSockets           bool          `mapstructure:"allow_websockets"`
	AllowFiles                bool          `mapstructure:"allow_files"`
	OptionsResponseStatusCode int           `mapstructure:"options_response_status_code"`
}

// SecurityHeadersConfig configures transport/header hardening middleware.
type SecurityHeadersConfig struct {
	Enabled                   bool              `mapstructure:"enabled"`
	IsDevelopment             bool              `mapstructure:"is_development"`
	AllowedHosts              []string          `mapstructure:"allowed_hosts"`
	SSLRedirect               bool              `mapstructure:"ssl_redirect"`
	SSLTemporaryRedirect      bool              `mapstructure:"ssl_temporary_redirect"`
	SSLHost                   string            `mapstructure:"ssl_host"`
	SSLProxyHeaders           map[string]string `mapstructure:"ssl_proxy_headers"`
	DontRedirectIPV4Hostnames bool              `mapstructure:"dont_redirect_ipv4_hostnames"`
	STSSeconds                int64             `mapstructure:"sts_seconds"`
	STSIncludeSubdomains      bool              `mapstructure:"sts_include_subdomains"`
	STSPreload                bool              `mapstructure:"sts_preload"`
	CustomFrameOptions        string            `mapstructure:"custom_frame_options"`
	ContentTypeNosniff        bool              `mapstructure:"content_type_nosniff"`
	ContentSecurityPolicy     string            `mapstructure:"content_security_policy"`
	ReferrerPolicy            string            `mapstructure:"referrer_policy"`
	PermissionsPolicy         string            `mapstructure:"permissions_policy"`
	IENoOpen                  bool              `mapstructure:"ie_no_open"`
	XDNSPrefetchControl       string            `mapstructure:"x_dns_prefetch_control"`
	CrossOriginOpenerPolicy   string            `mapstructure:"cross_origin_opener_policy"`
	CrossOriginResourcePolicy string            `mapstructure:"cross_origin_resource_policy"`
	CrossOriginEmbedderPolicy string            `mapstructure:"cross_origin_embedder_policy"`
	CustomHeaders             map[string]string `mapstructure:"custom_headers"`
}

// SecurityConfig configures non-header security middleware.
type SecurityConfig struct {
	HTTPSignature HTTPSignatureConfig `mapstructure:"http_signature"`
}

// HTTPSignatureConfig configures HMAC request signing verification.
type HTTPSignatureConfig struct {
	Enabled              bool              `mapstructure:"enabled"`
	KeyIDHeader          string            `mapstructure:"key_id_header"`
	TimestampHeader      string            `mapstructure:"timestamp_header"`
	NonceHeader          string            `mapstructure:"nonce_header"`
	SignatureHeader      string            `mapstructure:"signature_header"`
	MaxClockSkew         time.Duration     `mapstructure:"max_clock_skew"`
	NonceTTL             time.Duration     `mapstructure:"nonce_ttl"`
	RequireNonce         bool              `mapstructure:"require_nonce"`
	ExcludedPathPrefixes []string          `mapstructure:"excluded_path_prefixes"`
	StaticKeys           map[string]string `mapstructure:"static_keys"`
}

// I18nConfig configures locale resolution and translation catalogs.
type I18nConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	DefaultLocale    string   `mapstructure:"default_locale"`
	SupportedLocales []string `mapstructure:"supported_locales"`
	QueryParam       string   `mapstructure:"query_param"`
	HeaderName       string   `mapstructure:"header_name"`
	FallbackMode     string   `mapstructure:"fallback_mode"` // base, default
	CatalogPath      string   `mapstructure:"catalog_path"`
}

// SessionConfig configures server-side session management.
type SessionConfig struct {
	Enabled        bool                   `mapstructure:"enabled"`
	Store          string                 `mapstructure:"store"` // inmemory, redis, memcached
	TTL            time.Duration          `mapstructure:"ttl"`
	IdleTimeout    time.Duration          `mapstructure:"idle_timeout"`
	CookieName     string                 `mapstructure:"cookie_name"`
	CookiePath     string                 `mapstructure:"cookie_path"`
	CookieDomain   string                 `mapstructure:"cookie_domain"`
	CookieSecure   bool                   `mapstructure:"cookie_secure"`
	CookieHTTPOnly bool                   `mapstructure:"cookie_http_only"`
	CookieSameSite string                 `mapstructure:"cookie_same_site"` // lax, strict, none
	AutoCreate     bool                   `mapstructure:"auto_create"`
	Redis          SessionRedisConfig     `mapstructure:"redis"`
	Memcached      SessionMemcachedConfig `mapstructure:"memcached"`
}

// SessionRedisConfig configures Redis-backed sessions.
type SessionRedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	Prefix           string        `mapstructure:"prefix"`
}

// SessionMemcachedConfig configures memcached-backed sessions.
type SessionMemcachedConfig struct {
	Addresses []string      `mapstructure:"addresses"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Prefix    string        `mapstructure:"prefix"`
}

// CSRFConfig configures CSRF middleware for cookie/session authentication.
type CSRFConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	HeaderName     string        `mapstructure:"header_name"`
	CookieName     string        `mapstructure:"cookie_name"`
	CookiePath     string        `mapstructure:"cookie_path"`
	CookieDomain   string        `mapstructure:"cookie_domain"`
	CookieSecure   bool          `mapstructure:"cookie_secure"`
	CookieSameSite string        `mapstructure:"cookie_same_site"` // lax, strict, none
	CookieTTL      time.Duration `mapstructure:"cookie_ttl"`
	ExemptMethods  []string      `mapstructure:"exempt_methods"`
	ExemptPaths    []string      `mapstructure:"exempt_paths"`
}

// SSEConfig configures Server-Sent Events runtime.
type SSEConfig struct {
	Enabled               bool              `mapstructure:"enabled"`
	Endpoint              string            `mapstructure:"endpoint"`
	Store                 string            `mapstructure:"store"` // inmemory, redis
	Bus                   string            `mapstructure:"bus"`   // none, inmemory, redis, eventbus
	ReplayLimit           int               `mapstructure:"replay_limit"`
	ClientBuffer          int               `mapstructure:"client_buffer"`
	MaxConnections        int               `mapstructure:"max_connections"`
	HeartbeatInterval     time.Duration     `mapstructure:"heartbeat_interval"`
	DefaultRetryMS        int               `mapstructure:"default_retry_ms"`
	DropOnBackpressure    bool              `mapstructure:"drop_on_backpressure"`
	ChannelQueryParam     string            `mapstructure:"channel_query_param"`
	TenantQueryParam      string            `mapstructure:"tenant_query_param"`
	SubjectQueryParam     string            `mapstructure:"subject_query_param"`
	LastEventIDQueryParam string            `mapstructure:"last_event_id_query_param"`
	Redis                 SSERedisConfig    `mapstructure:"redis"`
	EventBus              SSEEventBusConfig `mapstructure:"eventbus"`
}

// SSERedisConfig configures Redis store/bus for SSE.
type SSERedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	HistoryPrefix    string        `mapstructure:"history_prefix"`
	PubSubPrefix     string        `mapstructure:"pubsub_prefix"`
}

// SSEEventBusConfig configures framework eventbus bridge for SSE fan-out.
type SSEEventBusConfig struct {
	TopicPrefix      string        `mapstructure:"topic_prefix"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// EmailConfig configures pluggable outbound email providers.
type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Provider string `mapstructure:"provider"` // smtp, ses, sendgrid, mailgun, mailchimp, mailersend, postmark, mailtrap, smtp2go, sendpulse, brevo, mailjet

	SMTP       EmailSMTPConfig     `mapstructure:"smtp"`
	SES        EmailSESConfig      `mapstructure:"ses"`
	SendGrid   EmailTokenConfig    `mapstructure:"sendgrid"`
	Mailgun    EmailMailgunConfig  `mapstructure:"mailgun"`
	Mailchimp  EmailTokenConfig    `mapstructure:"mailchimp"`
	MailerSend EmailTokenConfig    `mapstructure:"mailersend"`
	Postmark   EmailPostmarkConfig `mapstructure:"postmark"`
	Mailtrap   EmailTokenConfig    `mapstructure:"mailtrap"`
	SMTP2GO    EmailTokenConfig    `mapstructure:"smtp2go"`
	SendPulse  EmailTokenConfig    `mapstructure:"sendpulse"`
	Brevo      EmailTokenConfig    `mapstructure:"brevo"`
	Mailjet    EmailMailjetConfig  `mapstructure:"mailjet"`
}

// EmailTokenConfig configures token-based HTTP email providers.
type EmailTokenConfig struct {
	Token            string        `mapstructure:"token"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// EmailSMTPConfig configures SMTP provider.
type EmailSMTPConfig struct {
	Host               string        `mapstructure:"host"`
	Port               int           `mapstructure:"port"`
	Username           string        `mapstructure:"username"`
	Password           string        `mapstructure:"password"`
	From               string        `mapstructure:"from"`
	EnableTLS          bool          `mapstructure:"enable_tls"`
	InsecureSkipVerify bool          `mapstructure:"insecure_skip_verify"`
	OperationTimeout   time.Duration `mapstructure:"operation_timeout"`
}

// EmailSESConfig configures AWS SES provider.
type EmailSESConfig struct {
	Region           string        `mapstructure:"region"`
	Endpoint         string        `mapstructure:"endpoint"`
	AccessKeyID      string        `mapstructure:"access_key_id"`
	SecretAccessKey  string        `mapstructure:"secret_access_key"`
	SessionToken     string        `mapstructure:"session_token"`
	From             string        `mapstructure:"from"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// EmailMailgunConfig configures Mailgun provider.
type EmailMailgunConfig struct {
	Token            string        `mapstructure:"token"`
	Domain           string        `mapstructure:"domain"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// EmailPostmarkConfig configures Postmark provider.
type EmailPostmarkConfig struct {
	ServerToken      string        `mapstructure:"server_token"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// EmailMailjetConfig configures Mailjet provider.
type EmailMailjetConfig struct {
	APIKey           string        `mapstructure:"api_key"`
	APISecret        string        `mapstructure:"api_secret"`
	From             string        `mapstructure:"from"`
	BaseURL          string        `mapstructure:"base_url"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// AuthConfig configures OAuth2/OIDC authentication
type AuthConfig struct {
	Enabled      bool             `mapstructure:"enabled"`
	Issuer       string           `mapstructure:"issuer"`
	JWKSUrl      string           `mapstructure:"jwks_url"`
	JWKSCacheTTL time.Duration    `mapstructure:"jwks_cache_ttl"`
	Audience     string           `mapstructure:"audience"`
	Claims       AuthClaimsConfig `mapstructure:"claims"`
}

// AuthClaimsConfig configures claim mappings and declarative guard rules.
type AuthClaimsConfig struct {
	Rules    []AuthClaimRule     `mapstructure:"rules"`
	Mappings map[string][]string `mapstructure:"mappings"`
}

// AuthClaimRule configures one declarative claim guard rule.
type AuthClaimRule struct {
	Claim    string   `mapstructure:"claim"`
	Aliases  []string `mapstructure:"aliases"`
	Source   string   `mapstructure:"source"`   // route, header, query
	Key      string   `mapstructure:"key"`      // parameter/header/query key
	Operator string   `mapstructure:"operator"` // required, equals, one_of, regex
	Values   []string `mapstructure:"values"`
	Optional bool     `mapstructure:"optional"`
}

// DatabaseConfig configures database connections
type DatabaseConfig struct {
	Type            string        `mapstructure:"type"` // postgres, mysql, mongodb, dynamodb
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

// CacheConfig configures cache connections
type CacheConfig struct {
	Type             string        `mapstructure:"type"` // redis, inmemory
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// ObjectStorageConfig configures object storage backends.
type ObjectStorageConfig struct {
	Enabled bool                  `mapstructure:"enabled"`
	Type    string                `mapstructure:"type"` // s3
	S3      ObjectStorageS3Config `mapstructure:"s3"`
}

// ObjectStorageS3Config configures S3-compatible object storage.
type ObjectStorageS3Config struct {
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

// SearchConfig configures OpenSearch/Elasticsearch connections.
type SearchConfig struct {
	Type             string        `mapstructure:"type"`   // opensearch, elasticsearch
	Driver           string        `mapstructure:"driver"` // http, opensearch-sdk, elasticsearch-sdk
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

// EventBusConfig configures message broker connections
type EventBusConfig struct {
	Type              string        `mapstructure:"type"` // kafka, rabbitmq, sqs
	Brokers           []string      `mapstructure:"brokers"`
	Serializer        string        `mapstructure:"serializer"` // json, protobuf, avro
	OperationTimeout  time.Duration `mapstructure:"operation_timeout"`
	GroupID           string        `mapstructure:"group_id"`
	URL               string        `mapstructure:"url"`
	Exchange          string        `mapstructure:"exchange"`
	ExchangeType      string        `mapstructure:"exchange_type"`
	QueueName         string        `mapstructure:"queue_name"`
	RoutingKey        string        `mapstructure:"routing_key"`
	ConsumerTag       string        `mapstructure:"consumer_tag"`
	Region            string        `mapstructure:"region"`
	QueueURL          string        `mapstructure:"queue_url"`
	Endpoint          string        `mapstructure:"endpoint"`
	AccessKeyID       string        `mapstructure:"access_key_id"`
	SecretAccessKey   string        `mapstructure:"secret_access_key"`
	SessionToken      string        `mapstructure:"session_token"`
	WaitTimeSeconds   int32         `mapstructure:"wait_time_seconds"`
	MaxMessages       int32         `mapstructure:"max_messages"`
	VisibilityTimeout int32         `mapstructure:"visibility_timeout"`
}

// JobsConfig configures jobs runtime backend selection.
type JobsConfig struct {
	Backend      string           `mapstructure:"backend"`       // eventbus, redis
	DefaultQueue string           `mapstructure:"default_queue"` // default
	Worker       JobsWorkerConfig `mapstructure:"worker"`
	Retry        JobsRetryConfig  `mapstructure:"retry"`
	DLQ          JobsDLQConfig    `mapstructure:"dlq"`
	Redis        JobsRedisConfig  `mapstructure:"redis"`
}

// JobsWorkerConfig configures jobs worker lifecycle.
type JobsWorkerConfig struct {
	Concurrency    int           `mapstructure:"concurrency"`
	LeaseTTL       time.Duration `mapstructure:"lease_ttl"`
	ReserveTimeout time.Duration `mapstructure:"reserve_timeout"`
	StopTimeout    time.Duration `mapstructure:"stop_timeout"`
}

// JobsRetryConfig configures retry strategy for failed jobs.
type JobsRetryConfig struct {
	MaxAttempts    int           `mapstructure:"max_attempts"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
	AttemptTimeout time.Duration `mapstructure:"attempt_timeout"`
}

// JobsDLQConfig configures dead-letter queue behavior.
type JobsDLQConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	QueueSuffix string `mapstructure:"queue_suffix"`
}

// JobsRedisConfig configures Redis backend for jobs runtime.
type JobsRedisConfig struct {
	URL              string        `mapstructure:"url"`
	Prefix           string        `mapstructure:"prefix"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// SchedulerConfig configures distributed scheduler behavior.
type SchedulerConfig struct {
	Enabled         bool                    `mapstructure:"enabled"`
	Timezone        string                  `mapstructure:"timezone"`
	LockProvider    string                  `mapstructure:"lock_provider"` // redis, postgres
	LockTTL         time.Duration           `mapstructure:"lock_ttl"`
	DispatchTimeout time.Duration           `mapstructure:"dispatch_timeout"`
	Redis           SchedulerRedisConfig    `mapstructure:"redis"`
	Postgres        SchedulerPostgresConfig `mapstructure:"postgres"`
	Tasks           []SchedulerTaskConfig   `mapstructure:"tasks"`
}

// SchedulerRedisConfig configures Redis lock provider.
type SchedulerRedisConfig struct {
	URL              string        `mapstructure:"url"`
	Prefix           string        `mapstructure:"prefix"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// SchedulerPostgresConfig configures Postgres lock provider.
type SchedulerPostgresConfig struct {
	URL              string        `mapstructure:"url"`
	Table            string        `mapstructure:"table"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
}

// SchedulerTaskConfig describes one scheduler task from configuration.
type SchedulerTaskConfig struct {
	Name           string            `mapstructure:"name"`
	Cron           string            `mapstructure:"cron"`
	Queue          string            `mapstructure:"queue"`
	JobName        string            `mapstructure:"job_name"`
	Payload        string            `mapstructure:"payload"`
	Headers        map[string]string `mapstructure:"headers"`
	TenantID       string            `mapstructure:"tenant_id"`
	IdempotencyKey string            `mapstructure:"idempotency_key"`
	Timezone       string            `mapstructure:"timezone"`
	LockTTL        time.Duration     `mapstructure:"lock_ttl"`
	MisfirePolicy  string            `mapstructure:"misfire_policy"` // skip, fire_once
}

// ValidationConfig configures strong schema validation for Kafka.
type ValidationConfig struct {
	Kafka KafkaValidationConfig `mapstructure:"kafka"`
}

// KafkaValidationConfig configures local schema validation for Kafka events.
type KafkaValidationConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	Mode           string            `mapstructure:"mode"` // warn, enforce
	DescriptorPath string            `mapstructure:"descriptor_path"`
	DefaultPolicy  string            `mapstructure:"default_policy"` // BACKWARD, FULL
	Subjects       map[string]string `mapstructure:"subjects"`       // subject -> compatibility policy
}

// ObservabilityConfig configures logging, metrics, and tracing
type ObservabilityConfig struct {
	LogLevel          string               `mapstructure:"log_level"`
	LogFormat         string               `mapstructure:"log_format"` // json, text
	ServiceName       string               `mapstructure:"service_name"`
	TracingEnabled    bool                 `mapstructure:"tracing_enabled"`
	TracingSampleRate float64              `mapstructure:"tracing_sample_rate"`
	TracingEndpoint   string               `mapstructure:"tracing_endpoint"`
	AsyncLogging      AsyncLoggingConfig   `mapstructure:"async_logging"`
	RequestLogging    RequestLoggingConfig `mapstructure:"request_logging"`
	RequestTracing    RequestTracingConfig `mapstructure:"request_tracing"`
	RequestTimeout    RequestTimeoutConfig `mapstructure:"request_timeout"`
}

// AsyncLoggingConfig configures optional asynchronous logger dispatching.
type AsyncLoggingConfig struct {
	Enabled      bool `mapstructure:"enabled"`
	QueueSize    int  `mapstructure:"queue_size"`
	WorkerCount  int  `mapstructure:"worker_count"`
	DropWhenFull bool `mapstructure:"drop_when_full"`
}

// RequestLoggingConfig configures HTTP request logging middleware behavior.
type RequestLoggingConfig struct {
	Enabled              bool                   `mapstructure:"enabled"`
	LogStart             bool                   `mapstructure:"log_start"`
	Output               string                 `mapstructure:"output"` // logger, stdout, stderr
	Fields               []string               `mapstructure:"fields"`
	ExcludedPathPrefixes []string               `mapstructure:"excluded_path_prefixes"`
	PathPolicies         []RequestLogPathPolicy `mapstructure:"path_policies"`
}

// RequestLogPathPolicy configures request logging mode for a path prefix.
type RequestLogPathPolicy struct {
	PathPrefix string `mapstructure:"path_prefix"`
	Mode       string `mapstructure:"mode"` // off, minimal, full
}

// RequestTracingConfig configures HTTP tracing middleware behavior.
type RequestTracingConfig struct {
	Enabled              bool                     `mapstructure:"enabled"`
	ExcludedPathPrefixes []string                 `mapstructure:"excluded_path_prefixes"`
	PathPolicies         []RequestTracePathPolicy `mapstructure:"path_policies"`
}

// RequestTracePathPolicy configures tracing mode for a path prefix.
type RequestTracePathPolicy struct {
	PathPrefix string `mapstructure:"path_prefix"`
	Mode       string `mapstructure:"mode"` // off, minimal, full
}

// RequestTimeoutConfig configures HTTP timeout middleware behavior.
type RequestTimeoutConfig struct {
	Enabled              bool                       `mapstructure:"enabled"`
	Default              time.Duration              `mapstructure:"default"`
	ExcludedPathPrefixes []string                   `mapstructure:"excluded_path_prefixes"`
	PathPolicies         []RequestTimeoutPathPolicy `mapstructure:"path_policies"`
}

// RequestTimeoutPathPolicy configures timeout mode for a path prefix.
type RequestTimeoutPathPolicy struct {
	PathPrefix string `mapstructure:"path_prefix"`
	Mode       string `mapstructure:"mode"` // off, on
}

// SwaggerConfig configures API documentation
type SwaggerConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	SpecPath string `mapstructure:"spec_path"`
}

// RateLimitConfig configures the rate limit middleware.
type RateLimitConfig struct {
	Enabled           bool                 `mapstructure:"enabled"`
	Type              string               `mapstructure:"type"` // local, redis (future memcached)
	RequestsPerSecond int                  `mapstructure:"requests_per_second"`
	Burst             int                  `mapstructure:"burst"`
	Window            time.Duration        `mapstructure:"window"`
	Redis             RateLimitRedisConfig `mapstructure:"redis"`
}

// RateLimitRedisConfig configures the Redis-backed rate limiter backend.
type RateLimitRedisConfig struct {
	URL              string        `mapstructure:"url"`
	MaxConns         int           `mapstructure:"max_conns"`
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	Prefix           string        `mapstructure:"prefix"`
}
