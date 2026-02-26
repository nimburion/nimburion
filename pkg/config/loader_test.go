package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify HTTP defaults
	if cfg.RouterType != "gin" {
		t.Errorf("expected router type gin, got %s", cfg.RouterType)
	}
	if cfg.Service.Name != "app" {
		t.Errorf("expected service name app, got %s", cfg.Service.Name)
	}
	if cfg.Service.Environment != "production" {
		t.Errorf("expected service environment production, got %s", cfg.Service.Environment)
	}

	// Verify HTTP defaults
	if cfg.HTTP.Port != 8080 {
		t.Errorf("expected HTTP port 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.HTTP.ReadTimeout != 30*time.Second {
		t.Errorf("expected HTTP read timeout 30s, got %v", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.MaxRequestSize != 1<<20 {
		t.Errorf("expected HTTP max request size 1048576, got %d", cfg.HTTP.MaxRequestSize)
	}

	// Verify Management defaults
	if cfg.Management.Port != 9090 {
		t.Errorf("expected Management port 9090, got %d", cfg.Management.Port)
	}

	// Verify Auth defaults
	if cfg.Auth.Enabled {
		t.Error("expected Auth to be disabled by default")
	}
	if cfg.Auth.JWKSCacheTTL != 1*time.Hour {
		t.Errorf("expected JWKS cache TTL 1h, got %v", cfg.Auth.JWKSCacheTTL)
	}

	// Verify Observability defaults
	if cfg.Observability.LogLevel != "info" {
		t.Errorf("expected log level 'info', got %s", cfg.Observability.LogLevel)
	}
	if cfg.Observability.LogFormat != "json" {
		t.Errorf("expected log format 'json', got %s", cfg.Observability.LogFormat)
	}

	// Verify Swagger defaults
	if cfg.Swagger.Enabled {
		t.Error("expected Swagger to be disabled by default")
	}
	if cfg.Jobs.Backend != JobsBackendEventBus {
		t.Errorf("expected jobs backend %q, got %q", JobsBackendEventBus, cfg.Jobs.Backend)
	}
}

func TestViperLoader_LoadDefaults(t *testing.T) {
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error loading defaults, got: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config to be non-nil")
	}

	// Verify some default values
	if cfg.HTTP.Port != 8080 {
		t.Errorf("expected HTTP port 8080, got %d", cfg.HTTP.Port)
	}
}

func TestViperLoader_LoadWithEnvOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("APP_HTTP_PORT", "9000")
	os.Setenv("APP_HTTP_MAX_REQUEST_SIZE", "2048")
	os.Setenv("APP_OBSERVABILITY_LOG_LEVEL", "debug")
	os.Setenv("APP_ROUTER_TYPE", "gin")
	os.Setenv("APP_SERVICE_NAME", "orders-api")
	os.Setenv("APP_SERVICE_ENVIRONMENT", "production")
	defer func() {
		os.Unsetenv("APP_HTTP_PORT")
		os.Unsetenv("APP_HTTP_MAX_REQUEST_SIZE")
		os.Unsetenv("APP_OBSERVABILITY_LOG_LEVEL")
		os.Unsetenv("APP_ROUTER_TYPE")
		os.Unsetenv("APP_SERVICE_NAME")
		os.Unsetenv("APP_SERVICE_ENVIRONMENT")
	}()

	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify environment variable override
	if cfg.HTTP.Port != 9000 {
		t.Errorf("expected HTTP port 9000 from env, got %d", cfg.HTTP.Port)
	}
	if cfg.HTTP.MaxRequestSize != 2048 {
		t.Errorf("expected HTTP max request size 2048 from env, got %d", cfg.HTTP.MaxRequestSize)
	}
	if cfg.Observability.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from env, got %s", cfg.Observability.LogLevel)
	}
	if cfg.RouterType != "gin" {
		t.Errorf("expected router type 'gin' from env, got %s", cfg.RouterType)
	}
	if cfg.Service.Name != "orders-api" {
		t.Errorf("expected service name orders-api from env, got %s", cfg.Service.Name)
	}
	if cfg.Service.Environment != "production" {
		t.Errorf("expected service environment production from env, got %s", cfg.Service.Environment)
	}
}

func TestViperLoader_LoadSecurityHeadersFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SECURITY_HEADERS_ENABLED", "true")
	os.Setenv("APP_SECURITY_HEADERS_SSL_REDIRECT", "true")
	os.Setenv("APP_SECURITY_HEADERS_STS_SECONDS", "63072000")

	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !cfg.SecurityHeaders.Enabled {
		t.Fatalf("expected security_headers.enabled=true from env")
	}
	if !cfg.SecurityHeaders.SSLRedirect {
		t.Fatalf("expected security_headers.ssl_redirect=true from env")
	}
	if cfg.SecurityHeaders.STSSeconds != 63072000 {
		t.Fatalf("expected security_headers.sts_seconds=63072000 from env, got %d", cfg.SecurityHeaders.STSSeconds)
	}
}

func TestViperLoader_InvalidRouterType(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_ROUTER_TYPE", "invalid")
	defer os.Unsetenv("APP_ROUTER_TYPE")

	loader := NewViperLoader("", "APP")
	_, err := loader.Load()
	if err == nil {
		t.Fatal("expected error for invalid router type")
	}
	if !strings.Contains(err.Error(), "invalid router_type") {
		t.Fatalf("expected invalid router_type error, got %v", err)
	}
}

func TestViperLoader_InvalidSecurityHeadersSTSSeconds(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SECURITY_HEADERS_STS_SECONDS", "-1")
	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected error for negative security_headers.sts_seconds")
	}
	if !strings.Contains(err.Error(), "security_headers.sts_seconds cannot be negative") {
		t.Fatalf("expected security_headers.sts_seconds validation error, got %v", err)
	}
}

func TestViperLoader_InvalidHTTPMaxRequestSize(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_HTTP_MAX_REQUEST_SIZE", "-1")
	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected error for negative http.max_request_size")
	}
	if !strings.Contains(err.Error(), "http.max_request_size cannot be negative") {
		t.Fatalf("expected http.max_request_size validation error, got %v", err)
	}
}

func TestViperLoader_LoadI18nFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_I18N_ENABLED", "true")
	os.Setenv("APP_I18N_DEFAULT_LOCALE", "en")
	os.Setenv("APP_I18N_SUPPORTED_LOCALES", "en,it")
	os.Setenv("APP_I18N_QUERY_PARAM", "lang")
	os.Setenv("APP_I18N_HEADER_NAME", "X-Locale")
	os.Setenv("APP_I18N_FALLBACK_MODE", "base")
	os.Setenv("APP_I18N_CATALOG_PATH", "./i18n")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.I18n.Enabled {
		t.Fatal("expected i18n.enabled=true")
	}
	if cfg.I18n.DefaultLocale != "en" {
		t.Fatalf("expected i18n.default_locale=en, got %q", cfg.I18n.DefaultLocale)
	}
	if len(cfg.I18n.SupportedLocales) != 2 {
		t.Fatalf("expected two supported locales, got %v", cfg.I18n.SupportedLocales)
	}
}

func TestViperLoader_LoadValidationFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_VALIDATION_KAFKA_ENABLED", "true")
	os.Setenv("APP_VALIDATION_KAFKA_MODE", "warn")
	os.Setenv("APP_VALIDATION_KAFKA_DESCRIPTOR_PATH", "events/proto/schema.pb")
	os.Setenv("APP_VALIDATION_KAFKA_DEFAULT_POLICY", "FULL")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !cfg.Validation.Kafka.Enabled {
		t.Fatal("expected validation.kafka.enabled=true")
	}
	if cfg.Validation.Kafka.Mode != "warn" {
		t.Fatalf("expected validation.kafka.mode=warn, got %q", cfg.Validation.Kafka.Mode)
	}
	if cfg.Validation.Kafka.DefaultPolicy != "FULL" {
		t.Fatalf("expected validation.kafka.default_policy=FULL, got %q", cfg.Validation.Kafka.DefaultPolicy)
	}
}

func TestViperLoader_InvalidValidationKafkaDefaultPolicy(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_VALIDATION_KAFKA_DEFAULT_POLICY", "INVALID")
	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected error for invalid validation.kafka.default_policy")
	}
	if !strings.Contains(err.Error(), "invalid validation.kafka.default_policy") {
		t.Fatalf("expected validation.kafka.default_policy validation error, got %v", err)
	}
}

func TestViperLoader_LoadJobsFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_JOBS_BACKEND", "eventbus")
	os.Setenv("APP_JOBS_DEFAULT_QUEUE", "critical")
	os.Setenv("APP_JOBS_WORKER_CONCURRENCY", "4")
	os.Setenv("APP_JOBS_WORKER_LEASE_TTL", "45s")
	os.Setenv("APP_JOBS_WORKER_RESERVE_TIMEOUT", "2s")
	os.Setenv("APP_JOBS_WORKER_STOP_TIMEOUT", "12s")
	os.Setenv("APP_JOBS_RETRY_MAX_ATTEMPTS", "9")
	os.Setenv("APP_JOBS_RETRY_INITIAL_BACKOFF", "500ms")
	os.Setenv("APP_JOBS_RETRY_MAX_BACKOFF", "30s")
	os.Setenv("APP_JOBS_RETRY_ATTEMPT_TIMEOUT", "20s")
	os.Setenv("APP_JOBS_DLQ_ENABLED", "true")
	os.Setenv("APP_JOBS_DLQ_QUEUE_SUFFIX", ".dead")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Jobs.Backend != "eventbus" {
		t.Fatalf("expected jobs.backend=eventbus, got %q", cfg.Jobs.Backend)
	}
	if cfg.Jobs.DefaultQueue != "critical" {
		t.Fatalf("expected jobs.default_queue=critical, got %q", cfg.Jobs.DefaultQueue)
	}
	if cfg.Jobs.Worker.Concurrency != 4 {
		t.Fatalf("expected jobs.worker.concurrency=4, got %d", cfg.Jobs.Worker.Concurrency)
	}
	if cfg.Jobs.Worker.LeaseTTL != 45*time.Second {
		t.Fatalf("expected jobs.worker.lease_ttl=45s, got %v", cfg.Jobs.Worker.LeaseTTL)
	}
	if cfg.Jobs.Retry.MaxAttempts != 9 {
		t.Fatalf("expected jobs.retry.max_attempts=9, got %d", cfg.Jobs.Retry.MaxAttempts)
	}
	if cfg.Jobs.Retry.InitialBackoff != 500*time.Millisecond {
		t.Fatalf("expected jobs.retry.initial_backoff=500ms, got %v", cfg.Jobs.Retry.InitialBackoff)
	}
	if cfg.Jobs.DLQ.QueueSuffix != ".dead" {
		t.Fatalf("expected jobs.dlq.queue_suffix=.dead, got %q", cfg.Jobs.DLQ.QueueSuffix)
	}
}

