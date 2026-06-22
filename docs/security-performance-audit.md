# Nimburion Framework — Security, Performance & Design Audit

**Date:** 2026-06-22
**Scope:** Full framework (`pkg/`, `internal/`) — ~42k LOC of non-test Go across 343 files.
**Method:** Manual source review across six domain areas plus repo-wide pattern scans. Each
finding below was verified against the source and cites `file:line`.

> **Tooling note:** `govulncheck` could not run — this environment's network policy blocks
> `vuln.go.dev` (HTTP 403). Dependency versions were checked manually and are current
> (jwt v5.3.1, grpc v1.79.3, gin v1.12.0, kin-openapi v0.133.0); the latest commit already
> patches Go toolchain CVEs. An automated CVE cross-check should be re-run in an environment
> with access to the Go vulnerability database.

---

## Executive summary

| Severity | Count |
|----------|-------|
| Critical | 8 |
| High     | 17 |
| Medium   | 21 |
| Low      | 19 |

### Cross-cutting themes
1. **Insecure-by-default / fail-open posture.** The framework repeatedly *allows* when it should
   *deny*: empty scope requirement authorizes everything (auth M-2), empty SQL column allowlist
   permits all columns (persistence C1), WebSocket origin defaults to allow-all (http M2), CORS
   reflects credentialed wildcard origins (http C1), JWT accepts tokens with no `exp` (auth C1).
2. **Missing resource limits → DoS.** No gRPC message-size / concurrent-stream / keepalive limits
   (grpc C1/C2), unbounded in-memory cache (persistence M2), unbounded metric label cardinality
   (observability M1, eventbus H2), no enforced HTTP read-header timeout (http H4).
3. **Process-fatal panics.** Eventbus consumer goroutines, scheduler task loops, and app runners
   have no panic recovery — one bad message/handler crashes the whole process (eventbus C1,
   grpc C3, core M7, scheduler L2).
4. **Secret leakage.** Full config (with DB/SMTP/provider credentials) dumped at debug log level
   (config C2); `config show` leaks env/file-sourced secrets (config H1); raw SQL and cache keys
   in cleartext-exported traces (observability M5/M6).
5. **Distributed-correctness gaps.** Lock lease has a data race + no fencing token (coordination
   C4), SQS visibility/handler races (eventbus C2), saga orphans resources on compensation
   failure (reliability C3), query-timeout context cancelled while rows still streaming
   (persistence H1).

---

## CRITICAL

### CR-1 — JWT expiration not required; tokens without `exp` are valid forever
**Security** · `pkg/auth/jwt.go:108-138`
`jwt.Parse` is called with no parser options. golang-jwt v5 only validates `exp`/`nbf` when
present; a token minted or stripped without `exp` passes signature/issuer/audience checks and is
accepted indefinitely, turning any leaked token into a permanent credential.
**Fix:** Build the parser with `jwt.NewParser(jwt.WithExpirationRequired(),
jwt.WithValidMethods(...), jwt.WithIssuer(...), jwt.WithAudience(...))` and use `ParseWithClaims`.

### CR-2 — Full configuration (all secrets) written to logs at debug level
**Security** · `pkg/cli/main.go:827-837` (`logConfigIfDebug`)
At `log_level=debug` the CLI logs `fmt.Sprintf("%+v", cfg)`, dumping plaintext `Database.URL`
(embedded DB creds), `Email.SMTP.Password`, SES secret keys, every provider token, cache URL, etc.
No secret-bearing config type implements redacting `String()`/`LogValue()`.
**Fix:** Never dump the whole struct; log a redacted view driven by a sensitivity model, or
implement `slog.LogValuer` redaction on secret-bearing types.

### CR-3 — Path-traversal guard is ineffective (prefix-boundary bug)
**Security** · `internal/safepath/safepath.go:37`
The central traversal defense uses `strings.HasPrefix(absPath, absBase)` with no separator
boundary, so base `/var/app` accepts `/var/app-secrets/db.yaml`. No `filepath.EvalSymlinks`, and
when `baseDir == ""` (the i18n call pattern) only a lexical `..` check applies.
**Fix:** Use `absPath == absBase || strings.HasPrefix(absPath, absBase+string(os.PathSeparator))`;
resolve symlinks; reject empty `baseDir` when containment is required.

