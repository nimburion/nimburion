# Configuration Management

The Go Microservices Framework uses a hierarchical configuration system built on [Viper](https://github.com/spf13/viper) that supports multiple configuration sources with clear precedence rules.

## Configuration Precedence

Configuration values are loaded in the following order (highest to lowest priority):

1. **Environment Variables** - Highest priority, always override other sources
2. **Configuration File** - YAML, JSON, or TOML format
3. **Default Values** - Built-in sensible defaults

This precedence model follows the [12-Factor App](https://12factor.net/config) methodology and enables flexible deployment across different environments.

## Environment Variables

All environment variables use the `APP_` prefix and follow a hierarchical naming convention using underscores to separate levels.

### Naming Convention

- Top-level: `APP_<SECTION>_<KEY>`
- Example: `APP_HTTP_PORT`, `APP_AUTH_ISSUER`

### Complete Environment Variable Reference

#### Router Configuration

- `APP_ROUTER_TYPE` - Router type: `nethttp`, `gin`, `gorilla` (default: `nethttp`)
  - Example: `APP_ROUTER_TYPE=nethttp`
  - Example: `APP_ROUTER_TYPE=gin`
  - Example: `APP_ROUTER_TYPE=gorilla`

#### HTTP Server Configuration

- `APP_HTTP_PORT` - Public API server port (default: 8080)
- `APP_HTTP_READ_TIMEOUT` - Read timeout duration (default: 30s)
- `APP_HTTP_WRITE_TIMEOUT` - Write timeout duration (default: 30s)
- `APP_HTTP_IDLE_TIMEOUT` - Idle timeout duration (default: 120s)
- `APP_HTTP_MAX_REQUEST_SIZE` - Max request body size in bytes (default: 1048576, set `0` to disable)

#### Management Server Configuration

- `APP_MGMT_PORT` - Management server port (default: 9090)
- `APP_MGMT_READ_TIMEOUT` - Read timeout duration (default: 10s)
- `APP_MGMT_WRITE_TIMEOUT` - Write timeout duration (default: 10s)
- `APP_MGMT_AUTH_ENABLED` - Enable authentication (default: false)
- `APP_MGMT_MTLS_ENABLED` - Enable mTLS (default: false)
- `APP_MGMT_TLS_CERT_FILE` - Server certificate path for management mTLS
- `APP_MGMT_TLS_KEY_FILE` - Server private key path for management mTLS
- `APP_MGMT_TLS_CA_FILE` - CA certificate path used to verify client certificates
  Legacy aliases:
  `APP_MANAGEMENT_PORT`, `APP_MANAGEMENT_READ_TIMEOUT`, `APP_MANAGEMENT_WRITE_TIMEOUT`, `APP_MANAGEMENT_AUTH_ENABLED`, `APP_MANAGEMENT_MTLS_ENABLED`

When `APP_MGMT_AUTH_ENABLED=true`, `APP_AUTH_ENABLED` must also be `true`.
When `APP_MGMT_MTLS_ENABLED=true`, all three TLS file paths are required.

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `management.port` | `APP_MGMT_PORT` | `APP_MANAGEMENT_PORT` |
| `management.read_timeout` | `APP_MGMT_READ_TIMEOUT` | `APP_MANAGEMENT_READ_TIMEOUT` |
| `management.write_timeout` | `APP_MGMT_WRITE_TIMEOUT` | `APP_MANAGEMENT_WRITE_TIMEOUT` |
| `management.auth_enabled` | `APP_MGMT_AUTH_ENABLED` | `APP_MANAGEMENT_AUTH_ENABLED` |
| `management.mtls_enabled` | `APP_MGMT_MTLS_ENABLED` | `APP_MANAGEMENT_MTLS_ENABLED` |
| `management.tls_cert_file` | `APP_MGMT_TLS_CERT_FILE` | |
| `management.tls_key_file` | `APP_MGMT_TLS_KEY_FILE` | |
| `management.tls_ca_file` | `APP_MGMT_TLS_CA_FILE` | |

#### Security Headers Configuration

- `APP_SECURITY_HEADERS_ENABLED` - Enable security headers middleware (default: true)
- `APP_SECURITY_HEADERS_IS_DEVELOPMENT` - Disable policy enforcement in development mode (default: false)
- `APP_SECURITY_HEADERS_ALLOWED_HOSTS` - Comma-separated allowlist of accepted hosts
- `APP_SECURITY_HEADERS_SSL_REDIRECT` - Redirect HTTP requests to HTTPS (default: false)
- `APP_SECURITY_HEADERS_SSL_TEMPORARY_REDIRECT` - Use 307 instead of 301 when redirecting (default: false)
- `APP_SECURITY_HEADERS_SSL_HOST` - Override redirect host for HTTPS redirects
- `APP_SECURITY_HEADERS_DONT_REDIRECT_IPV4_HOSTNAMES` - Skip redirects for IPv4 hostnames (default: true)
- `APP_SECURITY_HEADERS_STS_SECONDS` - HSTS max-age in seconds (default: 31536000)
- `APP_SECURITY_HEADERS_STS_INCLUDE_SUBDOMAINS` - Add HSTS `includeSubDomains` directive (default: true)
- `APP_SECURITY_HEADERS_STS_PRELOAD` - Add HSTS `preload` directive (default: false)
- `APP_SECURITY_HEADERS_CUSTOM_FRAME_OPTIONS` - `X-Frame-Options` value (default: `DENY`)
- `APP_SECURITY_HEADERS_CONTENT_TYPE_NOSNIFF` - Emit `X-Content-Type-Options: nosniff` (default: true)
- `APP_SECURITY_HEADERS_CONTENT_SECURITY_POLICY` - `Content-Security-Policy` value
- `APP_SECURITY_HEADERS_REFERRER_POLICY` - `Referrer-Policy` value
- `APP_SECURITY_HEADERS_PERMISSIONS_POLICY` - `Permissions-Policy` value
- `APP_SECURITY_HEADERS_IE_NO_OPEN` - Emit `X-Download-Options: noopen` (default: true)
- `APP_SECURITY_HEADERS_X_DNS_PREFETCH_CONTROL` - `X-DNS-Prefetch-Control` value
- `APP_SECURITY_HEADERS_CROSS_ORIGIN_OPENER_POLICY` - `Cross-Origin-Opener-Policy` value
- `APP_SECURITY_HEADERS_CROSS_ORIGIN_RESOURCE_POLICY` - `Cross-Origin-Resource-Policy` value
- `APP_SECURITY_HEADERS_CROSS_ORIGIN_EMBEDDER_POLICY` - `Cross-Origin-Embedder-Policy` value

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `security_headers.enabled` | `APP_SECURITY_HEADERS_ENABLED` | |
| `security_headers.is_development` | `APP_SECURITY_HEADERS_IS_DEVELOPMENT` | |
| `security_headers.allowed_hosts` | `APP_SECURITY_HEADERS_ALLOWED_HOSTS` | |
| `security_headers.ssl_redirect` | `APP_SECURITY_HEADERS_SSL_REDIRECT` | |
| `security_headers.ssl_temporary_redirect` | `APP_SECURITY_HEADERS_SSL_TEMPORARY_REDIRECT` | |
| `security_headers.ssl_host` | `APP_SECURITY_HEADERS_SSL_HOST` | |
| `security_headers.dont_redirect_ipv4_hostnames` | `APP_SECURITY_HEADERS_DONT_REDIRECT_IPV4_HOSTNAMES` | |
| `security_headers.sts_seconds` | `APP_SECURITY_HEADERS_STS_SECONDS` | |
| `security_headers.sts_include_subdomains` | `APP_SECURITY_HEADERS_STS_INCLUDE_SUBDOMAINS` | |
| `security_headers.sts_preload` | `APP_SECURITY_HEADERS_STS_PRELOAD` | |
| `security_headers.custom_frame_options` | `APP_SECURITY_HEADERS_CUSTOM_FRAME_OPTIONS` | |
| `security_headers.content_type_nosniff` | `APP_SECURITY_HEADERS_CONTENT_TYPE_NOSNIFF` | |
| `security_headers.content_security_policy` | `APP_SECURITY_HEADERS_CONTENT_SECURITY_POLICY` | |
| `security_headers.referrer_policy` | `APP_SECURITY_HEADERS_REFERRER_POLICY` | |
| `security_headers.permissions_policy` | `APP_SECURITY_HEADERS_PERMISSIONS_POLICY` | |
| `security_headers.ie_no_open` | `APP_SECURITY_HEADERS_IE_NO_OPEN` | |
| `security_headers.x_dns_prefetch_control` | `APP_SECURITY_HEADERS_X_DNS_PREFETCH_CONTROL` | |
| `security_headers.cross_origin_opener_policy` | `APP_SECURITY_HEADERS_CROSS_ORIGIN_OPENER_POLICY` | |
| `security_headers.cross_origin_resource_policy` | `APP_SECURITY_HEADERS_CROSS_ORIGIN_RESOURCE_POLICY` | |
| `security_headers.cross_origin_embedder_policy` | `APP_SECURITY_HEADERS_CROSS_ORIGIN_EMBEDDER_POLICY` | |

#### Security HTTP Signature Configuration

- `APP_SECURITY_HTTP_SIGNATURE_ENABLED` - Enable HMAC request-signature middleware (default: false)
- `APP_SECURITY_HTTP_SIGNATURE_KEY_ID_HEADER` - Header containing key identifier (default: `X-Key-Id`)
- `APP_SECURITY_HTTP_SIGNATURE_TIMESTAMP_HEADER` - Header containing request timestamp (default: `X-Timestamp`)
- `APP_SECURITY_HTTP_SIGNATURE_NONCE_HEADER` - Header containing request nonce (default: `X-Nonce`)
- `APP_SECURITY_HTTP_SIGNATURE_SIGNATURE_HEADER` - Header containing base64 HMAC signature (default: `X-Signature`)
- `APP_SECURITY_HTTP_SIGNATURE_MAX_CLOCK_SKEW` - Max allowed clock drift (default: `5m`)
- `APP_SECURITY_HTTP_SIGNATURE_NONCE_TTL` - Nonce replay protection window (default: `10m`)
- `APP_SECURITY_HTTP_SIGNATURE_REQUIRE_NONCE` - Require nonce header/replay protection (default: true)
- `APP_SECURITY_HTTP_SIGNATURE_EXCLUDED_PATH_PREFIXES` - Comma-separated excluded path prefixes
- `security.http_signature.static_keys` - Key ID -> shared secret map (recommended via config file/secret manager)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `security.http_signature.enabled` | `APP_SECURITY_HTTP_SIGNATURE_ENABLED` | |
| `security.http_signature.key_id_header` | `APP_SECURITY_HTTP_SIGNATURE_KEY_ID_HEADER` | |
| `security.http_signature.timestamp_header` | `APP_SECURITY_HTTP_SIGNATURE_TIMESTAMP_HEADER` | |
| `security.http_signature.nonce_header` | `APP_SECURITY_HTTP_SIGNATURE_NONCE_HEADER` | |
| `security.http_signature.signature_header` | `APP_SECURITY_HTTP_SIGNATURE_SIGNATURE_HEADER` | |
| `security.http_signature.max_clock_skew` | `APP_SECURITY_HTTP_SIGNATURE_MAX_CLOCK_SKEW` | |
| `security.http_signature.nonce_ttl` | `APP_SECURITY_HTTP_SIGNATURE_NONCE_TTL` | |
| `security.http_signature.require_nonce` | `APP_SECURITY_HTTP_SIGNATURE_REQUIRE_NONCE` | |
| `security.http_signature.excluded_path_prefixes` | `APP_SECURITY_HTTP_SIGNATURE_EXCLUDED_PATH_PREFIXES` | |

#### I18n Configuration

- `APP_I18N_ENABLED` - Enable locale middleware + translator lookup (default: false)
- `APP_I18N_DEFAULT_LOCALE` - Default locale fallback (default: `en`)
- `APP_I18N_SUPPORTED_LOCALES` - Comma-separated locales accepted by matcher (default: `en`)
- `APP_I18N_QUERY_PARAM` - Query parameter used for locale override (default: `lang`)
- `APP_I18N_HEADER_NAME` - Header used for explicit locale override (default: `X-Locale`)
- `APP_I18N_FALLBACK_MODE` - Fallback mode: `base`, `default` (default: `base`)
- `APP_I18N_CATALOG_PATH` - Directory path containing locale catalogs (`.json`/`.yaml`)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `i18n.enabled` | `APP_I18N_ENABLED` | |
| `i18n.default_locale` | `APP_I18N_DEFAULT_LOCALE` | |
| `i18n.supported_locales` | `APP_I18N_SUPPORTED_LOCALES` | |
| `i18n.query_param` | `APP_I18N_QUERY_PARAM` | |
| `i18n.header_name` | `APP_I18N_HEADER_NAME` | |
| `i18n.fallback_mode` | `APP_I18N_FALLBACK_MODE` | |
| `i18n.catalog_path` | `APP_I18N_CATALOG_PATH` | |

#### Session Configuration

- `APP_SESSION_ENABLED` - Enable server-side sessions (default: false)
- `APP_SESSION_STORE` - Session backend: `inmemory`, `redis`, `memcached` (default: `inmemory`)
- `APP_SESSION_TTL` - Absolute session lifetime (default: 12h)
- `APP_SESSION_IDLE_TIMEOUT` - Sliding inactivity timeout (default: 30m)
- `APP_SESSION_COOKIE_NAME` - Session cookie name (default: `sid`)
- `APP_SESSION_COOKIE_PATH` - Session cookie path (default: `/`)
- `APP_SESSION_COOKIE_DOMAIN` - Session cookie domain (optional)
- `APP_SESSION_COOKIE_SECURE` - Use secure session cookie (default: true)
- `APP_SESSION_COOKIE_HTTP_ONLY` - Hide session cookie from JS (default: true)
- `APP_SESSION_COOKIE_SAME_SITE` - `lax`, `strict`, `none` (default: `lax`)
- `APP_SESSION_AUTO_CREATE` - Create session automatically when missing (default: true)
- `APP_SESSION_REDIS_URL` - Redis DSN when `session.store=redis`
- `APP_SESSION_REDIS_MAX_CONNS` - Redis pool size (default: 10)
- `APP_SESSION_REDIS_OPERATION_TIMEOUT` - Redis timeout (default: 5s)
- `APP_SESSION_REDIS_PREFIX` - Redis key prefix (default: `session`)
- `APP_SESSION_MEMCACHED_ADDRESSES` - Comma-separated memcached endpoints when `session.store=memcached`
- `APP_SESSION_MEMCACHED_TIMEOUT` - Memcached operation timeout (default: 500ms)
- `APP_SESSION_MEMCACHED_PREFIX` - Memcached key prefix (default: `session`)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `session.enabled` | `APP_SESSION_ENABLED` | |
| `session.store` | `APP_SESSION_STORE` | |
| `session.ttl` | `APP_SESSION_TTL` | |
| `session.idle_timeout` | `APP_SESSION_IDLE_TIMEOUT` | |
| `session.cookie_name` | `APP_SESSION_COOKIE_NAME` | |
| `session.cookie_path` | `APP_SESSION_COOKIE_PATH` | |
| `session.cookie_domain` | `APP_SESSION_COOKIE_DOMAIN` | |
| `session.cookie_secure` | `APP_SESSION_COOKIE_SECURE` | |
| `session.cookie_http_only` | `APP_SESSION_COOKIE_HTTP_ONLY` | |
| `session.cookie_same_site` | `APP_SESSION_COOKIE_SAME_SITE` | |
| `session.auto_create` | `APP_SESSION_AUTO_CREATE` | |
| `session.redis.url` | `APP_SESSION_REDIS_URL` | |
| `session.redis.max_conns` | `APP_SESSION_REDIS_MAX_CONNS` | |
| `session.redis.operation_timeout` | `APP_SESSION_REDIS_OPERATION_TIMEOUT` | |
| `session.redis.prefix` | `APP_SESSION_REDIS_PREFIX` | |
| `session.memcached.addresses` | `APP_SESSION_MEMCACHED_ADDRESSES` | |
| `session.memcached.timeout` | `APP_SESSION_MEMCACHED_TIMEOUT` | |
| `session.memcached.prefix` | `APP_SESSION_MEMCACHED_PREFIX` | |

#### CSRF Configuration

- `APP_CSRF_ENABLED` - Enable explicit CSRF middleware (default: false)
- `APP_CSRF_HEADER_NAME` - CSRF request header name (default: `X-CSRF-Token`)
- `APP_CSRF_COOKIE_NAME` - CSRF cookie name (default: `XSRF-TOKEN`)
- `APP_CSRF_COOKIE_PATH` - CSRF cookie path (default: `/`)
- `APP_CSRF_COOKIE_DOMAIN` - CSRF cookie domain (optional)
- `APP_CSRF_COOKIE_SECURE` - Use secure CSRF cookie (default: true)
- `APP_CSRF_COOKIE_SAME_SITE` - `lax`, `strict`, `none` (default: `lax`)
- `APP_CSRF_COOKIE_TTL` - CSRF cookie TTL (default: 12h)
- `APP_CSRF_EXEMPT_METHODS` - Comma-separated HTTP methods exempt from CSRF checks
- `APP_CSRF_EXEMPT_PATHS` - Comma-separated path prefixes exempt from CSRF checks

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `csrf.enabled` | `APP_CSRF_ENABLED` | |
| `csrf.header_name` | `APP_CSRF_HEADER_NAME` | |
| `csrf.cookie_name` | `APP_CSRF_COOKIE_NAME` | |
| `csrf.cookie_path` | `APP_CSRF_COOKIE_PATH` | |
| `csrf.cookie_domain` | `APP_CSRF_COOKIE_DOMAIN` | |
| `csrf.cookie_secure` | `APP_CSRF_COOKIE_SECURE` | |
| `csrf.cookie_same_site` | `APP_CSRF_COOKIE_SAME_SITE` | |
| `csrf.cookie_ttl` | `APP_CSRF_COOKIE_TTL` | |
| `csrf.exempt_methods` | `APP_CSRF_EXEMPT_METHODS` | |
| `csrf.exempt_paths` | `APP_CSRF_EXEMPT_PATHS` | |

#### Authentication Configuration

- `APP_AUTH_ENABLED` - Enable OAuth2 authentication (default: false)
- `APP_AUTH_ISSUER` - OAuth2 issuer URL (required if auth enabled)
- `APP_AUTH_JWKS_URL` - JWKS endpoint URL (required if auth enabled)
- `APP_AUTH_JWKS_CACHE_TTL` - JWKS cache TTL (default: 1h)
- `APP_AUTH_AUDIENCE` - Expected audience claim (required if auth enabled)
- `auth.claims.mappings` - Claim alias mappings used by JWT extraction (configured via file)
- `auth.claims.rules` - Claim guard rules configured via file (`claim`, `operator`, `source`, `key`, `values`, `optional`)
  - Required when `auth.enabled=true` (breaking change)

#### Email Configuration

- `APP_EMAIL_ENABLED` - Enable email subsystem (default: false)
- `APP_EMAIL_PROVIDER` - Provider: `smtp`, `ses`, `sendgrid`, `mailgun`, `mailchimp`, `mailersend`, `postmark`, `mailtrap`, `smtp2go`, `sendpulse`, `brevo`, `mailjet` (default: `smtp`)
- `APP_EMAIL_SMTP_HOST` - SMTP host (required when `provider=smtp`)
- `APP_EMAIL_SMTP_PORT` - SMTP port (default: 587)
- `APP_EMAIL_SMTP_USERNAME` - SMTP username (optional)
- `APP_EMAIL_SMTP_PASSWORD` - SMTP password (optional)
- `APP_EMAIL_SMTP_FROM` - Default sender email for SMTP
- `APP_EMAIL_SMTP_ENABLE_TLS` - Enable STARTTLS/TLS for SMTP (default: true)
- `APP_EMAIL_SMTP_INSECURE_SKIP_VERIFY` - Skip TLS cert verification for SMTP (default: false)
- `APP_EMAIL_SMTP_OPERATION_TIMEOUT` - SMTP operation timeout (default: 10s)
- `APP_EMAIL_SES_REGION` - AWS SES region (required when `provider=ses`)
- `APP_EMAIL_SES_ENDPOINT` - Optional SES custom endpoint
- `APP_EMAIL_SES_ACCESS_KEY_ID` - Optional static AWS access key ID
- `APP_EMAIL_SES_SECRET_ACCESS_KEY` - Optional static AWS secret key
- `APP_EMAIL_SES_SESSION_TOKEN` - Optional static AWS session token
- `APP_EMAIL_SES_FROM` - Default sender email for SES
- `APP_EMAIL_SES_OPERATION_TIMEOUT` - SES operation timeout (default: 10s)
- `APP_EMAIL_SENDGRID_TOKEN` - SendGrid API token (required when `provider=sendgrid`)
- `APP_EMAIL_SENDGRID_FROM` - Default sender email for SendGrid
- `APP_EMAIL_SENDGRID_BASE_URL` - SendGrid API base URL (default: `https://api.sendgrid.com`)
- `APP_EMAIL_SENDGRID_OPERATION_TIMEOUT` - SendGrid operation timeout (default: 10s)
- `APP_EMAIL_MAILGUN_TOKEN` - Mailgun API token (required when `provider=mailgun`)
- `APP_EMAIL_MAILGUN_DOMAIN` - Mailgun domain (required when `provider=mailgun`)
- `APP_EMAIL_MAILGUN_FROM` - Default sender email for Mailgun
- `APP_EMAIL_MAILGUN_BASE_URL` - Mailgun API base URL (default: `https://api.mailgun.net`)
- `APP_EMAIL_MAILGUN_OPERATION_TIMEOUT` - Mailgun operation timeout (default: 10s)
- `APP_EMAIL_MAILCHIMP_TOKEN` - Mailchimp/Mandrill API token (required when `provider=mailchimp`)
- `APP_EMAIL_MAILCHIMP_FROM` - Default sender email for Mailchimp
- `APP_EMAIL_MAILCHIMP_BASE_URL` - Mailchimp API base URL (default: `https://mandrillapp.com`)
- `APP_EMAIL_MAILCHIMP_OPERATION_TIMEOUT` - Mailchimp operation timeout (default: 10s)
- `APP_EMAIL_MAILERSEND_TOKEN` - MailerSend API token (required when `provider=mailersend`)
- `APP_EMAIL_MAILERSEND_FROM` - Default sender email for MailerSend
- `APP_EMAIL_MAILERSEND_BASE_URL` - MailerSend API base URL (default: `https://api.mailersend.com`)
- `APP_EMAIL_MAILERSEND_OPERATION_TIMEOUT` - MailerSend operation timeout (default: 10s)
- `APP_EMAIL_POSTMARK_SERVER_TOKEN` - Postmark server token (required when `provider=postmark`)
- `APP_EMAIL_POSTMARK_FROM` - Default sender email for Postmark
- `APP_EMAIL_POSTMARK_BASE_URL` - Postmark API base URL (default: `https://api.postmarkapp.com`)
- `APP_EMAIL_POSTMARK_OPERATION_TIMEOUT` - Postmark operation timeout (default: 10s)
- `APP_EMAIL_MAILTRAP_TOKEN` - Mailtrap API token (required when `provider=mailtrap`)
- `APP_EMAIL_MAILTRAP_FROM` - Default sender email for Mailtrap
- `APP_EMAIL_MAILTRAP_BASE_URL` - Mailtrap API base URL (default: `https://send.api.mailtrap.io`)
- `APP_EMAIL_MAILTRAP_OPERATION_TIMEOUT` - Mailtrap operation timeout (default: 10s)
- `APP_EMAIL_SMTP2GO_TOKEN` - SMTP2GO API token (required when `provider=smtp2go`)
- `APP_EMAIL_SMTP2GO_FROM` - Default sender email for SMTP2GO
- `APP_EMAIL_SMTP2GO_BASE_URL` - SMTP2GO API base URL (default: `https://api.smtp2go.com`)
- `APP_EMAIL_SMTP2GO_OPERATION_TIMEOUT` - SMTP2GO operation timeout (default: 10s)
- `APP_EMAIL_SENDPULSE_TOKEN` - SendPulse API token (required when `provider=sendpulse`)
- `APP_EMAIL_SENDPULSE_FROM` - Default sender email for SendPulse
- `APP_EMAIL_SENDPULSE_BASE_URL` - SendPulse API base URL (default: `https://api.sendpulse.com`)
- `APP_EMAIL_SENDPULSE_OPERATION_TIMEOUT` - SendPulse operation timeout (default: 10s)
- `APP_EMAIL_BREVO_TOKEN` - Brevo API token (required when `provider=brevo`)
- `APP_EMAIL_BREVO_FROM` - Default sender email for Brevo
- `APP_EMAIL_BREVO_BASE_URL` - Brevo API base URL (default: `https://api.brevo.com`)
- `APP_EMAIL_BREVO_OPERATION_TIMEOUT` - Brevo operation timeout (default: 10s)
- `APP_EMAIL_MAILJET_API_KEY` - Mailjet API key (required when `provider=mailjet`)
- `APP_EMAIL_MAILJET_API_SECRET` - Mailjet API secret (required when `provider=mailjet`)
- `APP_EMAIL_MAILJET_FROM` - Default sender email for Mailjet
- `APP_EMAIL_MAILJET_BASE_URL` - Mailjet API base URL (default: `https://api.mailjet.com`)
- `APP_EMAIL_MAILJET_OPERATION_TIMEOUT` - Mailjet operation timeout (default: 10s)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `email.enabled` | `APP_EMAIL_ENABLED` | |
| `email.provider` | `APP_EMAIL_PROVIDER` | |
| `email.smtp.host` | `APP_EMAIL_SMTP_HOST` | |
| `email.smtp.port` | `APP_EMAIL_SMTP_PORT` | |
| `email.smtp.username` | `APP_EMAIL_SMTP_USERNAME` | |
| `email.smtp.password` | `APP_EMAIL_SMTP_PASSWORD` | |
| `email.smtp.from` | `APP_EMAIL_SMTP_FROM` | |
| `email.smtp.enable_tls` | `APP_EMAIL_SMTP_ENABLE_TLS` | |
| `email.smtp.insecure_skip_verify` | `APP_EMAIL_SMTP_INSECURE_SKIP_VERIFY` | |
| `email.smtp.operation_timeout` | `APP_EMAIL_SMTP_OPERATION_TIMEOUT` | |
| `email.ses.region` | `APP_EMAIL_SES_REGION` | |
| `email.ses.endpoint` | `APP_EMAIL_SES_ENDPOINT` | |
| `email.ses.access_key_id` | `APP_EMAIL_SES_ACCESS_KEY_ID` | |
| `email.ses.secret_access_key` | `APP_EMAIL_SES_SECRET_ACCESS_KEY` | |
| `email.ses.session_token` | `APP_EMAIL_SES_SESSION_TOKEN` | |
| `email.ses.from` | `APP_EMAIL_SES_FROM` | |
| `email.ses.operation_timeout` | `APP_EMAIL_SES_OPERATION_TIMEOUT` | |
| `email.sendgrid.token` | `APP_EMAIL_SENDGRID_TOKEN` | |
| `email.sendgrid.from` | `APP_EMAIL_SENDGRID_FROM` | |
| `email.sendgrid.base_url` | `APP_EMAIL_SENDGRID_BASE_URL` | |
| `email.sendgrid.operation_timeout` | `APP_EMAIL_SENDGRID_OPERATION_TIMEOUT` | |
| `email.mailgun.token` | `APP_EMAIL_MAILGUN_TOKEN` | |
| `email.mailgun.domain` | `APP_EMAIL_MAILGUN_DOMAIN` | |
| `email.mailgun.from` | `APP_EMAIL_MAILGUN_FROM` | |
| `email.mailgun.base_url` | `APP_EMAIL_MAILGUN_BASE_URL` | |
| `email.mailgun.operation_timeout` | `APP_EMAIL_MAILGUN_OPERATION_TIMEOUT` | |
| `email.mailchimp.token` | `APP_EMAIL_MAILCHIMP_TOKEN` | |
| `email.mailchimp.from` | `APP_EMAIL_MAILCHIMP_FROM` | |
| `email.mailchimp.base_url` | `APP_EMAIL_MAILCHIMP_BASE_URL` | |
| `email.mailchimp.operation_timeout` | `APP_EMAIL_MAILCHIMP_OPERATION_TIMEOUT` | |
| `email.mailersend.token` | `APP_EMAIL_MAILERSEND_TOKEN` | |
| `email.mailersend.from` | `APP_EMAIL_MAILERSEND_FROM` | |
| `email.mailersend.base_url` | `APP_EMAIL_MAILERSEND_BASE_URL` | |
| `email.mailersend.operation_timeout` | `APP_EMAIL_MAILERSEND_OPERATION_TIMEOUT` | |
| `email.postmark.server_token` | `APP_EMAIL_POSTMARK_SERVER_TOKEN` | |
| `email.postmark.from` | `APP_EMAIL_POSTMARK_FROM` | |
| `email.postmark.base_url` | `APP_EMAIL_POSTMARK_BASE_URL` | |
| `email.postmark.operation_timeout` | `APP_EMAIL_POSTMARK_OPERATION_TIMEOUT` | |
| `email.mailtrap.token` | `APP_EMAIL_MAILTRAP_TOKEN` | |
| `email.mailtrap.from` | `APP_EMAIL_MAILTRAP_FROM` | |
| `email.mailtrap.base_url` | `APP_EMAIL_MAILTRAP_BASE_URL` | |
| `email.mailtrap.operation_timeout` | `APP_EMAIL_MAILTRAP_OPERATION_TIMEOUT` | |
| `email.smtp2go.token` | `APP_EMAIL_SMTP2GO_TOKEN` | |
| `email.smtp2go.from` | `APP_EMAIL_SMTP2GO_FROM` | |
| `email.smtp2go.base_url` | `APP_EMAIL_SMTP2GO_BASE_URL` | |
| `email.smtp2go.operation_timeout` | `APP_EMAIL_SMTP2GO_OPERATION_TIMEOUT` | |
| `email.sendpulse.token` | `APP_EMAIL_SENDPULSE_TOKEN` | |
| `email.sendpulse.from` | `APP_EMAIL_SENDPULSE_FROM` | |
| `email.sendpulse.base_url` | `APP_EMAIL_SENDPULSE_BASE_URL` | |
| `email.sendpulse.operation_timeout` | `APP_EMAIL_SENDPULSE_OPERATION_TIMEOUT` | |
| `email.brevo.token` | `APP_EMAIL_BREVO_TOKEN` | |
| `email.brevo.from` | `APP_EMAIL_BREVO_FROM` | |
| `email.brevo.base_url` | `APP_EMAIL_BREVO_BASE_URL` | |
| `email.brevo.operation_timeout` | `APP_EMAIL_BREVO_OPERATION_TIMEOUT` | |
| `email.mailjet.api_key` | `APP_EMAIL_MAILJET_API_KEY` | |
| `email.mailjet.api_secret` | `APP_EMAIL_MAILJET_API_SECRET` | |
| `email.mailjet.from` | `APP_EMAIL_MAILJET_FROM` | |
| `email.mailjet.base_url` | `APP_EMAIL_MAILJET_BASE_URL` | |
| `email.mailjet.operation_timeout` | `APP_EMAIL_MAILJET_OPERATION_TIMEOUT` | |

#### Database Configuration

- `APP_DB_TYPE` - Database type: postgres, mysql, mongodb, dynamodb
- `APP_DB_URL` - Database connection URL (required if type specified)
- `APP_DB_MAX_OPEN_CONNS` - Max open connections (default: 25)
- `APP_DB_MAX_IDLE_CONNS` - Max idle connections (default: 5)
- `APP_DB_CONN_MAX_LIFETIME` - Connection max lifetime (default: 5m)
- `APP_DB_QUERY_TIMEOUT` - Query timeout (default: 10s)
- `APP_DB_DATABASE_NAME` - MongoDB database name (required for mongodb)
- `APP_DB_REGION` - AWS region (required for dynamodb)
- `APP_DB_ENDPOINT` - Custom endpoint (optional, e.g. LocalStack)
- `APP_DB_ACCESS_KEY_ID` - AWS access key (optional)
- `APP_DB_SECRET_ACCESS_KEY` - AWS secret key (optional)
- `APP_DB_SESSION_TOKEN` - AWS session token (optional)
  Legacy aliases:
  all `APP_DATABASE_*` equivalents are supported for backward compatibility.

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `database.type` | `APP_DB_TYPE` | `APP_DATABASE_TYPE` |
| `database.url` | `APP_DB_URL` | `APP_DATABASE_URL` |
| `database.max_open_conns` | `APP_DB_MAX_OPEN_CONNS` | `APP_DATABASE_MAX_OPEN_CONNS` |
| `database.max_idle_conns` | `APP_DB_MAX_IDLE_CONNS` | `APP_DATABASE_MAX_IDLE_CONNS` |
| `database.conn_max_lifetime` | `APP_DB_CONN_MAX_LIFETIME` | `APP_DATABASE_CONN_MAX_LIFETIME` |
| `database.query_timeout` | `APP_DB_QUERY_TIMEOUT` | `APP_DATABASE_QUERY_TIMEOUT` |
| `database.database_name` | `APP_DB_DATABASE_NAME` | `APP_DATABASE_DATABASE_NAME` |
| `database.connect_timeout` | `APP_DB_CONNECT_TIMEOUT` | `APP_DATABASE_CONNECT_TIMEOUT` |
| `database.region` | `APP_DB_REGION` | `APP_DATABASE_REGION` |
| `database.endpoint` | `APP_DB_ENDPOINT` | `APP_DATABASE_ENDPOINT` |
| `database.access_key_id` | `APP_DB_ACCESS_KEY_ID` | `APP_DATABASE_ACCESS_KEY_ID` |
| `database.secret_access_key` | `APP_DB_SECRET_ACCESS_KEY` | `APP_DATABASE_SECRET_ACCESS_KEY` |
| `database.session_token` | `APP_DB_SESSION_TOKEN` | `APP_DATABASE_SESSION_TOKEN` |

#### Cache Configuration

- `APP_CACHE_TYPE` - Cache type: redis, inmemory
- `APP_CACHE_URL` - Cache connection URL (required for redis)
- `APP_CACHE_MAX_CONNS` - Max connections (default: 10)
- `APP_CACHE_OPERATION_TIMEOUT` - Operation timeout (default: 5s)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `cache.type` | `APP_CACHE_TYPE` | |
| `cache.url` | `APP_CACHE_URL` | |
| `cache.max_conns` | `APP_CACHE_MAX_CONNS` | |
| `cache.operation_timeout` | `APP_CACHE_OPERATION_TIMEOUT` | |

#### Object Storage Configuration

- `APP_OBJECT_STORAGE_ENABLED` - Enable object storage adapter initialization (default: false)
- `APP_OBJECT_STORAGE_TYPE` - Object storage type: `s3` (default: `s3`)
- `APP_OBJECT_STORAGE_S3_BUCKET` - S3 bucket name (required when object storage is enabled)
- `APP_OBJECT_STORAGE_S3_REGION` - AWS region (required when object storage is enabled)
- `APP_OBJECT_STORAGE_S3_ENDPOINT` - Optional custom endpoint (R2, MinIO, LocalStack, etc.)
- `APP_OBJECT_STORAGE_S3_ACCESS_KEY_ID` - Static access key (optional)
- `APP_OBJECT_STORAGE_S3_SECRET_ACCESS_KEY` - Static secret key (optional)
- `APP_OBJECT_STORAGE_S3_SESSION_TOKEN` - Static session token (optional)
- `APP_OBJECT_STORAGE_S3_USE_PATH_STYLE` - Force path-style addressing (default: false)
- `APP_OBJECT_STORAGE_S3_OPERATION_TIMEOUT` - Operation timeout (default: 10s)
- `APP_OBJECT_STORAGE_S3_PRESIGN_EXPIRY` - Default presigned URL expiration (default: 15m)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `object_storage.enabled` | `APP_OBJECT_STORAGE_ENABLED` | |
| `object_storage.type` | `APP_OBJECT_STORAGE_TYPE` | |
| `object_storage.s3.bucket` | `APP_OBJECT_STORAGE_S3_BUCKET` | |
| `object_storage.s3.region` | `APP_OBJECT_STORAGE_S3_REGION` | |
| `object_storage.s3.endpoint` | `APP_OBJECT_STORAGE_S3_ENDPOINT` | |
| `object_storage.s3.access_key_id` | `APP_OBJECT_STORAGE_S3_ACCESS_KEY_ID` | |
| `object_storage.s3.secret_access_key` | `APP_OBJECT_STORAGE_S3_SECRET_ACCESS_KEY` | |
| `object_storage.s3.session_token` | `APP_OBJECT_STORAGE_S3_SESSION_TOKEN` | |
| `object_storage.s3.use_path_style` | `APP_OBJECT_STORAGE_S3_USE_PATH_STYLE` | |
| `object_storage.s3.operation_timeout` | `APP_OBJECT_STORAGE_S3_OPERATION_TIMEOUT` | |
| `object_storage.s3.presign_expiry` | `APP_OBJECT_STORAGE_S3_PRESIGN_EXPIRY` | |

#### Search Store Configuration

- `APP_SEARCH_TYPE` - Search backend type: `opensearch`, `elasticsearch`
- `APP_SEARCH_DRIVER` - Client driver: `http`, `opensearch-sdk`, `elasticsearch-sdk` (default: `http`)
- `APP_SEARCH_URL` - Primary search backend base URL
- `APP_SEARCH_URLS` - Comma-separated node URLs for cluster fallback (optional)
- `APP_SEARCH_USERNAME` - Basic auth username (optional)
- `APP_SEARCH_PASSWORD` - Basic auth password (optional)
- `APP_SEARCH_API_KEY` - API key auth token (optional, has precedence over basic auth)
- `APP_SEARCH_AWS_AUTH_ENABLED` - Enable AWS SigV4 authentication (default: false)
- `APP_SEARCH_AWS_REGION` - AWS region (required when AWS auth is enabled)
- `APP_SEARCH_AWS_SERVICE` - AWS service name (default: `es`, use `aoss` for OpenSearch Serverless)
- `APP_SEARCH_AWS_ACCESS_KEY_ID` - Static AWS access key id (optional)
- `APP_SEARCH_AWS_SECRET_ACCESS_KEY` - Static AWS secret access key (optional)
- `APP_SEARCH_AWS_SESSION_TOKEN` - Static AWS session token (optional)
- `APP_SEARCH_MAX_CONNS` - Max pooled HTTP connections (default: 10)
- `APP_SEARCH_OPERATION_TIMEOUT` - Operation timeout (default: 5s)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `search.type` | `APP_SEARCH_TYPE` | |
| `search.driver` | `APP_SEARCH_DRIVER` | |
| `search.url` | `APP_SEARCH_URL` | |
| `search.urls` | `APP_SEARCH_URLS` | |
| `search.username` | `APP_SEARCH_USERNAME` | |
| `search.password` | `APP_SEARCH_PASSWORD` | |
| `search.api_key` | `APP_SEARCH_API_KEY` | |
| `search.aws_auth_enabled` | `APP_SEARCH_AWS_AUTH_ENABLED` | |
| `search.aws_region` | `APP_SEARCH_AWS_REGION` | |
| `search.aws_service` | `APP_SEARCH_AWS_SERVICE` | |
| `search.aws_access_key_id` | `APP_SEARCH_AWS_ACCESS_KEY_ID` | |
| `search.aws_secret_access_key` | `APP_SEARCH_AWS_SECRET_ACCESS_KEY` | |
| `search.aws_session_token` | `APP_SEARCH_AWS_SESSION_TOKEN` | |
| `search.max_conns` | `APP_SEARCH_MAX_CONNS` | |
| `search.operation_timeout` | `APP_SEARCH_OPERATION_TIMEOUT` | |

#### Rate Limiting Configuration

- `APP_RATE_LIMIT_ENABLED` - Enable rate limiting middleware (default: false)
- `APP_RATE_LIMIT_TYPE` - Rate limiter backend: `local`, `redis` (future `memcached`)
- `APP_RATE_LIMIT_REQUESTS_PER_SECOND` - Allowed requests per second (default: 100)
- `APP_RATE_LIMIT_BURST` - Burst allowance beyond the per-second rate (default: 20)
- `APP_RATE_LIMIT_WINDOW` - Sliding window size for redis limiter (default: 1s)
- `APP_RATE_LIMIT_REDIS_URL` - Redis connection URL for shared rate limiting
- `APP_RATE_LIMIT_REDIS_MAX_CONNS` - Redis connection pool size (default: 10)
- `APP_RATE_LIMIT_REDIS_OPERATION_TIMEOUT` - Redis operation timeout (default: 5s)
- `APP_RATE_LIMIT_REDIS_PREFIX` - Redis key prefix used by the distributed limiter (default: `ratelimit`)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `rate_limit.enabled` | `APP_RATE_LIMIT_ENABLED` | |
| `rate_limit.type` | `APP_RATE_LIMIT_TYPE` | |
| `rate_limit.requests_per_second` | `APP_RATE_LIMIT_REQUESTS_PER_SECOND` | |
| `rate_limit.burst` | `APP_RATE_LIMIT_BURST` | |
| `rate_limit.window` | `APP_RATE_LIMIT_WINDOW` | |
| `rate_limit.redis.url` | `APP_RATE_LIMIT_REDIS_URL` | |
| `rate_limit.redis.max_conns` | `APP_RATE_LIMIT_REDIS_MAX_CONNS` | |
| `rate_limit.redis.operation_timeout` | `APP_RATE_LIMIT_REDIS_OPERATION_TIMEOUT` | |
| `rate_limit.redis.prefix` | `APP_RATE_LIMIT_REDIS_PREFIX` | |

#### Event Bus Configuration

- `APP_EVENTBUS_TYPE` - Event bus type: kafka, rabbitmq, sqs
- `APP_EVENTBUS_BROKERS` - Comma-separated broker addresses
- `APP_EVENTBUS_SERIALIZER` - Message serializer: json, protobuf, avro (default: json)
- `APP_EVENTBUS_OPERATION_TIMEOUT` - Operation timeout (default: 30s)
- `APP_EVENTBUS_GROUP_ID` - Kafka consumer group (default: default-consumer-group)
- `APP_EVENTBUS_URL` - RabbitMQ connection URL
- `APP_EVENTBUS_EXCHANGE` - RabbitMQ exchange name (default: events)
- `APP_EVENTBUS_EXCHANGE_TYPE` - RabbitMQ exchange type (default: topic)
- `APP_EVENTBUS_QUEUE_NAME` - RabbitMQ queue name (optional)
- `APP_EVENTBUS_ROUTING_KEY` - RabbitMQ routing key (optional)
- `APP_EVENTBUS_CONSUMER_TAG` - RabbitMQ consumer tag (optional)
- `APP_EVENTBUS_REGION` - AWS region (required for sqs)
- `APP_EVENTBUS_QUEUE_URL` - SQS queue URL (required for sqs)
- `APP_EVENTBUS_ENDPOINT` - Custom endpoint (optional, e.g. LocalStack)
- `APP_EVENTBUS_ACCESS_KEY_ID` - AWS access key (optional)
- `APP_EVENTBUS_SECRET_ACCESS_KEY` - AWS secret key (optional)
- `APP_EVENTBUS_SESSION_TOKEN` - AWS session token (optional)
- `APP_EVENTBUS_WAIT_TIME_SECONDS` - SQS long polling wait (default: 10)
- `APP_EVENTBUS_MAX_MESSAGES` - SQS max receive batch (default: 10)
- `APP_EVENTBUS_VISIBILITY_TIMEOUT` - SQS visibility timeout (seconds)

#### Jobs Configuration

- `APP_JOBS_BACKEND` - Jobs runtime backend: `eventbus` (default: `eventbus`)

| Config key | Env ufficiale | Alias legacy |
| --- | --- | --- |
| `jobs.backend` | `APP_JOBS_BACKEND` | |

#### Observability Configuration

- `APP_SERVICE_NAME` - Service name for the application identity.
  With `cli.NewServiceCommand`, if not set, the default is `ServiceCommandOptions.Name`.
- `APP_OBSERVABILITY_LOG_LEVEL` - Log level: debug, info, warn, error (default: info)
- `APP_OBSERVABILITY_LOG_FORMAT` - Log format: json, text (default: json)
- `APP_OBSERVABILITY_SERVICE_NAME` - Optional tracing service-name override (default: falls back to `service.name`)
- `APP_OBSERVABILITY_TRACING_ENABLED` - Enable tracing (default: false)
- `APP_OBSERVABILITY_TRACING_SAMPLE_RATE` - Trace sample rate (default: 0.1)
- `APP_OBSERVABILITY_TRACING_ENDPOINT` - OpenTelemetry endpoint (required if tracing enabled)
- `APP_OBSERVABILITY_REQUEST_LOGGING_OUTPUT` - Request log destination: logger, stdout, stderr (default: logger)
- `APP_OBSERVABILITY_REQUEST_LOGGING_FIELDS` - Comma-separated request log fields.
  Supported fields: `request_id`, `method`, `path`, `status`, `duration_ms`, `error`, `remote_addr`, `remote_port`, `request_method`, `request_uri`, `uri`, `args`, `query_string`, `request_time`, `time_local`, `host`, `server_protocol`, `scheme`, `http_referer`, `http_user_agent`, `x_forwarded_for`, `remote_user`, `request_length`

#### Swagger Configuration

- `APP_SWAGGER_ENABLED` - Enable Swagger UI (default: false)
- `APP_SWAGGER_SPEC_PATH` - OpenAPI spec path (default: /api/openapi/openapi.yaml)

#### Validation Configuration

Kafka schema validation contract (local descriptor-based):

- `APP_VALIDATION_KAFKA_ENABLED` - Enable schema-validation hooks (default: false)
- `APP_VALIDATION_KAFKA_MODE` - Validation mode: `warn`, `enforce` (default: `enforce`)
- `APP_VALIDATION_KAFKA_DESCRIPTOR_PATH` - Descriptor-set path (default: `events/proto/schema.pb`)
- `APP_VALIDATION_KAFKA_DEFAULT_POLICY` - Default compatibility policy: `BACKWARD`, `FULL` (default: `BACKWARD`)

## Configuration Files

The framework supports YAML, JSON, and TOML configuration files. YAML is recommended for readability.

### Example YAML Configuration

```yaml
service:
  # optional with cli.NewServiceCommand, defaults to ServiceCommandOptions.Name
  name: "my-service"
  environment: "development"

http:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s

management:
  port: 9090
  read_timeout: 10s
  write_timeout: 10s
  auth_enabled: false
  mtls_enabled: false
  tls_cert_file: ""
  tls_key_file: ""
  tls_ca_file: ""

security_headers:
  enabled: true
  is_development: false
  allowed_hosts: []
  ssl_redirect: false
  ssl_temporary_redirect: false
  ssl_host: ""
  ssl_proxy_headers:
    X-Forwarded-Proto: "https"
  dont_redirect_ipv4_hostnames: true
  sts_seconds: 31536000
  sts_include_subdomains: true
  sts_preload: false
  custom_frame_options: "DENY"
  content_type_nosniff: true
  content_security_policy: "default-src 'self'"
  referrer_policy: "strict-origin-when-cross-origin"
  permissions_policy: "geolocation=(), microphone=(), camera=()"
  ie_no_open: true
  x_dns_prefetch_control: "off"
  cross_origin_opener_policy: "same-origin"
  cross_origin_resource_policy: "same-origin"
  cross_origin_embedder_policy: ""
  custom_headers: {}

session:
  enabled: true
  store: "memcached"
  ttl: 12h
  idle_timeout: 30m
  cookie_name: "sid"
  cookie_path: "/"
  cookie_domain: ""
  cookie_secure: true
  cookie_http_only: true
  cookie_same_site: "lax"
  auto_create: true
  memcached:
    addresses:
      - "memcached-1.internal:11211"
      - "memcached-2.internal:11211"
    timeout: 500ms
    prefix: "session"

csrf:
  enabled: true
  header_name: "X-CSRF-Token"
  cookie_name: "XSRF-TOKEN"
  cookie_path: "/"
  cookie_domain: ""
  cookie_secure: true
  cookie_same_site: "lax"
  cookie_ttl: 12h
  exempt_methods: ["GET", "HEAD", "OPTIONS", "TRACE"]
  exempt_paths: ["/health", "/metrics"]

auth:
  enabled: true
  issuer: "https://auth.example.com"
  jwks_url: "https://auth.example.com/.well-known/jwks.json"
  jwks_cache_ttl: 1h
  audience: "my-service"
  claims:
    mappings:
      subject:
        - "sub"
      issuer:
        - "iss"
      audience:
        - "aud"
      scopes:
        - "scope"
        - "scopes"
      roles:
        - "role"
        - "roles"
      tenant_id:
        - "tenant_id"
        - "tenantId"
        - "https://example.localhost/tenant_id"
    rules:
      - claim: "tenant_id"
        aliases:
          - "tenantId"
          - "https://example.localhost/tenant_id"
        operator: "required"
        optional: false
      - claim: "tenant_id"
        operator: "equals"
        source: "header"
        key: "X-Tenant-ID"
        optional: false

database:
  type: "postgres"
  url: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
  query_timeout: 10s

cache:
  type: "redis"
  url: "redis://localhost:6379/0"
  max_conns: 10
  operation_timeout: 5s

search:
  type: "opensearch"
  driver: "http"
  url: "http://localhost:9200"
  urls:
    - "http://localhost:9200"
    - "http://localhost:9201"
  username: "elastic"
  password: "changeme"
  api_key: ""
  aws_auth_enabled: false
  aws_region: "eu-west-1"
  aws_service: "es"
  aws_access_key_id: ""
  aws_secret_access_key: ""
  aws_session_token: ""
  max_conns: 10
  operation_timeout: 5s

eventbus:
  type: "kafka"
  brokers:
    - "localhost:9092"
  serializer: "json"
  operation_timeout: 30s

observability:
  log_level: "info"
  log_format: "json"
  service_name: "" # optional tracing service-name override
  tracing_enabled: true
  tracing_sample_rate: 0.1
  tracing_endpoint: "http://localhost:4318"
  request_logging:
    output: "logger"
    fields:
      - "request_id"
      - "request_method"
      - "request_uri"
      - "status"
      - "request_time"
      - "http_user_agent"

swagger:
  enabled: false
  spec_path: "/api/openapi/openapi.yaml"
```

### Recommended ENV presets

```bash
# Production with Memcached session store
APP_SESSION_ENABLED=true
APP_SESSION_STORE=memcached
APP_SESSION_TTL=12h
APP_SESSION_IDLE_TIMEOUT=30m
APP_SESSION_COOKIE_NAME=sid
APP_SESSION_COOKIE_SECURE=true
APP_SESSION_COOKIE_HTTP_ONLY=true
APP_SESSION_COOKIE_SAME_SITE=lax
APP_SESSION_MEMCACHED_ADDRESSES=memcached-1.internal:11211,memcached-2.internal:11211
APP_SESSION_MEMCACHED_TIMEOUT=500ms
APP_SESSION_MEMCACHED_PREFIX=session

APP_CSRF_ENABLED=true
APP_CSRF_HEADER_NAME=X-CSRF-Token
APP_CSRF_COOKIE_NAME=XSRF-TOKEN
APP_CSRF_COOKIE_SECURE=true
APP_CSRF_COOKIE_SAME_SITE=lax
APP_CSRF_COOKIE_TTL=12h
APP_CSRF_EXEMPT_METHODS=GET,HEAD,OPTIONS,TRACE
APP_CSRF_EXEMPT_PATHS=/health,/metrics
```

### OAuth token migration to server-side session

Store tokens in OAuth callback/login:

```go
token, err := auth.ExchangeAuthorizationCode(ctx, httpClient, oauthCfg, code)
if err != nil {
    return c.JSON(http.StatusUnauthorized, map[string]string{"error": "oauth exchange failed"})
}
if err := session.SetOAuthTokens(c, token.AccessToken, token.RefreshToken); err != nil {
    return c.JSON(http.StatusInternalServerError, map[string]string{"error": "session unavailable"})
}
```

Then inject bearer token from session before `authz.Authenticate`:

```go
r.Use(session.BearerFromSession())
r.Use(authz.Authenticate(jwtValidator))
```

## Usage in Code

### Loading Configuration

```go
package main

import (
    "log"
    "github.com/nimburion/nimburion/pkg/config"
)

func main() {
    // Load from defaults and environment variables only
    loader := config.NewViperLoader("", "APP")
    
    // Or load from a config file with environment variable overrides
    // loader := config.NewViperLoader("config.yaml", "APP")
    
    cfg, err := loader.Load()
    if err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    
    // Use configuration
    fmt.Printf("HTTP Port: %d\n", cfg.HTTP.Port)
    fmt.Printf("Log Level: %s\n", cfg.Observability.LogLevel)
}
```

### Configuration Validation

The configuration loader automatically validates all settings at startup and fails fast with clear error messages if any required values are missing or invalid.

Example validation errors:

```
config validation failed: auth.issuer is required when auth is enabled
auth.jwks_url is required when auth is enabled
auth.audience is required when auth is enabled
```

```
config validation failed: invalid database.type: invalid-type (must be one of: [postgres mysql mongodb dynamodb])
```

```
config validation failed: http.port and management.port must be different
```

## Default Values

The framework provides sensible defaults for all configuration options. See `pkg/config/defaults.go` for the complete list of default values.

Key defaults:
- HTTP Port: 8080
- Management Port: 9090
- Log Level: info
- Log Format: json
- Auth Enabled: false
- Tracing Enabled: false
- Swagger Enabled: false

## Best Practices

1. **Use Environment Variables in Production**: Environment variables are the most secure and flexible way to configure services in production environments.

2. **Use Configuration Files for Development**: Configuration files are convenient for local development and testing.

3. **Never Commit Secrets**: Never commit sensitive values (passwords, API keys) to configuration files. Always use environment variables or secret management systems.

4. **Validate Early**: The framework validates configuration at startup. This fail-fast approach prevents runtime errors due to misconfiguration.

5. **Document Custom Configuration**: If you add custom configuration options, document them clearly with examples.

6. **Use Sensible Defaults**: Leverage the framework's defaults for non-critical settings to reduce configuration complexity.

## Troubleshooting

### Configuration Not Loading from Environment Variables

Ensure environment variables use the correct prefix (`APP_`) and naming convention (underscores for nested keys).

Correct: `APP_HTTP_PORT=9000`
Incorrect: `HTTP_PORT=9000`

If you still use `APP_MANAGEMENT_*` or `APP_DATABASE_*`, they are accepted as legacy aliases, but `APP_MGMT_*` and `APP_DB_*` take precedence when both are set.

### Validation Errors

Read the error message carefully - it will tell you exactly which configuration value is missing or invalid.

### Configuration File Not Found

If you specify a configuration file path, ensure the file exists and is readable. The loader will return an error if the file cannot be read.

### Precedence Issues

Remember: Environment variables always override configuration file values. If a value isn't changing, check if an environment variable is set.