func TestViperLoader_InvalidJobsBackend(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_JOBS_BACKEND", "invalid")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for invalid jobs.backend")
	}
	if !strings.Contains(err.Error(), "invalid jobs.backend") {
		t.Fatalf("expected jobs.backend validation error, got %v", err)
	}
}

func TestViperLoader_InvalidJobsWorkerConfig(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_JOBS_WORKER_CONCURRENCY", "0")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for jobs.worker.concurrency")
	}
	if !strings.Contains(err.Error(), "jobs.worker.concurrency must be greater than zero") {
		t.Fatalf("expected jobs.worker.concurrency validation error, got %v", err)
	}
}

func TestViperLoader_LoadJobsRedisFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_JOBS_BACKEND", "redis")
	os.Setenv("APP_JOBS_REDIS_URL", "redis://localhost:6379/1")
	os.Setenv("APP_JOBS_REDIS_PREFIX", "jobs:test")
	os.Setenv("APP_JOBS_REDIS_OPERATION_TIMEOUT", "7s")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Jobs.Backend != "redis" {
		t.Fatalf("expected jobs.backend=redis, got %q", cfg.Jobs.Backend)
	}
	if cfg.Jobs.Redis.URL != "redis://localhost:6379/1" {
		t.Fatalf("expected jobs.redis.url to be loaded from env, got %q", cfg.Jobs.Redis.URL)
	}
	if cfg.Jobs.Redis.Prefix != "jobs:test" {
		t.Fatalf("expected jobs.redis.prefix=jobs:test, got %q", cfg.Jobs.Redis.Prefix)
	}
	if cfg.Jobs.Redis.OperationTimeout != 7*time.Second {
		t.Fatalf("expected jobs.redis.operation_timeout=7s, got %v", cfg.Jobs.Redis.OperationTimeout)
	}
}

func TestViperLoader_InvalidJobsRedisConfig(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_JOBS_BACKEND", "redis")
	os.Setenv("APP_JOBS_REDIS_PREFIX", "jobs:test")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for missing jobs.redis.url")
	}
	if !strings.Contains(err.Error(), "jobs.redis.url is required when jobs.backend is redis") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadSchedulerFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SCHEDULER_ENABLED", "true")
	os.Setenv("APP_SCHEDULER_TIMEZONE", "Europe/Rome")
	os.Setenv("APP_SCHEDULER_LOCK_PROVIDER", "postgres")
	os.Setenv("APP_SCHEDULER_LOCK_TTL", "30s")
	os.Setenv("APP_SCHEDULER_DISPATCH_TIMEOUT", "9s")
	os.Setenv("APP_DATABASE_URL", "postgres://localhost:5432/app?sslmode=disable")
	os.Setenv("APP_SCHEDULER_POSTGRES_TABLE", "scheduler_locks")
	os.Setenv("APP_SCHEDULER_POSTGRES_OPERATION_TIMEOUT", "4s")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.Scheduler.Enabled {
		t.Fatal("expected scheduler.enabled=true")
	}
	if cfg.Scheduler.Timezone != "Europe/Rome" {
		t.Fatalf("expected scheduler.timezone=Europe/Rome, got %q", cfg.Scheduler.Timezone)
	}
	if cfg.Scheduler.LockProvider != "postgres" {
		t.Fatalf("expected scheduler.lock_provider=postgres, got %q", cfg.Scheduler.LockProvider)
	}
	if cfg.Scheduler.LockTTL != 30*time.Second {
		t.Fatalf("expected scheduler.lock_ttl=30s, got %v", cfg.Scheduler.LockTTL)
	}
	if cfg.Scheduler.DispatchTimeout != 9*time.Second {
		t.Fatalf("expected scheduler.dispatch_timeout=9s, got %v", cfg.Scheduler.DispatchTimeout)
	}
	if cfg.Scheduler.Postgres.Table != "scheduler_locks" {
		t.Fatalf("expected scheduler.postgres.table=scheduler_locks, got %q", cfg.Scheduler.Postgres.Table)
	}
	if cfg.Scheduler.Postgres.OperationTimeout != 4*time.Second {
		t.Fatalf("expected scheduler.postgres.operation_timeout=4s, got %v", cfg.Scheduler.Postgres.OperationTimeout)
	}
}

func TestViperLoader_InvalidI18nDefaultLocale(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_I18N_ENABLED", "true")
	os.Setenv("APP_I18N_DEFAULT_LOCALE", "fr")
	os.Setenv("APP_I18N_SUPPORTED_LOCALES", "en,it")
	os.Setenv("APP_I18N_FALLBACK_MODE", "base")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for invalid i18n default locale")
	}
	if !strings.Contains(err.Error(), "i18n.default_locale must be included in i18n.supported_locales") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadEmailFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_EMAIL_ENABLED", "true")
	os.Setenv("APP_EMAIL_PROVIDER", "sendgrid")
	os.Setenv("APP_EMAIL_SENDGRID_TOKEN", "sg-token")
	os.Setenv("APP_EMAIL_SENDGRID_FROM", "no-reply@example.com")
	os.Setenv("APP_EMAIL_SENDGRID_BASE_URL", "https://api.sendgrid.com")
	os.Setenv("APP_EMAIL_SENDGRID_OPERATION_TIMEOUT", "12s")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.Email.Enabled {
		t.Fatal("expected email.enabled=true")
	}
	if cfg.Email.Provider != "sendgrid" {
		t.Fatalf("expected email.provider=sendgrid, got %s", cfg.Email.Provider)
	}
	if cfg.Email.SendGrid.Token != "sg-token" {
		t.Fatalf("expected email.sendgrid.token to be loaded from env")
	}
	if cfg.Email.SendGrid.From != "no-reply@example.com" {
		t.Fatalf("expected email.sendgrid.from to be loaded from env")
	}
	if cfg.Email.SendGrid.OperationTimeout != 12*time.Second {
		t.Fatalf("expected email.sendgrid.operation_timeout=12s, got %v", cfg.Email.SendGrid.OperationTimeout)
	}
}