### CR-4 — SQL injection via unvalidated identifiers; column allowlist fails open
**Security** · `pkg/persistence/relational/generic_crud.go:116-373`, allowlist at `:399-407`
Table name, ID column, filter fields and sort fields are interpolated via `fmt.Sprintf` /
`strings.Join`. `validateColumnAllowed` returns `nil` (allow all) when the allowlist is empty —
the default constructor path. Values are parameterized; identifiers are not.
**Fix:** Make the allowlist mandatory (fail closed); validate table/ID identifiers against a
strict regex at construction; treat empty allowlist as "deny all dynamic columns."

### CR-5 — Memcached command injection via cache keys (text protocol)
**Security** · `internal/memcachedkit/memcachedkit.go:74,121,155,186`
Keys are formatted directly into the line-delimited memcached text protocol with no validation.
A key containing `\r\n`/whitespace injects arbitrary memcached commands (cache poisoning,
exfiltration). The server-declared `size` also drives an unbounded `make([]byte, size+2)`.
**Fix:** Reject keys with whitespace/control chars, enforce the 250-byte limit (or hash keys);
bound the declared size before allocating.

### CR-6 — No gRPC message-size / concurrent-stream / connection limits (DoS)
**Security (DoS)** · `pkg/grpc/server/server.go:79-90`
`grpc.NewServer` is built with only interceptors and optional TLS — no `MaxRecvMsgSize`,
`MaxConcurrentStreams`, `ConnectionTimeout`, and no passthrough for callers to add them. A single
client can open unbounded streams and hold half-open connections.
**Fix:** Apply framework defaults (`MaxConcurrentStreams`, explicit `MaxRecvMsgSize`,
`ConnectionTimeout`) and expose an `Options.ServerOptions []grpc.ServerOption` passthrough.

### CR-7 — Unrecovered panics in eventbus consumer goroutines crash the process
**Reliability** · `pkg/eventbus/kafka/adapter.go:380-433`, `rabbitmq/adapter.go:242-271`,
`sqs/adapter.go:196-234`, `retry_dlq.go:243-249`
All three broker adapters call `handler(ctx, msg)` in a background goroutine with no
`defer recover()`. A single handler panic terminates the entire process across all topics.
(The jobs worker recovers correctly — the gap is the eventbus adapters.)
**Fix:** Wrap each handler call in a recover that converts the panic to an error → DLQ/quarantine.

### CR-8 — CORS reflects credentialed wildcard/function-approved origins (credential theft)
**Security** · `pkg/http/cors/cors.go:270-281`, `:169-199`
With `AllowCredentials=true`, any origin approved by `AllowWildcard` patterns or
`AllowOriginFunc` is reflected verbatim alongside `Access-Control-Allow-Credentials: true`.
`wildcardMatch` is an unanchored prefix/suffix check (`https://good.com*` matches
`https://good.com.evil.com`), letting a malicious origin read authenticated responses.
**Fix:** Forbid combining credentials with wildcard/pattern origins (require exact-origin
allowlist); anchor `wildcardMatch` to DNS label boundaries.

---

## HIGH

### HI-1 — JWT signing-algorithm allowlist not enforced at parser; JWK `alg`/`use` ignored
**Security** · `pkg/auth/jwt.go:110-114`, `pkg/auth/jwks.go:255-285`
The keyfunc's `*jwt.SigningMethodRSA` type assertion does block RS→HS confusion, but there is no
`jwt.WithValidMethods` defense-in-depth and the JWK's declared `alg`/`use` are never validated
(an `enc` key would still verify signatures).
**Fix:** Add `WithValidMethods`; in `parseJWK` require `use == "sig"` and an RSA signing `alg`.

### HI-2 — RSA keys parsed from JWKS have no minimum modulus / exponent validation
**Security** · `pkg/auth/jwks.go:257-285`
`parseJWK` builds an `rsa.PublicKey` with no key-size check; a 512-bit modulus or `e=1` would be
accepted, and `e.Int64()` silently truncates oversized exponents.
**Fix:** Reject `n.BitLen() < 2048`; validate the exponent is a sane odd value before `Int64()`.

### HI-3 — JWKS cache failure mode is fail-open + thundering herd + unknown-`kid` amplification
**Security / Performance** · `pkg/auth/jwks.go:88-111`, `:289-307`
After TTL the cache returns `nil` for *all* keys, so every request triggers a synchronous refresh
with no singleflight (thundering herd). An unknown `kid` always forces a network fetch with no
negative caching, so attacker-controlled `kid` values drive one upstream JWKS fetch per request.
**Fix:** Use `singleflight` around refresh; add short negative caching / rate-limiting for unknown
`kid` lookups.