func TestViperLoader_InvalidEmailConfig(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_EMAIL_ENABLED", "true")
	os.Setenv("APP_EMAIL_PROVIDER", "postmark")
	// Missing APP_EMAIL_POSTMARK_SERVER_TOKEN on purpose.

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for missing postmark server token")
	}
	if !strings.Contains(err.Error(), "email.postmark.server_token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadHTTPSignatureFromFile(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	content := `
security:
  http_signature:
    enabled: true
    key_id_header: "X-Key-Id"
    timestamp_header: "X-Timestamp"
    nonce_header: "X-Nonce"
    signature_header: "X-Signature"
    max_clock_skew: 2m
    nonce_ttl: 15m
    require_nonce: true
    excluded_path_prefixes: ["/health", "/metrics"]
    static_keys:
      device-a: "secret-a"
`

	tmpFile, err := os.CreateTemp("", "nimburion-config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write temp config file: %v", err)
	}
	_ = tmpFile.Close()

	cfg, err := NewViperLoader(tmpFile.Name(), "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.Security.HTTPSignature.Enabled {
		t.Fatal("expected security.http_signature.enabled=true")
	}
	if cfg.Security.HTTPSignature.MaxClockSkew != 2*time.Minute {
		t.Fatalf("expected max_clock_skew=2m, got %v", cfg.Security.HTTPSignature.MaxClockSkew)
	}
	if cfg.Security.HTTPSignature.StaticKeys["device-a"] != "secret-a" {
		t.Fatalf("expected static key for device-a to be loaded")
	}
}

func TestViperLoader_InvalidHTTPSignatureMissingStaticKeys(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SECURITY_HTTP_SIGNATURE_ENABLED", "true")
	defer os.Unsetenv("APP_SECURITY_HTTP_SIGNATURE_ENABLED")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for missing security.http_signature.static_keys")
	}
	if !strings.Contains(err.Error(), "security.http_signature.static_keys must contain at least one key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadObjectStorageFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBJECT_STORAGE_ENABLED", "true")
	os.Setenv("APP_OBJECT_STORAGE_TYPE", "s3")
	os.Setenv("APP_OBJECT_STORAGE_S3_BUCKET", "documents")
	os.Setenv("APP_OBJECT_STORAGE_S3_REGION", "eu-west-1")
	os.Setenv("APP_OBJECT_STORAGE_S3_ENDPOINT", "http://localhost:9000")
	os.Setenv("APP_OBJECT_STORAGE_S3_USE_PATH_STYLE", "true")
	os.Setenv("APP_OBJECT_STORAGE_S3_OPERATION_TIMEOUT", "7s")
	os.Setenv("APP_OBJECT_STORAGE_S3_PRESIGN_EXPIRY", "20m")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.ObjectStorage.Enabled {
		t.Fatal("expected object_storage.enabled=true")
	}
	if cfg.ObjectStorage.Type != "s3" {
		t.Fatalf("expected object_storage.type=s3, got %q", cfg.ObjectStorage.Type)
	}
	if cfg.ObjectStorage.S3.Bucket != "documents" {
		t.Fatalf("expected object_storage.s3.bucket=documents, got %q", cfg.ObjectStorage.S3.Bucket)
	}
	if cfg.ObjectStorage.S3.OperationTimeout != 7*time.Second {
		t.Fatalf("expected object_storage.s3.operation_timeout=7s, got %v", cfg.ObjectStorage.S3.OperationTimeout)
	}
	if cfg.ObjectStorage.S3.PresignExpiry != 20*time.Minute {
		t.Fatalf("expected object_storage.s3.presign_expiry=20m, got %v", cfg.ObjectStorage.S3.PresignExpiry)
	}
}

func TestViperLoader_InvalidObjectStorageS3MissingBucket(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBJECT_STORAGE_ENABLED", "true")
	os.Setenv("APP_OBJECT_STORAGE_TYPE", "s3")
	os.Setenv("APP_OBJECT_STORAGE_S3_REGION", "eu-west-1")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for missing object_storage.s3.bucket")
	}
	if !strings.Contains(err.Error(), "object_storage.s3.bucket is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_SessionAndCSRFValidation(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_CSRF_ENABLED", "true")
	_, err := NewViperLoader("", "APP").Load()
	if err == nil || !strings.Contains(err.Error(), "session.enabled must be true when csrf.enabled is true") {
		t.Fatalf("expected csrf/session validation error, got %v", err)
	}
}

func TestViperLoader_LoadSessionFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SESSION_ENABLED", "true")
	os.Setenv("APP_SESSION_STORE", "inmemory")
	os.Setenv("APP_SESSION_COOKIE_NAME", "my_sid")
	os.Setenv("APP_SESSION_TTL", "6h")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.Session.Enabled {
		t.Fatalf("expected session.enabled=true from env")
	}
	if cfg.Session.CookieName != "my_sid" {
		t.Fatalf("expected session cookie name from env, got %q", cfg.Session.CookieName)
	}
	if cfg.Session.TTL != 6*time.Hour {
		t.Fatalf("expected session.ttl=6h, got %v", cfg.Session.TTL)
	}
}

func TestViperLoader_LoadMgmtPortFromAbbreviatedEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_MGMT_PORT", "9191")
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if cfg.Management.Port != 9191 {
		t.Fatalf("expected management port 9191, got %d", cfg.Management.Port)
	}
}

func TestViperLoader_LoadMgmtPortFromLegacyEnvAlias(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_MANAGEMENT_PORT", "9292")
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if cfg.Management.Port != 9292 {
		t.Fatalf("expected management port 9292 from legacy alias, got %d", cfg.Management.Port)
	}
}

func TestViperLoader_AbbreviatedMgmtTakesPrecedenceOverLegacy(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_MANAGEMENT_PORT", "9393")
	os.Setenv("APP_MGMT_PORT", "9494")
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if cfg.Management.Port != 9494 {
		t.Fatalf("expected abbreviated APP_MGMT_PORT to win, got %d", cfg.Management.Port)
	}
}

func TestViperLoader_ValidationErrorMessages(t *testing.T) {
	t.Run("missing auth issuer message", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_AUTH_ENABLED", "true")
		_, err := NewViperLoader("", "APP").Load()
		if err == nil || !strings.Contains(err.Error(), "auth.issuer is required") {
			t.Fatalf("expected missing auth.issuer message, got %v", err)
		}
	})

	t.Run("auth requires claim rules", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_AUTH_ENABLED", "true")
		os.Setenv("APP_AUTH_ISSUER", "https://issuer.example.com/")
		os.Setenv("APP_AUTH_JWKS_URL", "https://issuer.example.com/.well-known/jwks.json")
		os.Setenv("APP_AUTH_AUDIENCE", "api")
		_, err := NewViperLoader("", "APP").Load()
		if err == nil || !strings.Contains(err.Error(), "auth.claims.rules must contain at least one rule when auth is enabled") {
			t.Fatalf("expected auth.claims.rules validation error, got %v", err)
		}
	})

	t.Run("invalid management port message", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_MGMT_PORT", "70000")
		_, err := NewViperLoader("", "APP").Load()
		if err == nil || !strings.Contains(err.Error(), "invalid management.port") {
			t.Fatalf("expected invalid management.port message, got %v", err)
		}
	})

	t.Run("http and management same port message", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_HTTP_PORT", "8080")
		os.Setenv("APP_MGMT_PORT", "8080")
		_, err := NewViperLoader("", "APP").Load()
		if err == nil || !strings.Contains(err.Error(), "http.port and management.port must be different") {
			t.Fatalf("expected same-port validation message, got %v", err)
		}
	})

	t.Run("http and management same port allowed when management disabled", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_HTTP_PORT", "8080")
		os.Setenv("APP_MGMT_ENABLED", "false")
		os.Setenv("APP_MGMT_PORT", "8080")
		_, err := NewViperLoader("", "APP").Load()
		if err != nil {
			t.Fatalf("expected no validation error when management is disabled, got %v", err)
		}
	})

	t.Run("management auth requires global auth", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_MGMT_AUTH_ENABLED", "true")
		_, err := NewViperLoader("", "APP").Load()
		if err == nil || !strings.Contains(err.Error(), "auth.enabled must be true when management.auth_enabled is true") {
			t.Fatalf("expected management auth dependency validation error, got %v", err)
		}
	})

	t.Run("management mtls requires tls files", func(t *testing.T) {
		clearAppEnv()
		defer clearAppEnv()
		os.Setenv("APP_MGMT_MTLS_ENABLED", "true")
		_, err := NewViperLoader("", "APP").Load()
		if err == nil {
			t.Fatal("expected error for missing management TLS files, got nil")
		}
		for _, msg := range []string{
			"management.tls_cert_file is required when management.mtls_enabled is true",
			"management.tls_key_file is required when management.mtls_enabled is true",
			"management.tls_ca_file is required when management.mtls_enabled is true",
		} {
			if !strings.Contains(err.Error(), msg) {
				t.Fatalf("expected error to contain %q, got %v", msg, err)
			}
		}
	})
}

func TestViperLoader_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		cleanupEnv  func()
		expectError bool
		errorMsg    string
	}{
		{
			name: "auth enabled without issuer",
			setupEnv: func() {
				os.Setenv("APP_AUTH_ENABLED", "true")
			},
			cleanupEnv: func() {
				os.Unsetenv("APP_AUTH_ENABLED")
			},
			expectError: true,
			errorMsg:    "auth.issuer is required",
		},
		{
			name: "invalid log level",
			setupEnv: func() {
				os.Setenv("APP_AUTH_ENABLED", "false")
				os.Setenv("APP_OBSERVABILITY_LOG_LEVEL", "invalid")
			},
			cleanupEnv: func() {
				os.Unsetenv("APP_AUTH_ENABLED")
				os.Unsetenv("APP_OBSERVABILITY_LOG_LEVEL")
			},
			expectError: true,
			errorMsg:    "invalid observability.log_level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all APP_ environment variables before each test
			for _, env := range os.Environ() {
				if strings.HasPrefix(env, "APP_") {
					key := strings.Split(env, "=")[0]
					os.Unsetenv(key)
				}
			}

			tt.setupEnv()
			defer tt.cleanupEnv()

			loader := NewViperLoader("", "APP")
			_, err := loader.Load()

			if tt.expectError && err == nil {
				t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			}
		})
	}
}

func TestViperLoader_ValidConfiguration(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_HTTP_PORT", "18080")
	os.Setenv("APP_OBSERVABILITY_LOG_LEVEL", "info")
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error with valid config, got: %v", err)
	}

	// Verify configuration was loaded correctly
	if cfg.HTTP.Port != 18080 {
		t.Errorf("expected HTTP port 18080, got %d", cfg.HTTP.Port)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"item exists", []string{"a", "b", "c"}, "b", true},
		{"item does not exist", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Property-Based Tests

// TestProperty8_ConfigurationPrecedence tests that configuration precedence
// follows the order: ENV > file > defaults
// **Property 8: Configuration Precedence**
// **Validates: Requirements 10.1**
func TestProperty8_ConfigurationPrecedence(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Generator for configuration values
	genPort := gen.IntRange(1024, 65535)
	genLogLevel := gen.OneConstOf("debug", "info", "warn", "error")
	genTimeout := gen.IntRange(1, 300).Map(func(seconds int) time.Duration {
		return time.Duration(seconds) * time.Second
	})

	properties.Property("ENV overrides file and defaults", prop.ForAll(
		func(envPort, filePort int, envLogLevel, fileLogLevel string, envTimeout, fileTimeout time.Duration) bool {
			// Clear all APP_ environment variables
			clearAppEnv()
			defer clearAppEnv()

			// Create a temporary config file
			configFile := createTempConfigFile(t, map[string]interface{}{
				"http": map[string]interface{}{
					"port":         filePort,
					"read_timeout": fileTimeout.String(),
				},
				"observability": map[string]interface{}{
					"log_level": fileLogLevel,
				},
			})
			defer os.Remove(configFile)

			// Set environment variables (should override file)
			os.Setenv("APP_HTTP_PORT", fmt.Sprintf("%d", envPort))
			os.Setenv("APP_HTTP_READ_TIMEOUT", envTimeout.String())
			os.Setenv("APP_OBSERVABILITY_LOG_LEVEL", envLogLevel)

			// Load configuration
			loader := NewViperLoader(configFile, "APP")
			cfg, err := loader.Load()

			if err != nil {
				t.Logf("Load error: %v", err)
				return false
			}

			// Verify ENV values take precedence
			if cfg.HTTP.Port != envPort {
				t.Logf("Expected HTTP port %d from ENV, got %d", envPort, cfg.HTTP.Port)
				return false
			}
			if cfg.HTTP.ReadTimeout != envTimeout {
				t.Logf("Expected HTTP read timeout %v from ENV, got %v", envTimeout, cfg.HTTP.ReadTimeout)
				return false
			}
			if cfg.Observability.LogLevel != envLogLevel {
				t.Logf("Expected log level %s from ENV, got %s", envLogLevel, cfg.Observability.LogLevel)
				return false
			}

			return true
		},
		genPort,
		genPort,
		genLogLevel,
		genLogLevel,
		genTimeout,
		genTimeout,
	))

	properties.Property("File overrides defaults when ENV not set", prop.ForAll(
		func(filePort int, fileLogLevel string, fileTimeout time.Duration) bool {
			// Clear all APP_ environment variables
			clearAppEnv()
			defer clearAppEnv()

			// Get default values
			defaults := DefaultConfig()

			// Create a temporary config file with different values than defaults
			configFile := createTempConfigFile(t, map[string]interface{}{
				"http": map[string]interface{}{
					"port":         filePort,
					"read_timeout": fileTimeout.String(),
				},
				"observability": map[string]interface{}{
					"log_level": fileLogLevel,
				},
			})
			defer os.Remove(configFile)

			// Load configuration (no ENV vars set)
			loader := NewViperLoader(configFile, "APP")
			cfg, err := loader.Load()

			if err != nil {
				t.Logf("Load error: %v", err)
				return false
			}

			// Verify file values take precedence over defaults
			if cfg.HTTP.Port != filePort {
				t.Logf("Expected HTTP port %d from file, got %d", filePort, cfg.HTTP.Port)
				return false
			}
			if cfg.HTTP.ReadTimeout != fileTimeout {
				t.Logf("Expected HTTP read timeout %v from file, got %v", fileTimeout, cfg.HTTP.ReadTimeout)
				return false
			}
			if cfg.Observability.LogLevel != fileLogLevel {
				t.Logf("Expected log level %s from file, got %s", fileLogLevel, cfg.Observability.LogLevel)
				return false
			}

			// Verify other values still use defaults
			if cfg.Management.Port != defaults.Management.Port {
				t.Logf("Expected Management port %d from defaults, got %d", defaults.Management.Port, cfg.Management.Port)
				return false
			}

			return true
		},
		genPort,
		genLogLevel,
		genTimeout,
	))

	properties.Property("Defaults used when no file or ENV", prop.ForAll(
		func() bool {
			// Clear all APP_ environment variables
			clearAppEnv()
			defer clearAppEnv()

			// Get default values
			defaults := DefaultConfig()

			// Load configuration (no file, no ENV vars)
			loader := NewViperLoader("", "APP")
			cfg, err := loader.Load()

			if err != nil {
				t.Logf("Load error: %v", err)
				return false
			}

			// Verify all values match defaults
			if cfg.HTTP.Port != defaults.HTTP.Port {
				t.Logf("Expected HTTP port %d from defaults, got %d", defaults.HTTP.Port, cfg.HTTP.Port)
				return false
			}
			if cfg.HTTP.ReadTimeout != defaults.HTTP.ReadTimeout {
				t.Logf("Expected HTTP read timeout %v from defaults, got %v", defaults.HTTP.ReadTimeout, cfg.HTTP.ReadTimeout)
				return false
			}
			if cfg.Management.Port != defaults.Management.Port {
				t.Logf("Expected Management port %d from defaults, got %d", defaults.Management.Port, cfg.Management.Port)
				return false
			}
			if cfg.Observability.LogLevel != defaults.Observability.LogLevel {
				t.Logf("Expected log level %s from defaults, got %s", defaults.Observability.LogLevel, cfg.Observability.LogLevel)
				return false
			}

			return true
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_EnvPrecedenceAcrossSections(t *testing.T) {
	properties := gopter.NewProperties(nil)

	genPort := gen.IntRange(1024, 65000)
	genBool := gen.Bool()

	properties.Property("ENV overrides file values across major sections", prop.ForAll(
		func(httpPort, mgmtPort, dbMaxConns int, swaggerEnabled bool) bool {
			if httpPort == mgmtPort {
				mgmtPort++
			}
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempConfigFile(t, map[string]interface{}{
				"http": map[string]interface{}{
					"port": httpPort + 1,
				},
				"management": map[string]interface{}{
					"port": mgmtPort + 1,
				},
				"database": map[string]interface{}{
					"max_open_conns": dbMaxConns + 1,
				},
				"swagger": map[string]interface{}{
					"enabled": !swaggerEnabled,
				},
			})
			defer os.Remove(configFile)

			os.Setenv("APP_HTTP_PORT", fmt.Sprintf("%d", httpPort))
			os.Setenv("APP_MGMT_PORT", fmt.Sprintf("%d", mgmtPort))
			os.Setenv("APP_DB_MAX_OPEN_CONNS", fmt.Sprintf("%d", dbMaxConns))
			os.Setenv("APP_SWAGGER_ENABLED", fmt.Sprintf("%t", swaggerEnabled))

			cfg, err := NewViperLoader(configFile, "APP").Load()
			if err != nil {
				t.Logf("load error: %v", err)
				return false
			}

			return cfg.HTTP.Port == httpPort &&
				cfg.Management.Port == mgmtPort &&
				cfg.Database.MaxOpenConns == dbMaxConns &&
				cfg.Swagger.Enabled == swaggerEnabled
		},
		genPort,
		genPort,
		gen.IntRange(1, 100),
		genBool,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_LegacyEnvVariablesWorkAsAliases(t *testing.T) {
	properties := gopter.NewProperties(nil)
	properties.Property("legacy APP_MANAGEMENT_PORT maps to management.port", prop.ForAll(
		func(port int) bool {
			if port < 1024 || port > 65000 {
				return true
			}
			clearAppEnv()
			defer clearAppEnv()
			os.Setenv("APP_MANAGEMENT_PORT", fmt.Sprintf("%d", port))
			cfg, err := NewViperLoader("", "APP").Load()
			return err == nil && cfg.Management.Port == port
		},
		gen.IntRange(1024, 65000),
	))

	properties.Property("legacy APP_DATABASE_MAX_OPEN_CONNS maps to database.max_open_conns", prop.ForAll(
		func(v int) bool {
			if v < 1 || v > 500 {
				return true
			}
			clearAppEnv()
			defer clearAppEnv()
			os.Setenv("APP_DATABASE_MAX_OPEN_CONNS", fmt.Sprintf("%d", v))
			cfg, err := NewViperLoader("", "APP").Load()
			return err == nil && cfg.Database.MaxOpenConns == v
		},
		gen.IntRange(1, 500),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_AbbreviatedEnvTakesPrecedenceOverLegacy(t *testing.T) {
	properties := gopter.NewProperties(nil)
	properties.Property("APP_MGMT_PORT wins over APP_MANAGEMENT_PORT", prop.ForAll(
		func(legacy, abbrev int) bool {
			if legacy < 1024 || legacy > 65000 || abbrev < 1024 || abbrev > 65000 || legacy == abbrev {
				return true
			}
			clearAppEnv()
			defer clearAppEnv()
			os.Setenv("APP_MANAGEMENT_PORT", fmt.Sprintf("%d", legacy))
			os.Setenv("APP_MGMT_PORT", fmt.Sprintf("%d", abbrev))
			cfg, err := NewViperLoader("", "APP").Load()
			return err == nil && cfg.Management.Port == abbrev
		},
		gen.IntRange(1024, 65000),
		gen.IntRange(1024, 65000),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to clear all APP_ environment variables
func clearAppEnv() {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "APP_") {
			key := strings.Split(env, "=")[0]
			os.Unsetenv(key)
		}
	}
}

// Helper function to create a temporary config file
func createTempConfigFile(t *testing.T, config map[string]interface{}) string {
	t.Helper()

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Write YAML content
	var content strings.Builder
	writeYAML(&content, config, 0)

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write config file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Helper function to write YAML content recursively
func writeYAML(w *strings.Builder, data map[string]interface{}, indent int) {
	indentStr := strings.Repeat("  ", indent)
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			w.WriteString(fmt.Sprintf("%s%s:\n", indentStr, key))
			writeYAML(w, v, indent+1)
		default:
			w.WriteString(fmt.Sprintf("%s%s: %v\n", indentStr, key, v))
		}
	}
}

// TestProperty9_ConfigurationFormatSupport tests that configuration can be loaded
// from YAML, JSON, and TOML formats and produces equivalent Config structs
// **Property 9: Configuration Format Support**
// **Validates: Requirements 10.5**
func TestProperty9_ConfigurationFormatSupport(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Generator for configuration values
	genPort := gen.IntRange(1024, 65535)
	genLogLevel := gen.OneConstOf("debug", "info", "warn", "error")
	genLogFormat := gen.OneConstOf("json", "text")
	genTimeout := gen.IntRange(1, 300).Map(func(seconds int) time.Duration {
		return time.Duration(seconds) * time.Second
	})
	genBool := gen.Bool()

	properties.Property("YAML, JSON, and TOML produce equivalent configs", prop.ForAll(
		func(httpPort, mgmtPort int, logLevel, logFormat string,
			readTimeout, writeTimeout time.Duration, swaggerEnabled bool) bool {

			// Ensure ports are different
			if httpPort == mgmtPort {
				mgmtPort = httpPort + 1
				if mgmtPort > 65535 {
					mgmtPort = httpPort - 1
				}
			}

			// Clear all APP_ environment variables
			clearAppEnv()
			defer clearAppEnv()

			// Create the same configuration in different formats
			configData := map[string]interface{}{
				"http": map[string]interface{}{
					"port":          httpPort,
					"read_timeout":  readTimeout.String(),
					"write_timeout": writeTimeout.String(),
					"idle_timeout":  "120s",
				},
				"management": map[string]interface{}{
					"port":          mgmtPort,
					"read_timeout":  "10s",
					"write_timeout": "10s",
					"auth_enabled":  false,
					"mtls_enabled":  false,
				},
				"auth": map[string]interface{}{
					"enabled":        false,
					"issuer":         "https://auth.example.com",
					"jwks_url":       "https://auth.example.com/.well-known/jwks.json",
					"audience":       "nimburion-service",
					"jwks_cache_ttl": "1h",
				},
				"database": map[string]interface{}{
					"max_open_conns":    25,
					"max_idle_conns":    5,
					"conn_max_lifetime": "5m",
					"query_timeout":     "10s",
				},
				"cache": map[string]interface{}{
					"max_conns":         10,
					"operation_timeout": "5s",
				},
				"eventbus": map[string]interface{}{
					"serializer":        "json",
					"operation_timeout": "30s",
				},
				"observability": map[string]interface{}{
					"log_level":           logLevel,
					"log_format":          logFormat,
					"tracing_enabled":     false,
					"tracing_sample_rate": 0.1,
				},
				"swagger": map[string]interface{}{
					"enabled":   swaggerEnabled,
					"spec_path": "/api/openapi/openapi.yaml",
				},
			}

			// Create config files in different formats
			yamlFile := createTempYAMLFile(t, configData)
			defer os.Remove(yamlFile)

			jsonFile := createTempJSONFile(t, configData)
			defer os.Remove(jsonFile)

			tomlFile := createTempTOMLFile(t, configData)
			defer os.Remove(tomlFile)

			// Load configuration from each format
			yamlLoader := NewViperLoader(yamlFile, "APP")
			yamlCfg, yamlErr := yamlLoader.Load()

			jsonLoader := NewViperLoader(jsonFile, "APP")
			jsonCfg, jsonErr := jsonLoader.Load()

			tomlLoader := NewViperLoader(tomlFile, "APP")
			tomlCfg, tomlErr := tomlLoader.Load()

			// All should load without error
			if yamlErr != nil {
				t.Logf("YAML load error: %v", yamlErr)
				return false
			}
			if jsonErr != nil {
				t.Logf("JSON load error: %v", jsonErr)
				return false
			}
			if tomlErr != nil {
				t.Logf("TOML load error: %v", tomlErr)
				return false
			}

			// Compare configurations - they should be equivalent
			if !configsEqual(yamlCfg, jsonCfg) {
				t.Logf("YAML and JSON configs are not equal")
				t.Logf("YAML: %+v", yamlCfg)
				t.Logf("JSON: %+v", jsonCfg)
				return false
			}

			if !configsEqual(yamlCfg, tomlCfg) {
				t.Logf("YAML and TOML configs are not equal")
				t.Logf("YAML: %+v", yamlCfg)
				t.Logf("TOML: %+v", tomlCfg)
				return false
			}

			if !configsEqual(jsonCfg, tomlCfg) {
				t.Logf("JSON and TOML configs are not equal")
				t.Logf("JSON: %+v", jsonCfg)
				t.Logf("TOML: %+v", tomlCfg)
				return false
			}

			// Verify specific values match what we set
			if yamlCfg.HTTP.Port != httpPort {
				t.Logf("Expected HTTP port %d, got %d", httpPort, yamlCfg.HTTP.Port)
				return false
			}
			if yamlCfg.Management.Port != mgmtPort {
				t.Logf("Expected Management port %d, got %d", mgmtPort, yamlCfg.Management.Port)
				return false
			}
			if yamlCfg.Observability.LogLevel != logLevel {
				t.Logf("Expected log level %s, got %s", logLevel, yamlCfg.Observability.LogLevel)
				return false
			}
			if yamlCfg.Observability.LogFormat != logFormat {
				t.Logf("Expected log format %s, got %s", logFormat, yamlCfg.Observability.LogFormat)
				return false
			}
			if yamlCfg.HTTP.ReadTimeout != readTimeout {
				t.Logf("Expected read timeout %v, got %v", readTimeout, yamlCfg.HTTP.ReadTimeout)
				return false
			}
			if yamlCfg.Management.AuthEnabled {
				t.Logf("Expected management auth disabled, got true")
				return false
			}
			if yamlCfg.Swagger.Enabled != swaggerEnabled {
				t.Logf("Expected swagger enabled %v, got %v", swaggerEnabled, yamlCfg.Swagger.Enabled)
				return false
			}

			return true
		},
		genPort,
		genPort,
		genLogLevel,
		genLogFormat,
		genTimeout,
		genTimeout,
		genBool,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty10_ConfigurationValidation tests that invalid configurations
// are rejected with clear error messages
// **Property 10: Configuration Validation**
// **Validates: Requirements 10.6**
func TestProperty10_ConfigurationValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// Generator for invalid auth configurations (auth enabled but missing required fields)
	genInvalidAuth := gen.OneGenOf(
		// Missing issuer
		gen.Const(map[string]interface{}{
			"auth": map[string]interface{}{
				"enabled":  true,
				"jwks_url": "https://auth.example.com/.well-known/jwks.json",
				"audience": "my-service",
			},
		}),
		// Missing jwks_url
		gen.Const(map[string]interface{}{
			"auth": map[string]interface{}{
				"enabled":  true,
				"issuer":   "https://auth.example.com",
				"audience": "my-service",
			},
		}),
		// Missing audience
		gen.Const(map[string]interface{}{
			"auth": map[string]interface{}{
				"enabled":  true,
				"issuer":   "https://auth.example.com",
				"jwks_url": "https://auth.example.com/.well-known/jwks.json",
			},
		}),
	)

	// Generator for invalid database configurations
	genInvalidDatabase := gen.OneGenOf(
		// Invalid database type
		gen.Const(map[string]interface{}{
			"database": map[string]interface{}{
				"type": "invalid_db",
				"url":  "postgresql://localhost:5432/db",
			},
		}),
		// Missing URL when type is specified
		gen.Const(map[string]interface{}{
			"database": map[string]interface{}{
				"type": "postgres",
			},
		}),
	)

	// Generator for invalid cache configurations
	genInvalidCache := gen.OneGenOf(
		// Invalid cache type
		gen.Const(map[string]interface{}{
			"cache": map[string]interface{}{
				"type": "invalid_cache",
				"url":  "redis://localhost:6379",
			},
		}),
		// Missing URL for redis cache
		gen.Const(map[string]interface{}{
			"cache": map[string]interface{}{
				"type": "redis",
			},
		}),
	)

	// Generator for invalid eventbus configurations
	genInvalidEventBus := gen.OneGenOf(
		// Invalid eventbus type
		gen.Const(map[string]interface{}{
			"eventbus": map[string]interface{}{
				"type":    "invalid_bus",
				"brokers": []string{"localhost:9092"},
			},
		}),
		// Missing brokers
		gen.Const(map[string]interface{}{
			"eventbus": map[string]interface{}{
				"type": "kafka",
			},
		}),
		// Invalid serializer
		gen.Const(map[string]interface{}{
			"eventbus": map[string]interface{}{
				"type":       "kafka",
				"brokers":    []string{"localhost:9092"},
				"serializer": "invalid_serializer",
			},
		}),
	)

	// Generator for invalid observability configurations
	genInvalidObservability := gen.OneGenOf(
		// Invalid log level
		gen.Const(map[string]interface{}{
			"observability": map[string]interface{}{
				"log_level": "invalid_level",
			},
		}),
		// Invalid log format
		gen.Const(map[string]interface{}{
			"observability": map[string]interface{}{
				"log_format": "invalid_format",
			},
		}),
		// Tracing enabled without endpoint
		gen.Const(map[string]interface{}{
			"observability": map[string]interface{}{
				"tracing_enabled": true,
			},
		}),
	)

	// Generator for invalid port configurations
	genInvalidPorts := gen.OneGenOf(
		// HTTP port out of range (too low)
		gen.Const(map[string]interface{}{
			"http": map[string]interface{}{
				"port": 0,
			},
		}),
		// HTTP port out of range (too high)
		gen.Const(map[string]interface{}{
			"http": map[string]interface{}{
				"port": 70000,
			},
		}),
		// Management port out of range
		gen.Const(map[string]interface{}{
			"management": map[string]interface{}{
				"port": -1,
			},
		}),
		// Same port for HTTP and Management
		gen.Const(map[string]interface{}{
			"http": map[string]interface{}{
				"port": 8080,
			},
			"management": map[string]interface{}{
				"port": 8080,
			},
		}),
	)

	properties.Property("Invalid auth config fails with clear error", prop.ForAll(
		func(invalidConfig map[string]interface{}) bool {
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempYAMLFile(t, invalidConfig)
			defer os.Remove(configFile)

			loader := NewViperLoader(configFile, "APP")
			_, err := loader.Load()

			// Should fail validation
			if err == nil {
				t.Logf("Expected validation error for invalid auth config, got nil")
				return false
			}

			// Error message should mention auth
			errMsg := err.Error()
			if !strings.Contains(errMsg, "auth") {
				t.Logf("Expected error message to mention 'auth', got: %s", errMsg)
				return false
			}

			// Error should be specific about what's missing
			hasSpecificError := strings.Contains(errMsg, "issuer") ||
				strings.Contains(errMsg, "jwks_url") ||
				strings.Contains(errMsg, "audience")
			if !hasSpecificError {
				t.Logf("Expected specific error about missing field, got: %s", errMsg)
				return false
			}

			return true
		},
		genInvalidAuth,
	))

	properties.Property("Invalid database config fails with clear error", prop.ForAll(
		func(invalidConfig map[string]interface{}) bool {
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempYAMLFile(t, invalidConfig)
			defer os.Remove(configFile)

			loader := NewViperLoader(configFile, "APP")
			_, err := loader.Load()

			// Should fail validation
			if err == nil {
				t.Logf("Expected validation error for invalid database config, got nil")
				return false
			}

			// Error message should mention database
			errMsg := err.Error()
			if !strings.Contains(errMsg, "database") {
				t.Logf("Expected error message to mention 'database', got: %s", errMsg)
				return false
			}

			// Error should be specific
			hasSpecificError := strings.Contains(errMsg, "type") ||
				strings.Contains(errMsg, "url")
			if !hasSpecificError {
				t.Logf("Expected specific error about database field, got: %s", errMsg)
				return false
			}

			return true
		},
		genInvalidDatabase,
	))

	properties.Property("Invalid cache config fails with clear error", prop.ForAll(
		func(invalidConfig map[string]interface{}) bool {
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempYAMLFile(t, invalidConfig)
			defer os.Remove(configFile)

			loader := NewViperLoader(configFile, "APP")
			_, err := loader.Load()

			// Should fail validation
			if err == nil {
				t.Logf("Expected validation error for invalid cache config, got nil")
				return false
			}

			// Error message should mention cache
			errMsg := err.Error()
			if !strings.Contains(errMsg, "cache") {
				t.Logf("Expected error message to mention 'cache', got: %s", errMsg)
				return false
			}

			return true
		},
		genInvalidCache,
	))

	properties.Property("Invalid eventbus config fails with clear error", prop.ForAll(
		func(invalidConfig map[string]interface{}) bool {
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempYAMLFile(t, invalidConfig)
			defer os.Remove(configFile)

			loader := NewViperLoader(configFile, "APP")
			_, err := loader.Load()

			// Should fail validation
			if err == nil {
				t.Logf("Expected validation error for invalid eventbus config, got nil")
				return false
			}

			// Error message should mention eventbus
			errMsg := err.Error()
			if !strings.Contains(errMsg, "eventbus") {
				t.Logf("Expected error message to mention 'eventbus', got: %s", errMsg)
				return false
			}

			return true
		},
		genInvalidEventBus,
	))

	properties.Property("Invalid observability config fails with clear error", prop.ForAll(
		func(invalidConfig map[string]interface{}) bool {
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempYAMLFile(t, invalidConfig)
			defer os.Remove(configFile)

			loader := NewViperLoader(configFile, "APP")
			_, err := loader.Load()

			// Should fail validation
			if err == nil {
				t.Logf("Expected validation error for invalid observability config, got nil")
				return false
			}

			// Error message should mention observability
			errMsg := err.Error()
			if !strings.Contains(errMsg, "observability") {
				t.Logf("Expected error message to mention 'observability', got: %s", errMsg)
				return false
			}

			return true
		},
		genInvalidObservability,
	))

	properties.Property("Invalid port config fails with clear error", prop.ForAll(
		func(invalidConfig map[string]interface{}) bool {
			clearAppEnv()
			defer clearAppEnv()

			configFile := createTempYAMLFile(t, invalidConfig)
			defer os.Remove(configFile)

			loader := NewViperLoader(configFile, "APP")
			_, err := loader.Load()

			// Should fail validation
			if err == nil {
				t.Logf("Expected validation error for invalid port config, got nil")
				return false
			}

			// Error message should mention port
			errMsg := err.Error()
			if !strings.Contains(errMsg, "port") {
				t.Logf("Expected error message to mention 'port', got: %s", errMsg)
				return false
			}

			return true
		},
		genInvalidPorts,
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to create a temporary YAML config file
func createTempYAMLFile(t *testing.T, config map[string]interface{}) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp YAML file: %v", err)
	}

	var content strings.Builder
	writeYAML(&content, config, 0)

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Helper function to create a temporary JSON config file
func createTempJSONFile(t *testing.T, config map[string]interface{}) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp JSON file: %v", err)
	}

	var content strings.Builder
	writeJSON(&content, config, 0)

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write JSON file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Helper function to create a temporary TOML config file
func createTempTOMLFile(t *testing.T, config map[string]interface{}) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp TOML file: %v", err)
	}

	var content strings.Builder
	writeTOML(&content, config, "")

	if _, err := tmpFile.WriteString(content.String()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write TOML file: %v", err)
	}

	tmpFile.Close()
	return tmpFile.Name()
}

// Helper function to write JSON content recursively
func writeJSON(w *strings.Builder, data map[string]interface{}, indent int) {
	indentStr := strings.Repeat("  ", indent)
	w.WriteString("{\n")

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	for i, key := range keys {
		value := data[key]
		w.WriteString(fmt.Sprintf("%s  \"%s\": ", indentStr, key))

		switch v := value.(type) {
		case map[string]interface{}:
			writeJSON(w, v, indent+1)
		case string:
			w.WriteString(fmt.Sprintf("\"%s\"", v))
		case bool:
			w.WriteString(fmt.Sprintf("%t", v))
		case int:
			w.WriteString(fmt.Sprintf("%d", v))
		case float64:
			w.WriteString(fmt.Sprintf("%f", v))
		default:
			w.WriteString(fmt.Sprintf("\"%v\"", v))
		}

		if i < len(keys)-1 {
			w.WriteString(",")
		}
		w.WriteString("\n")
	}

	w.WriteString(fmt.Sprintf("%s}", indentStr))
}

// Helper function to write TOML content recursively
func writeTOML(w *strings.Builder, data map[string]interface{}, prefix string) {
	// First write simple key-value pairs
	for key, value := range data {
		if _, isMap := value.(map[string]interface{}); !isMap {
			switch v := value.(type) {
			case string:
				w.WriteString(fmt.Sprintf("%s = \"%s\"\n", key, v))
			case bool:
				w.WriteString(fmt.Sprintf("%s = %t\n", key, v))
			case int:
				w.WriteString(fmt.Sprintf("%s = %d\n", key, v))
			case float64:
				w.WriteString(fmt.Sprintf("%s = %f\n", key, v))
			default:
				w.WriteString(fmt.Sprintf("%s = \"%v\"\n", key, v))
			}
		}
	}

	// Then write sections (tables)
	for key, value := range data {
		if subMap, isMap := value.(map[string]interface{}); isMap {
			sectionName := key
			if prefix != "" {
				sectionName = prefix + "." + key
			}
			w.WriteString(fmt.Sprintf("\n[%s]\n", sectionName))
			writeTOML(w, subMap, sectionName)
		}
	}
}

// Helper function to compare two Config structs for equality
func configsEqual(a, b *Config) bool {
	if a == nil || b == nil {
		return a == b
	}

	// Compare HTTP config
	if a.HTTP.Port != b.HTTP.Port ||
		a.HTTP.ReadTimeout != b.HTTP.ReadTimeout ||
		a.HTTP.WriteTimeout != b.HTTP.WriteTimeout ||
		a.HTTP.IdleTimeout != b.HTTP.IdleTimeout ||
		a.HTTP.MaxRequestSize != b.HTTP.MaxRequestSize {
		return false
	}

	// Compare Management config
	if a.Management.Port != b.Management.Port ||
		a.Management.ReadTimeout != b.Management.ReadTimeout ||
		a.Management.WriteTimeout != b.Management.WriteTimeout ||
		a.Management.AuthEnabled != b.Management.AuthEnabled ||
		a.Management.MTLSEnabled != b.Management.MTLSEnabled ||
		a.Management.TLSCertFile != b.Management.TLSCertFile ||
		a.Management.TLSKeyFile != b.Management.TLSKeyFile ||
		a.Management.TLSCAFile != b.Management.TLSCAFile {
		return false
	}

	// Compare Auth config
	if a.Auth.Enabled != b.Auth.Enabled ||
		a.Auth.Issuer != b.Auth.Issuer ||
		a.Auth.JWKSUrl != b.Auth.JWKSUrl ||
		a.Auth.JWKSCacheTTL != b.Auth.JWKSCacheTTL ||
		a.Auth.Audience != b.Auth.Audience {
		return false
	}

	// Compare Database config
	if a.Database.Type != b.Database.Type ||
		a.Database.URL != b.Database.URL ||
		a.Database.MaxOpenConns != b.Database.MaxOpenConns ||
		a.Database.MaxIdleConns != b.Database.MaxIdleConns ||
		a.Database.ConnMaxLifetime != b.Database.ConnMaxLifetime ||
		a.Database.QueryTimeout != b.Database.QueryTimeout {
		return false
	}

	// Compare Cache config
	if a.Cache.Type != b.Cache.Type ||
		a.Cache.URL != b.Cache.URL ||
		a.Cache.MaxConns != b.Cache.MaxConns ||
		a.Cache.OperationTimeout != b.Cache.OperationTimeout {
		return false
	}

	// Compare EventBus config
	if a.EventBus.Type != b.EventBus.Type ||
		a.EventBus.Serializer != b.EventBus.Serializer ||
		a.EventBus.OperationTimeout != b.EventBus.OperationTimeout {
		return false
	}
	if len(a.EventBus.Brokers) != len(b.EventBus.Brokers) {
		return false
	}
	for i := range a.EventBus.Brokers {
		if a.EventBus.Brokers[i] != b.EventBus.Brokers[i] {
			return false
		}
	}

	// Compare Observability config
	if a.Observability.LogLevel != b.Observability.LogLevel ||
		a.Observability.LogFormat != b.Observability.LogFormat ||
		a.Observability.TracingEnabled != b.Observability.TracingEnabled ||
		a.Observability.TracingSampleRate != b.Observability.TracingSampleRate ||
		a.Observability.TracingEndpoint != b.Observability.TracingEndpoint {
		return false
	}

	// Compare Swagger config
	if a.Swagger.Enabled != b.Swagger.Enabled ||
		a.Swagger.SpecPath != b.Swagger.SpecPath {
		return false
	}

	return true
}

// TestConfigurationDefaults_AllSections tests that all configuration sections
// have proper defaults applied when no config is provided
// **Validates: Requirements 10.4**
func TestConfigurationDefaults_AllSections(t *testing.T) {
	// Clear all APP_ environment variables to ensure clean state
	clearAppEnv()
	defer clearAppEnv()

	// Load configuration with no file and no environment variables
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error loading defaults, got: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config to be non-nil")
	}

	// Get expected defaults
	defaults := DefaultConfig()

	// Test HTTP configuration defaults
	t.Run("HTTP defaults", func(t *testing.T) {
		if cfg.HTTP.Port != defaults.HTTP.Port {
			t.Errorf("HTTP.Port: expected %d, got %d", defaults.HTTP.Port, cfg.HTTP.Port)
		}
		if cfg.HTTP.ReadTimeout != defaults.HTTP.ReadTimeout {
			t.Errorf("HTTP.ReadTimeout: expected %v, got %v", defaults.HTTP.ReadTimeout, cfg.HTTP.ReadTimeout)
		}
		if cfg.HTTP.WriteTimeout != defaults.HTTP.WriteTimeout {
			t.Errorf("HTTP.WriteTimeout: expected %v, got %v", defaults.HTTP.WriteTimeout, cfg.HTTP.WriteTimeout)
		}
		if cfg.HTTP.IdleTimeout != defaults.HTTP.IdleTimeout {
			t.Errorf("HTTP.IdleTimeout: expected %v, got %v", defaults.HTTP.IdleTimeout, cfg.HTTP.IdleTimeout)
		}
		if cfg.HTTP.MaxRequestSize != defaults.HTTP.MaxRequestSize {
			t.Errorf("HTTP.MaxRequestSize: expected %d, got %d", defaults.HTTP.MaxRequestSize, cfg.HTTP.MaxRequestSize)
		}
	})

	// Test Management configuration defaults
	t.Run("Management defaults", func(t *testing.T) {
		if cfg.Management.Port != defaults.Management.Port {
			t.Errorf("Management.Port: expected %d, got %d", defaults.Management.Port, cfg.Management.Port)
		}
		if cfg.Management.ReadTimeout != defaults.Management.ReadTimeout {
			t.Errorf("Management.ReadTimeout: expected %v, got %v", defaults.Management.ReadTimeout, cfg.Management.ReadTimeout)
		}
		if cfg.Management.WriteTimeout != defaults.Management.WriteTimeout {
			t.Errorf("Management.WriteTimeout: expected %v, got %v", defaults.Management.WriteTimeout, cfg.Management.WriteTimeout)
		}
		if cfg.Management.AuthEnabled != defaults.Management.AuthEnabled {
			t.Errorf("Management.AuthEnabled: expected %v, got %v", defaults.Management.AuthEnabled, cfg.Management.AuthEnabled)
		}
		if cfg.Management.MTLSEnabled != defaults.Management.MTLSEnabled {
			t.Errorf("Management.MTLSEnabled: expected %v, got %v", defaults.Management.MTLSEnabled, cfg.Management.MTLSEnabled)
		}
	})

	// Test Auth configuration defaults
	t.Run("Auth defaults", func(t *testing.T) {
		if cfg.Auth.Enabled != defaults.Auth.Enabled {
			t.Errorf("Auth.Enabled: expected %v, got %v", defaults.Auth.Enabled, cfg.Auth.Enabled)
		}
		if cfg.Auth.JWKSCacheTTL != defaults.Auth.JWKSCacheTTL {
			t.Errorf("Auth.JWKSCacheTTL: expected %v, got %v", defaults.Auth.JWKSCacheTTL, cfg.Auth.JWKSCacheTTL)
		}
		// Issuer, JWKSUrl, and Audience should be empty strings by default
		if cfg.Auth.Issuer != "" {
			t.Errorf("Auth.Issuer: expected empty string, got %s", cfg.Auth.Issuer)
		}
		if cfg.Auth.JWKSUrl != "" {
			t.Errorf("Auth.JWKSUrl: expected empty string, got %s", cfg.Auth.JWKSUrl)
		}
		if cfg.Auth.Audience != "" {
			t.Errorf("Auth.Audience: expected empty string, got %s", cfg.Auth.Audience)
		}
	})

	// Test Database configuration defaults
	t.Run("Database defaults", func(t *testing.T) {
		if cfg.Database.MaxOpenConns != defaults.Database.MaxOpenConns {
			t.Errorf("Database.MaxOpenConns: expected %d, got %d", defaults.Database.MaxOpenConns, cfg.Database.MaxOpenConns)
		}
		if cfg.Database.MaxIdleConns != defaults.Database.MaxIdleConns {
			t.Errorf("Database.MaxIdleConns: expected %d, got %d", defaults.Database.MaxIdleConns, cfg.Database.MaxIdleConns)
		}
		if cfg.Database.ConnMaxLifetime != defaults.Database.ConnMaxLifetime {
			t.Errorf("Database.ConnMaxLifetime: expected %v, got %v", defaults.Database.ConnMaxLifetime, cfg.Database.ConnMaxLifetime)
		}
		if cfg.Database.QueryTimeout != defaults.Database.QueryTimeout {
			t.Errorf("Database.QueryTimeout: expected %v, got %v", defaults.Database.QueryTimeout, cfg.Database.QueryTimeout)
		}
		// Type and URL should be empty by default
		if cfg.Database.Type != "" {
			t.Errorf("Database.Type: expected empty string, got %s", cfg.Database.Type)
		}
		if cfg.Database.URL != "" {
			t.Errorf("Database.URL: expected empty string, got %s", cfg.Database.URL)
		}
	})

	// Test Cache configuration defaults
	t.Run("Cache defaults", func(t *testing.T) {
		if cfg.Cache.MaxConns != defaults.Cache.MaxConns {
			t.Errorf("Cache.MaxConns: expected %d, got %d", defaults.Cache.MaxConns, cfg.Cache.MaxConns)
		}
		if cfg.Cache.OperationTimeout != defaults.Cache.OperationTimeout {
			t.Errorf("Cache.OperationTimeout: expected %v, got %v", defaults.Cache.OperationTimeout, cfg.Cache.OperationTimeout)
		}
		// Type and URL should be empty by default
		if cfg.Cache.Type != "" {
			t.Errorf("Cache.Type: expected empty string, got %s", cfg.Cache.Type)
		}
		if cfg.Cache.URL != "" {
			t.Errorf("Cache.URL: expected empty string, got %s", cfg.Cache.URL)
		}
	})

	// Test EventBus configuration defaults
	t.Run("EventBus defaults", func(t *testing.T) {
		if cfg.EventBus.Serializer != defaults.EventBus.Serializer {
			t.Errorf("EventBus.Serializer: expected %s, got %s", defaults.EventBus.Serializer, cfg.EventBus.Serializer)
		}
		if cfg.EventBus.OperationTimeout != defaults.EventBus.OperationTimeout {
			t.Errorf("EventBus.OperationTimeout: expected %v, got %v", defaults.EventBus.OperationTimeout, cfg.EventBus.OperationTimeout)
		}
		// Type should be empty and Brokers should be nil/empty by default
		if cfg.EventBus.Type != "" {
			t.Errorf("EventBus.Type: expected empty string, got %s", cfg.EventBus.Type)
		}
		if len(cfg.EventBus.Brokers) != 0 {
			t.Errorf("EventBus.Brokers: expected empty slice, got %v", cfg.EventBus.Brokers)
		}
	})

	// Test Observability configuration defaults
	t.Run("Observability defaults", func(t *testing.T) {
		if cfg.Observability.LogLevel != defaults.Observability.LogLevel {
			t.Errorf("Observability.LogLevel: expected %s, got %s", defaults.Observability.LogLevel, cfg.Observability.LogLevel)
		}
		if cfg.Observability.LogFormat != defaults.Observability.LogFormat {
			t.Errorf("Observability.LogFormat: expected %s, got %s", defaults.Observability.LogFormat, cfg.Observability.LogFormat)
		}
		if cfg.Observability.TracingEnabled != defaults.Observability.TracingEnabled {
			t.Errorf("Observability.TracingEnabled: expected %v, got %v", defaults.Observability.TracingEnabled, cfg.Observability.TracingEnabled)
		}
		if cfg.Observability.TracingSampleRate != defaults.Observability.TracingSampleRate {
			t.Errorf("Observability.TracingSampleRate: expected %v, got %v", defaults.Observability.TracingSampleRate, cfg.Observability.TracingSampleRate)
		}
		// TracingEndpoint should be empty by default
		if cfg.Observability.TracingEndpoint != "" {
			t.Errorf("Observability.TracingEndpoint: expected empty string, got %s", cfg.Observability.TracingEndpoint)
		}
	})

	// Test Swagger configuration defaults
	t.Run("Swagger defaults", func(t *testing.T) {
		if cfg.Swagger.Enabled != defaults.Swagger.Enabled {
			t.Errorf("Swagger.Enabled: expected %v, got %v", defaults.Swagger.Enabled, cfg.Swagger.Enabled)
		}
		if cfg.Swagger.SpecPath != defaults.Swagger.SpecPath {
			t.Errorf("Swagger.SpecPath: expected %s, got %s", defaults.Swagger.SpecPath, cfg.Swagger.SpecPath)
		}
	})
}

// TestConfigurationDefaults_SpecificValues tests specific default values
// to ensure they match the design document specifications
// **Validates: Requirements 10.4**
func TestConfigurationDefaults_SpecificValues(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error loading defaults, got: %v", err)
	}

	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		// HTTP defaults
		{"HTTP.Port", cfg.HTTP.Port, 8080},
		{"HTTP.ReadTimeout", cfg.HTTP.ReadTimeout, 30 * time.Second},
		{"HTTP.WriteTimeout", cfg.HTTP.WriteTimeout, 30 * time.Second},
		{"HTTP.IdleTimeout", cfg.HTTP.IdleTimeout, 120 * time.Second},
		{"HTTP.MaxRequestSize", cfg.HTTP.MaxRequestSize, int64(1 << 20)},

		// Management defaults
		{"Management.Port", cfg.Management.Port, 9090},
		{"Management.ReadTimeout", cfg.Management.ReadTimeout, 10 * time.Second},
		{"Management.WriteTimeout", cfg.Management.WriteTimeout, 10 * time.Second},
		{"Management.AuthEnabled", cfg.Management.AuthEnabled, false},
		{"Management.MTLSEnabled", cfg.Management.MTLSEnabled, false},

		// Auth defaults
		{"Auth.Enabled", cfg.Auth.Enabled, false},
		{"Auth.JWKSCacheTTL", cfg.Auth.JWKSCacheTTL, 1 * time.Hour},

		// Database defaults
		{"Database.MaxOpenConns", cfg.Database.MaxOpenConns, 25},
		{"Database.MaxIdleConns", cfg.Database.MaxIdleConns, 5},
		{"Database.ConnMaxLifetime", cfg.Database.ConnMaxLifetime, 5 * time.Minute},
		{"Database.QueryTimeout", cfg.Database.QueryTimeout, 10 * time.Second},

		// Cache defaults
		{"Cache.MaxConns", cfg.Cache.MaxConns, 10},
		{"Cache.OperationTimeout", cfg.Cache.OperationTimeout, 5 * time.Second},

		// EventBus defaults
		{"EventBus.Serializer", cfg.EventBus.Serializer, "json"},
		{"EventBus.OperationTimeout", cfg.EventBus.OperationTimeout, 30 * time.Second},

		// Observability defaults
		{"Observability.LogLevel", cfg.Observability.LogLevel, "info"},
		{"Observability.LogFormat", cfg.Observability.LogFormat, "json"},
		{"Observability.TracingEnabled", cfg.Observability.TracingEnabled, false},
		{"Observability.TracingSampleRate", cfg.Observability.TracingSampleRate, 0.1},

		// Swagger defaults
		{"Swagger.Enabled", cfg.Swagger.Enabled, false},
		{"Swagger.SpecPath", cfg.Swagger.SpecPath, "/api/openapi/openapi.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, tt.actual)
			}
		})
	}
}

// TestConfigurationDefaults_NoFileNoEnv tests that defaults are used
// when neither config file nor environment variables are provided
// **Validates: Requirements 10.4**
func TestConfigurationDefaults_NoFileNoEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	// Create loader with no config file
	loader := NewViperLoader("", "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify that configuration is valid and uses defaults
	defaults := DefaultConfig()

	if !configsEqual(cfg, defaults) {
		t.Error("configuration does not match defaults")
		t.Logf("Expected: %+v", defaults)
		t.Logf("Got: %+v", cfg)
	}
}

// TestConfigurationDefaults_EmptyConfigFile tests that defaults are used
// when an empty config file is provided
// **Validates: Requirements 10.4**
func TestConfigurationDefaults_EmptyConfigFile(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	// Create an empty config file
	tmpFile, err := os.CreateTemp("", "empty-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Load configuration with empty file
	loader := NewViperLoader(tmpFile.Name(), "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error with empty config file, got: %v", err)
	}

	// Verify that configuration uses defaults
	defaults := DefaultConfig()

	if !configsEqual(cfg, defaults) {
		t.Error("configuration with empty file does not match defaults")
		t.Logf("Expected: %+v", defaults)
		t.Logf("Got: %+v", cfg)
	}
}

// TestConfigurationDefaults_PartialConfig tests that defaults are used
// for fields not specified in a partial config file
// **Validates: Requirements 10.4**
func TestConfigurationDefaults_PartialConfig(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	// Create a partial config file (only HTTP port specified)
	configFile := createTempYAMLFile(t, map[string]interface{}{
		"http": map[string]interface{}{
			"port": 9999,
		},
	})
	defer os.Remove(configFile)

	loader := NewViperLoader(configFile, "APP")
	cfg, err := loader.Load()

	if err != nil {
		t.Fatalf("expected no error with partial config, got: %v", err)
	}

	defaults := DefaultConfig()

	// Verify specified value is used
	if cfg.HTTP.Port != 9999 {
		t.Errorf("HTTP.Port: expected 9999, got %d", cfg.HTTP.Port)
	}

	// Verify other HTTP fields use defaults
	if cfg.HTTP.ReadTimeout != defaults.HTTP.ReadTimeout {
		t.Errorf("HTTP.ReadTimeout: expected default %v, got %v", defaults.HTTP.ReadTimeout, cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != defaults.HTTP.WriteTimeout {
		t.Errorf("HTTP.WriteTimeout: expected default %v, got %v", defaults.HTTP.WriteTimeout, cfg.HTTP.WriteTimeout)
	}
	if cfg.HTTP.IdleTimeout != defaults.HTTP.IdleTimeout {
		t.Errorf("HTTP.IdleTimeout: expected default %v, got %v", defaults.HTTP.IdleTimeout, cfg.HTTP.IdleTimeout)
	}
	if cfg.HTTP.MaxRequestSize != defaults.HTTP.MaxRequestSize {
		t.Errorf("HTTP.MaxRequestSize: expected default %d, got %d", defaults.HTTP.MaxRequestSize, cfg.HTTP.MaxRequestSize)
	}

	// Verify all other sections use defaults
	if cfg.Management.Port != defaults.Management.Port {
		t.Errorf("Management.Port: expected default %d, got %d", defaults.Management.Port, cfg.Management.Port)
	}
	if cfg.Observability.LogLevel != defaults.Observability.LogLevel {
		t.Errorf("Observability.LogLevel: expected default %s, got %s", defaults.Observability.LogLevel, cfg.Observability.LogLevel)
	}
	if cfg.Swagger.Enabled != defaults.Swagger.Enabled {
		t.Errorf("Swagger.Enabled: expected default %v, got %v", defaults.Swagger.Enabled, cfg.Swagger.Enabled)
	}
}

// TestConfigurationDefaults_TableDriven tests various scenarios using table-driven approach
// **Validates: Requirements 10.4**
func TestConfigurationDefaults_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		configData map[string]interface{}
		envVars    map[string]string
		checkFunc  func(*testing.T, *Config)
	}{
		{
			name:       "no config, no env - all defaults",
			configData: nil,
			envVars:    nil,
			checkFunc: func(t *testing.T, cfg *Config) {
				defaults := DefaultConfig()
				if !configsEqual(cfg, defaults) {
					t.Error("config does not match defaults")
				}
			},
		},
		{
			name: "partial HTTP config - other fields use defaults",
			configData: map[string]interface{}{
				"http": map[string]interface{}{
					"port": 7777,
				},
			},
			envVars: nil,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.HTTP.Port != 7777 {
					t.Errorf("expected port 7777, got %d", cfg.HTTP.Port)
				}
				if cfg.HTTP.ReadTimeout != 30*time.Second {
					t.Errorf("expected default read timeout, got %v", cfg.HTTP.ReadTimeout)
				}
				if cfg.Management.Port != 9090 {
					t.Errorf("expected default management port, got %d", cfg.Management.Port)
				}
			},
		},
		{
			name: "partial observability config - other sections use defaults",
			configData: map[string]interface{}{
				"observability": map[string]interface{}{
					"log_level": "debug",
				},
			},
			envVars: nil,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.Observability.LogLevel != "debug" {
					t.Errorf("expected log level debug, got %s", cfg.Observability.LogLevel)
				}
				if cfg.Observability.LogFormat != "json" {
					t.Errorf("expected default log format json, got %s", cfg.Observability.LogFormat)
				}
				if cfg.HTTP.Port != 8080 {
					t.Errorf("expected default HTTP port, got %d", cfg.HTTP.Port)
				}
			},
		},
		{
			name:       "env overrides defaults",
			configData: nil,
			envVars: map[string]string{
				"APP_HTTP_PORT":               "5555",
				"APP_OBSERVABILITY_LOG_LEVEL": "warn",
			},
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.HTTP.Port != 5555 {
					t.Errorf("expected port 5555 from env, got %d", cfg.HTTP.Port)
				}
				if cfg.Observability.LogLevel != "warn" {
					t.Errorf("expected log level warn from env, got %s", cfg.Observability.LogLevel)
				}
				// Other fields should still use defaults
				if cfg.Management.Port != 9090 {
					t.Errorf("expected default management port, got %d", cfg.Management.Port)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAppEnv()
			defer clearAppEnv()

			// Set environment variables if provided
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Create config file if data provided
			var configFile string
			if tt.configData != nil {
				configFile = createTempYAMLFile(t, tt.configData)
				defer os.Remove(configFile)
			}

			// Load configuration
			loader := NewViperLoader(configFile, "APP")
			cfg, err := loader.Load()

			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			// Run check function
			tt.checkFunc(t, cfg)
		})
	}
}

func TestViperLoader_RequestTimeoutFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBSERVABILITY_REQUEST_TIMEOUT_ENABLED", "true")
	os.Setenv("APP_OBSERVABILITY_REQUEST_TIMEOUT_DEFAULT", "7s")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.Observability.RequestTimeout.Enabled {
		t.Fatal("expected observability.request_timeout.enabled=true")
	}
	if cfg.Observability.RequestTimeout.Default != 7*time.Second {
		t.Fatalf("expected observability.request_timeout.default=7s, got %v", cfg.Observability.RequestTimeout.Default)
	}
}

func TestViperLoader_InvalidRequestTimeoutConfig(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBSERVABILITY_REQUEST_TIMEOUT_ENABLED", "true")
	os.Setenv("APP_OBSERVABILITY_REQUEST_TIMEOUT_DEFAULT", "0s")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for request timeout default")
	}
	if !strings.Contains(err.Error(), "observability.request_timeout.default must be greater than zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_RequestLoggingOutputAndFieldsFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBSERVABILITY_REQUEST_LOGGING_OUTPUT", "stderr")
	os.Setenv("APP_OBSERVABILITY_REQUEST_LOGGING_FIELDS", "request_id,request_method,request_uri,status,request_time,http_user_agent")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Observability.RequestLogging.Output != "stderr" {
		t.Fatalf("expected observability.request_logging.output=stderr, got %q", cfg.Observability.RequestLogging.Output)
	}
	if len(cfg.Observability.RequestLogging.Fields) != 6 {
		t.Fatalf("expected 6 request logging fields, got %d (%v)", len(cfg.Observability.RequestLogging.Fields), cfg.Observability.RequestLogging.Fields)
	}
}

func TestViperLoader_InvalidRequestLoggingOutput(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBSERVABILITY_REQUEST_LOGGING_OUTPUT", "file")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for request logging output")
	}
	if !strings.Contains(err.Error(), "observability.request_logging.output must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_InvalidRequestLoggingField(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_OBSERVABILITY_REQUEST_LOGGING_FIELDS", "request_id,bad_field,status")

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for request logging fields")
	}
	if !strings.Contains(err.Error(), "observability.request_logging.fields[1] must be one of") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestViperLoader_LoadSSEFromEnv(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SSE_ENABLED", "true")
	os.Setenv("APP_SSE_ENDPOINT", "/events")
	os.Setenv("APP_SSE_STORE", "redis")
	os.Setenv("APP_SSE_BUS", "eventbus")
	os.Setenv("APP_SSE_REPLAY_LIMIT", "200")
	os.Setenv("APP_SSE_HEARTBEAT_INTERVAL", "15s")
	os.Setenv("APP_SSE_REDIS_URL", "redis://localhost:6379/3")
	os.Setenv("APP_EVENTBUS_TYPE", "kafka")
	os.Setenv("APP_EVENTBUS_BROKERS", "localhost:9092")

	cfg, err := NewViperLoader("", "APP").Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !cfg.SSE.Enabled {
		t.Fatal("expected sse.enabled=true")
	}
	if cfg.SSE.Store != "redis" {
		t.Fatalf("expected sse.store=redis, got %s", cfg.SSE.Store)
	}
	if cfg.SSE.Bus != "eventbus" {
		t.Fatalf("expected sse.bus=eventbus, got %s", cfg.SSE.Bus)
	}
	if cfg.SSE.ReplayLimit != 200 {
		t.Fatalf("expected sse.replay_limit=200, got %d", cfg.SSE.ReplayLimit)
	}
	if cfg.SSE.HeartbeatInterval != 15*time.Second {
		t.Fatalf("expected sse.heartbeat_interval=15s, got %v", cfg.SSE.HeartbeatInterval)
	}
}

func TestViperLoader_InvalidSSEConfig(t *testing.T) {
	clearAppEnv()
	defer clearAppEnv()

	os.Setenv("APP_SSE_ENABLED", "true")
	os.Setenv("APP_SSE_STORE", "redis")
	os.Setenv("APP_SSE_BUS", "redis")
	// Missing APP_SSE_REDIS_URL on purpose

	_, err := NewViperLoader("", "APP").Load()
	if err == nil {
		t.Fatal("expected validation error for missing sse redis url")
	}
	if !strings.Contains(err.Error(), "sse.redis.url is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