### HI-4 — Router leaks internal error strings to clients (info disclosure)
**Security** · `pkg/http/router/nethttp/adapter.go:112-117`
When a handler/middleware returns an error and nothing was written, the net/http adapter sends
raw `err.Error()` with a 500, bypassing the sanitized `response.MapError`. The gin adapter
correctly returns a bare 500.
**Fix:** Write a generic 500 body in the net/http adapter; route returned errors through
`response.MapError`.

### HI-5 — Panic recovery middleware positioned 9th — earlier middleware panics bypass it
**Security / Design** · `pkg/http/server/public.go:317-343`
Recovery sits after `http_signature`, `security_headers`, `session`, `csrf`, `cors`, `i18n`,
`logging`. A panic in any of those (session/CSRF run store code) unwinds past recovery and leaks
via HI-4.
**Fix:** Move `recovery` (and `request_id`) to the top of the stack.

### HI-6 — Timeout middleware doesn't abort handlers (cooperative only) — slow-handler DoS
**Security / Performance** · `pkg/http/middleware/timeout/timeout.go:51-71`
The middleware sets a context deadline then calls `next(c)` synchronously; a handler ignoring
`ctx.Done()` runs to completion and holds the worker. 504 is only emitted after `next` returns.
**Fix:** Enforce hard timeouts (`http.TimeoutHandler` or goroutine race) and always set server
read/write deadlines.

### HI-7 — `http.Server` sets no `ReadHeaderTimeout` (Slowloris)
**Security** · `pkg/http/server/server.go:58-65`; timeouts unvalidated in `server/config/config.go:94-135`
No `ReadHeaderTimeout`, and `ReadTimeout`/`WriteTimeout`/`IdleTimeout` have no positivity
validation (0 disables them), enabling slow-header Slowloris.
**Fix:** Always set `ReadHeaderTimeout`; validate timeouts are > 0 or apply non-zero fallbacks.

### HI-8 — No datastore TLS enforcement; insecure-by-default connections
**Security** · `pkg/persistence/config/config.go:24-58`; postgres/mysql adapters `:39`;
`internal/rediskit/rediskit.go:35-45`
No config surface or default to require TLS for Postgres/MySQL/Redis/Mongo; memcachedkit has no
TLS at all. All fixtures use `sslmode=disable`, the pattern users copy. Credentials/data travel
in plaintext by default.
**Fix:** Add a TLS/sslmode option, default to requiring TLS (or warn loudly), reject
`sslmode=disable` without an explicit insecure flag.

### HI-9 — Query-timeout context cancelled while `*sql.Rows` still streaming
**Performance / Correctness** · `pkg/persistence/relational/postgres/adapter.go:193-220`;
mysql `:159-185`
`QueryContext` derives a timeout context with `defer cancel()`; the cancel fires when the function
returns, but the caller iterates the returned rows afterward (`generic_crud.go:200-217`), so
iteration can fail mid-scan with "context canceled." `QueryRowContext` skips the timeout entirely.
**Fix:** Tie cancellation to rows lifetime (`context.AfterFunc` on `Close`); apply the same to
`QueryRowContext`.

### HI-10 — Reflection mapper deserializes rows via unchecked `reflect.Convert` (panic/DoS)
**Security / Design** · `pkg/persistence/relational/generic_crud.go:505-508`
`ReflectionMapper.FromRow` blindly `Convert`s driver values to field types; a non-convertible type
(`[]byte`→`int`) panics, and combined with `WithTransaction`'s re-panicking `recover`, one bad row
crashes the process.
**Fix:** Guard with `ConvertibleTo()`, handle `[]byte`/`time.Time` explicitly, never panic.

### HI-11 — Internal error causes leaked to gRPC clients
**Security** · `pkg/grpc/status/status.go:68-87`, `pkg/core/errors/errors.go:55-67`
`SafeMessage` returns operator-supplied `FallbackMessage` for any AppError including
internal-class ones (e.g. "failed to connect to postgres at 10.0.0.5:5432"), with no enforcement
that 500-class errors collapse to a generic message.
**Fix:** Force a constant "internal error" message for `codes.Internal` / HTTP 500; only surface
messages for client-facing codes.

### HI-12 — gRPC reflection registers unconditionally (info disclosure)
**Security** · `pkg/grpc/reflection/reflection.go:11-18`
`Registration()` calls `grpcreflection.Register` with no environment/debug guard; if wired in
production, the full schema is exposed to unauthenticated clients.
**Fix:** Gate behind explicit opt-in or non-production/debug check.

### HI-13 — Async logger blocks request goroutines or silently drops logs
**Design / Performance** · `pkg/observability/logger/async.go:145-153`
With `DropWhenFull=false`, a slow sink blocks every logging goroutine (full-service stall); with
the default `DropWhenFull=true`, logs are dropped silently with no counter — security-relevant
lines vanish under load.
**Fix:** Increment a dropped-logs metric on the drop branch; bound/timeout the blocking branch.

### HI-14 — `resilience.WithTimeout` leaks a goroutine per call when `fn` ignores ctx
**Performance / Reliability** · `pkg/resilience/timeout.go:14-37`
On timeout the function returns but the goroutine running `fn` keeps running until `fn` returns;
this wraps *every* job attempt (`worker.go:343`), so one misbehaving handler leaks a goroutine per
attempt — a slow goroutine/memory DoS.
**Fix:** Require ctx-aware handlers; avoid spawning a goroutine, or surface leaked-goroutine
metrics.

### HI-15 — Metrics cardinality explosion: raw error string used as Prometheus label
**Security / Performance** · `pkg/eventbus/retry_dlq.go:256` → `:159-167`
`retryTotal.WithLabelValues(lastErr.Error())` puts full error text (IDs, offsets, payload
fragments) into a label, creating unbounded time series — OOM of the process and scrape target.
The stable `Classify(lastErr)` category is computed nearby and should be used instead.
**Fix:** Label by the bounded classification, not the raw error.

### HI-16 — SSRF + cleartext-credential exposure via configurable email `base_url`
**Security** · `internal/emailkit/emailkit.go:38-51` (`ValidateEndpointURL`); all HTTP providers
(e.g. `sendgrid/provider.go:64-72`, `ses/provider.go:82-99`)
Provider URLs are validated only for scheme + non-empty host — no domain allowlist, no
private/loopback/metadata block, and `http://` is accepted. A misconfigured/injected `base_url`
(e.g. `http://169.254.169.254/...`) causes authenticated POSTs to internal targets, leaking the
provider credential. `#nosec G704` suppresses the warning.
**Fix:** Require `https`, validate against a per-provider host allowlist (or block private ranges).

### HI-17 — Poison-message hot loops with no backoff/DLQ (Kafka & RabbitMQ)
**Reliability / Performance** · `pkg/eventbus/kafka/adapter.go:419`, `rabbitmq/adapter.go:261`
A consistently failing message is redelivered immediately and indefinitely (Kafka `continue` with
no commit/backoff; RabbitMQ `Nack(...,requeue=true)` forever). No attempt counting, no DLX. Also
RabbitMQ never recovers from a closed deliveries channel.
**Fix:** Add redelivery counting + backoff, route exhausted messages to a DLX/DLQ, handle
`NotifyClose`/reconnect.

---

## MEDIUM

> Condensed — see `file:line` for each.

- **ME-1** Empty scope requirement authorizes any authenticated caller. *Design* ·
  `pkg/policy/policy.go:37-41`, `pkg/grpc/auth/auth.go:104`. Treat empty as deny.
- **ME-2** No rate limiting on token validation / unknown-`kid` lookups. *Security* ·
  `pkg/grpc/auth/auth.go:70-91`, `pkg/auth/jwks.go:88-111`.
- **ME-3** `iat` never validated; no configured clock-skew leeway. *Security* ·
  `pkg/auth/jwt.go:182-185`.
- **ME-4** `config show` redaction is provenance-based, leaks env/main-file secrets. *Security* ·
  `pkg/cli/main.go:330-340`, `pkg/audit/mask.go:35-49`. Redact by sensitivity classification.
- **ME-5** SMTP `InsecureSkipVerify` option + no STARTTLS on port 587 (creds sent plaintext).
  *Security* · `pkg/email/config/config.go:63`, `pkg/email/smtp/provider.go:67-77`.
- **ME-6** SMTP `OperationTimeout` computed then discarded (`_ = cctx`); hung server blocks
  forever. *Performance* · `pkg/email/smtp/provider.go:64-70`.
- **ME-7** i18n catalog reads call `safepath.ValidateFilePath(path, "")` (no containment).
  *Security* · `pkg/i18n/catalog.go:209,226`.
- **ME-8** In-memory cache store unbounded (no max size, lazy eviction only) — memory DoS.
  *Performance/Security* · `pkg/cache/store_inmemory.go:13-55`.
- **ME-9** Cache stampede: no single-flight / `GetOrLoad` on miss across all stores.
  *Performance* · `pkg/cache/store_redis.go:73-91` et al.
- **ME-10** No connection-pool bounds validation; zero values disable SQL limits (conn
  exhaustion). *Performance* · postgres `adapter.go:45-48`, mysql `:44-47`.
- **ME-11** Static long-lived AWS creds in plaintext config structs, encouraged over IAM
  roles/IRSA. *Security* · `pkg/persistence/config/config.go:37-39`.
- **ME-12** `SameSite=None` cookies not forced to `Secure` (session + CSRF). *Security* ·
  `pkg/http/session/session.go:317-345`, `pkg/http/csrf/csrf.go:140-153`.
- **ME-13** WebSocket origin policy defaults to allow-all (CSWSH). *Security* ·
  `pkg/http/ws/ws.go:314-328`.
- **ME-14** `request_size` middleware is innermost; OpenAPI validator `io.ReadAll`s body before
  the cap applies. *Security(DoS)* · `pkg/http/server/public.go:332-333`,
  `pkg/http/contract/openapi/openapi_request_validation.go:114-131`.
- **ME-15** Management `/health` bypasses IP allowlist + auth. *Security/Design* ·
  `pkg/http/server/management.go:151`.
- **ME-16** `securityheaders` fully disabled when `IsDevelopment` true, no warning. *Security* ·
  `pkg/http/securityheaders/securityheaders.go:78-80`.
- **ME-17** Tracing exporter defaults to insecure plaintext OTLP, no TLS config knob. *Security* ·
  `pkg/observability/tracing/tracer.go:74-80`.
- **ME-18** Span attributes capture raw SQL (`db.statement`) and cache keys (incl. span name);
  combined with ME-17 leaks PII in cleartext. *Security* · `pkg/observability/tracing/spans.go:99-103,232-236`.
- **ME-19** `runRunners` launches runners in bare goroutines with no recover — runner panic
  crashes process. *Availability* · `pkg/core/app/app.go:492-504`.
- **ME-20** gRPC health `Watch` sends one snapshot then blocks forever (stale SERVING).
  *Design* · `pkg/grpc/health/health.go:49-59`.
- **ME-21** Distributed/delivery correctness cluster — SQS visibility vs handler duration
  (duplicate processing/unsafe delete, `pkg/eventbus/sqs/adapter.go:196-234`); saga aborts
  remaining compensations on first failure, orphaning resources (`pkg/reliability/saga/saga.go:131-141`);
  lock-lease data race + no fencing token (`pkg/coordination/redis/lock.go:198`,
  `postgres/lock.go:226`); RuntimeBackend lease-expiry races handler completion
  (`pkg/jobs/runtime_backend.go:150-152,421-433`); outbox/dedup TOCTOU + no dead-letter
  (`pkg/reliability/dedup/dedup.go:55-66`, `outbox/outbox.go:307-345`); DLQ payloads unbounded +
  ignore data classification (`pkg/eventbus/retry_dlq.go:347-357`); divergent backoff with no
  jitter (`pkg/reliability/retry/retry.go:199-214`, `outbox/outbox.go:355-370`); postgres lock
  table grows unbounded, release returns spurious `ErrConflict`
  (`pkg/coordination/postgres/lock.go:162-188,251`); aggregate-version check-then-set race
  (`pkg/eventbus/aggregate_versioning.go:101-123`). All *Reliability/Design*.

---

## LOW

- **LO-1** Issuer/audience compared with `==` (non-constant-time; low-entropy, theoretical).
  `pkg/auth/jwt.go:147,414-420`.
- **LO-2** Session stores trust caller IDs (no entropy/format validation at store boundary).
  `pkg/session/store_*.go`.
- **LO-3** Memcached not-found detection by error-string match (fragile, fail-open risk).
  `pkg/session/store_memcached.go:158-163`.
- **LO-4** JWKS refresh caps neither key count nor response size. `pkg/auth/jwks.go:146-161`.
- **LO-5** Client-controlled `X-Request-ID` reflected + logged unbounded (log injection).
  `pkg/http/middleware/requestid/requestid.go:25-37`.
- **LO-6** Swagger UI loads assets from unpkg CDN with no SRI. `pkg/http/openapi/swagger.go:53-98`.
- **LO-7** SSE `event.ID`/`event.Type` written without stripping newlines (stream injection).
  `pkg/http/sse/handler.go:162-193`.
- **LO-8** Static-file `Exists` check and `FileServer` serve use different path computations
  (fragile dual-path). `pkg/http/static/static.go:36-83`.
- **LO-9** Per-request allocations on hot paths: linear route match with double `strings.Split`;
  CORS re-`Join`s headers each response. `pkg/http/router/nethttp/adapter.go:165-186`,
  `pkg/http/cors/cors.go:64-115`.
- **LO-10** Email recipient/from addresses not RFC-5322 validated. `pkg/email/provider.go:42-54`.
- **LO-11** CLI profile/secret-file flags mutate global process env via `os.Setenv` (leaks to
  child processes). `pkg/cli/main.go:777-796`.
- **LO-12** i18n interpolation via `strings.ReplaceAll`, output not HTML-safe / undocumented.
  `pkg/i18n/catalog.go:188-206`.
- **LO-13** Memcached `Get` trusts server-declared size for allocation. `internal/memcachedkit/memcachedkit.go:90-94`.
- **LO-14** memcached node pick loop is O(n) + uses FNV-modulo (mass remap on node change).
  `internal/memcachedkit/memcachedkit.go:228-241`.
- **LO-15** `FindAll` returns unbounded result set when no pagination supplied; uses `SELECT *`.
  `pkg/persistence/relational/generic_crud.go:161-220`.
- **LO-16** Cleaner loops (dedup/idempotency) exit permanently on a single transient error.
  `pkg/reliability/dedup/dedup.go:111-113`, `idempotency/idempotency.go:182-184`.
- **LO-17** Scheduler per-task goroutines lack recover. `pkg/scheduler/runtime.go:153,197`.
- **LO-18** Weak token fallback to `UnixNano()` on `crypto/rand` failure (lock ownership
  collision). `pkg/jobs/runtime_backend.go:453-458`, coordination locks.
- **LO-19** `time.After` in retry/reserve loops not stopped on cancel (timer accumulation);
  metadata not redacted in gRPC context; `RedactSettings` is a denylist; OpenSearch SDK adapter
  doesn't escape IDs. Misc. — `pkg/eventbus/retry_dlq.go:268-272`, `pkg/jobs/redis_backend.go:256`,
  `pkg/grpc/interceptor/interceptor.go:69-86`, `pkg/audit/mask.go:62-69`.

---

## Verified correct (to bound the report)
- SSRF protection on JWKS/OAuth2 URLs is solid (DNS-rebinding-resistant re-resolution at dial).
- JWT alg-confusion to HMAC is blocked by the RSA type assertion.
- CSRF and session IDs use `crypto/rand` + `subtle.ConstantTimeCompare`; TLS sets `MinVersion 1.2`,
  mTLS uses `RequireAndVerifyClientCert`.
- SQL **values** are consistently parameterized (injection risk is identifiers only).
- Transaction rollback-on-error/panic is correct in postgres/mysql; rows are `defer`-closed.
- Jobs worker recovers from handler panics; circuit-breaker state machine is mutex-correct.
- Audit hash-chain verification is sound; trace context uses W3C TraceContext + Baggage.
- `go vet ./...` is clean.

---

## Recommended remediation order
1. **CR-2, CR-1, CR-3, CR-4, CR-5** — secret-in-logs, JWT expiry, path traversal, SQL identifier
   injection, memcached injection. Highest impact, mostly localized.
2. **CR-6, CR-7, CR-8** — gRPC resource limits, eventbus panic recovery, CORS credential reflection.
3. **HI-1…HI-8** — auth hardening, HTTP error leakage/recovery ordering/timeouts, datastore TLS.
4. **HI-9…HI-17** + ME cluster — query-timeout lifetime, reflection mapper, logging backpressure,
   goroutine leak, metric cardinality, SSRF, poison loops.
5. MEDIUM/LOW as hardening backlog.
</content>
</invoke>
