# Review Tasks

> Current branch state: this document captures the pre-merge production-readiness review backlog for the current worktree before merging into `main`.
> Source of truth: findings here come from package-by-package review, targeted test execution, and cross-package contract inspection on the current branch.

This document turns the current review findings into an execution backlog aligned with the structure used in [refactoring-plan.md](./refactoring-plan.md).
The target architecture and package boundaries remain described in [refactoring-design.md](./refactoring-design.md).
Runtime and configuration expectations remain described in [refactoring-operational-specs.md](./refactoring-operational-specs.md) and [testing.md](./testing.md).

## Working Assumptions

- The goal of this document is merge readiness, not a new architectural redesign.
- Existing public contracts may still change before merge if the current behavior is observably unsafe or misleading.
- High-severity review findings are merge blockers.
- Medium-severity findings should be closed before merge unless there is an explicit waiver, owner, and follow-up issue.
- Test green status alone is not sufficient when the current tests miss the reviewed failure mode.

## Merge Gate Overview

The branch should not be merged into `main` until:

- all Milestone R0 tasks are complete
- regression coverage exists for each corrected high-severity behavior
- lifecycle and integration tests are runnable without environmental false negatives
- exported APIs do not silently violate their documented or implied contract

## Review Workstreams

| Workstream | Primary packages | Main risk |
| --- | --- | --- |
| Runtime correctness | `pkg/reliability/idempotency`, `pkg/http/session`, `pkg/http/server`, `pkg/jobs`, `pkg/config` | Production behavior differs from package/API intent. |
| Persistence correctness | `pkg/persistence/relational`, `pkg/persistence/keyvalue/dynamodb` | Runtime query failures and broken lifecycle semantics. |
| Transport and security | `pkg/http/httpsignature`, `internal/emailkit`, `pkg/email/smtp` | Unsafe defaults and injection surface. |
| Configuration and CLI wiring | `pkg/persistence/relational/migrate`, `pkg/http/server` | Flags and constructors appear valid but are not reliably wired. |
| Test reliability | `pkg/cache/redis`, `pkg/session/redis`, `pkg/persistence/relational/postgres` | Full test runs fail for environmental reasons instead of skipping cleanly. |

## Milestones

### Milestone R0: Merge Blockers

### Scope

- Close the high-severity issues that can cause incorrect production behavior, duplicate effects, broken shutdown, or runtime-only failures after configuration already validated successfully.

### Tasks

- `RT0.1` Make `pkg/reliability/idempotency` actually safe under contention.
  - Affected packages:
    - [`pkg/reliability/idempotency`](../pkg/reliability/idempotency/idempotency.go)
  - Current issue:
    - `Guard.ExecuteOnce` performs `IsProcessed` and `MarkProcessed` as separate operations around the handler, so concurrent callers can execute the side effect more than once.
  - Expected outcome:
    - the exported non-transactional path is either made atomic or narrowed so it no longer claims idempotent execution it cannot provide
    - concurrent duplicate execution is prevented or explicitly impossible through the public API
  - Required regression coverage:
    - a contention-oriented test that proves two concurrent calls for the same `(scope, key)` do not both execute the handler

- `RT0.2` Fix `QueryRowContext` timeout handling in relational adapters.
  - Affected packages:
    - [`pkg/persistence/relational/postgres`](../pkg/persistence/relational/postgres/adapter.go)
    - [`pkg/persistence/relational/mysql`](../pkg/persistence/relational/mysql/adapter.go)
    - [`pkg/persistence/relational`](../pkg/persistence/relational/generic_crud.go)
  - Current issue:
    - both adapters `defer cancel()` before returning `*sql.Row`, so the query context is cancelled before the caller can `Scan`
  - Expected outcome:
    - `QueryRowContext` no longer invalidates the returned row before use
    - callers such as `GenericCrudRepository.Count` behave correctly with query timeouts enabled
  - Required regression coverage:
    - adapter-level tests that exercise `QueryRowContext(...).Scan(...)` through the wrapper, not just directly through `*sql.DB`

- `RT0.3` Make `pkg/persistence/relational` truly dialect-aware or scope it to PostgreSQL only.
  - Affected packages:
    - [`pkg/persistence/relational`](../pkg/persistence/relational/generic_crud.go)
    - [`pkg/persistence/relational/postgres`](../pkg/persistence/relational/postgres/adapter.go)
    - [`pkg/persistence/relational/mysql`](../pkg/persistence/relational/mysql/adapter.go)
  - Current issue:
    - the generic CRUD repository hardcodes PostgreSQL placeholders (`$1`, `$2`, ...) while the public family exposes both PostgreSQL and MySQL adapters
  - Expected outcome:
    - placeholder generation is dialect-aware, or MySQL support is explicitly removed from this generic repository surface before merge
  - Required regression coverage:
    - a MySQL-oriented test that exercises create/find/list/count through the shared repository abstraction

- `RT0.4` Add missing cross-package validation for `jobs.backend=eventbus`.
  - Affected packages:
    - [`pkg/jobs/config`](../pkg/jobs/config/config.go)
    - [`pkg/config`](../pkg/config/loader.go)
    - [`pkg/jobs`](../pkg/jobs/provider.go)
    - [`pkg/eventbus/config`](../pkg/eventbus/config/config.go)
  - Current issue:
    - config validation allows `jobs.backend=eventbus` without requiring a supported `eventbus.type`, but runtime construction fails later with `unsupported eventbus.type`
  - Expected outcome:
    - invalid jobs/eventbus combinations fail during config validation only for commands and runtimes that actually require the jobs eventbus backend, without breaking the default global config surface
  - Required regression coverage:
    - config validation test for `jobs.backend=eventbus` with missing and invalid `eventbus.type`
    - coverage that generic/global validation still accepts the default eventbus-backed jobs config when no feature-specific requirement is active

- `RT0.5` Restore caller control over `http/session` auto-creation behavior.
  - Affected packages:
    - [`pkg/http/session`](../pkg/http/session/session.go)
    - [`pkg/http/server`](../pkg/http/server/public.go)
  - Current issue:
    - `normalizeConfig` rewrites `AutoCreate=false` back to the default `true`
  - Expected outcome:
    - explicit `AutoCreate=false` is preserved end-to-end
    - stateless routes can use the middleware without forced cookie emission
  - Required regression coverage:
    - test proving `AutoCreate=false` does not create a session and does not emit `Set-Cookie`

- `RT0.6` Preserve shutdown deadlines and cancellation budgets.
  - Affected packages:
    - [`pkg/http/server`](../pkg/http/server/server.go)
    - [`pkg/http/server`](../pkg/http/server/bootstrap.go)
    - [`pkg/core/app`](../pkg/core/app/app.go)
  - Current issue:
    - shutdown paths replace caller context with `context.Background()`, discarding orchestrator or parent timeout budget
  - Expected outcome:
    - shutdown uses caller-provided context unless there is a deliberate, documented derived timeout that still respects upstream cancellation
  - Required regression coverage:
    - lifecycle tests proving cancellation and deadlines propagate through server shutdown and shutdown hooks

### Done

- High-severity runtime issues no longer reproduce.
- Config validation catches invalid startup combinations before runtime assembly.
- Session and shutdown behavior match explicit caller intent.
- Regression tests cover the exact reviewed failure modes.

### Milestone R1: API Contract Hardening

### Scope

- Remove misleading exported behavior where constructors, normalizers, or lifecycle methods appear safe but are not defensive enough for general framework use.

### Tasks

- `RT1.1` Make HTTP signature config normalization honor explicit boolean intent.
  - Affected packages:
    - [`pkg/http/httpsignature`](../pkg/http/httpsignature/httpsignature.go)
    - [`pkg/http/httpsignature/config`](../pkg/http/httpsignature/config/config.go)
    - [`pkg/http/server`](../pkg/http/server/public.go)
  - Current issue:
    - partial config plus zero-value normalization can silently restore defaults such as `RequireNonce=true`
  - Expected outcome:
    - exported config handling does not reinterpret explicit boolean intent
    - `Enabled` and `RequireNonce` are predictable when callers provide partial config structs
  - Required regression coverage:
    - tests for `RequireNonce=false`
    - tests for `Enabled=false` with otherwise empty config

- `RT1.2` Make `BuildHTTPServers` fail defensively on nil options.
  - Affected packages:
    - [`pkg/http/server`](../pkg/http/server/bootstrap.go)
  - Current issue:
    - `BuildHTTPServers(nil)` can panic because `opts` is dereferenced before validation
  - Expected outcome:
    - exported constructor returns a normal error for missing required inputs instead of panicking

- `RT1.3` Make `NewManagementServer` safe when called directly.
  - Affected packages:
    - [`pkg/http/server`](../pkg/http/server/management.go)
    - [`pkg/health`](../pkg/health/registry.go)
    - [`pkg/observability/metrics`](../pkg/observability/metrics/registry.go)
  - Current issue:
    - nil registries are accepted, but `/ready` and `/metrics` handlers dereference them without guard
  - Expected outcome:
    - either nil registries are rejected at construction time or handlers degrade safely without panic

- `RT1.4` Make `migrate --migrations-path` a real feature contract, not an `APP_*` side effect.
  - Affected packages:
    - [`pkg/persistence/relational/migrate`](../pkg/persistence/relational/migrate/feature.go)
    - [`pkg/cli`](../pkg/cli/main.go)
    - [`pkg/config`](../pkg/config/loader.go)
  - Current issue:
    - the flag is implemented via hard-coded `APP_MIGRATIONS_PATH` and `APP_PLATFORM_MIGRATIONS_PATH`, ignoring non-`APP` env prefixes
  - Expected outcome:
    - migrations path override is propagated through typed inputs or prefix-aware config loading
    - the flag has one clear contract independent of ambient process env

- `RT1.5` Make DynamoDB adapter lifecycle semantics truthful.
  - Affected packages:
    - [`pkg/persistence/keyvalue/dynamodb`](../pkg/persistence/keyvalue/dynamodb/adapter.go)
  - Current issue:
    - `Close()` marks the adapter as closed, but operations still run through the client
  - Expected outcome:
    - either closed-state checks are enforced consistently or `Close()` semantics are narrowed and documented accordingly

- `RT1.6` Finish the Redis role split or make the divergence explicit.
  - Affected packages:
    - [`pkg/cache`](../pkg/cache/store_redis.go)
    - [`pkg/cache/redis`](../pkg/cache/redis/adapter.go)
    - [`pkg/session`](../pkg/session/store_redis.go)
    - [`pkg/session/redis`](../pkg/session/redis/adapter.go)
    - [`internal/rediskit`](../internal/rediskit/rediskit.go)
  - Current issue:
    - public store wrappers and role-specific adapters expose different semantics for the same backend role
  - Expected outcome:
    - one role-owned public contract is chosen for each package family
    - role-specific Redis adapters follow the parent package contract for not-found mapping and per-operation timeout behavior

### Done

- Exported constructors and middleware configs are safe under direct external use.
- Lifecycle methods do not imply guarantees they do not enforce.
- Role-owned Redis packages expose one coherent semantic contract per role.

### Milestone R2: Security And Message Safety

### Scope

- Close concrete security issues introduced or exposed by the refactor.

### Tasks

- `RT2.1` Sanitize raw MIME header construction.
  - Affected packages:
    - [`internal/emailkit`](../internal/emailkit/emailkit.go)
    - [`pkg/email/smtp`](../pkg/email/smtp/provider.go)
  - Current issue:
    - `BuildMIMEMessage` writes `From`, `Reply-To`, `Subject`, and arbitrary custom headers directly into the MIME stream without CR/LF sanitization
  - Expected outcome:
    - raw header values cannot inject additional headers or malformed MIME structure
    - caller-provided data is canonicalized or rejected before MIME assembly
  - Required regression coverage:
    - tests for CR/LF injection attempts in standard and custom headers

### Done

- User-controlled email fields cannot alter header structure.
- SMTP raw message generation has explicit validation around header-safe values.

### Milestone R3: Test Reliability And Merge Confidence

### Scope

- Make the pre-merge verification path reliable on developer machines and in CI, with clear separation between unit, property, and integration requirements.

### Tasks

- `RT3.1` Make integration tests skip cleanly when Docker is unavailable.
  - Affected packages:
    - [`pkg/cache/redis`](../pkg/cache/redis/adapter_integration_test.go)
    - [`pkg/session/redis`](../pkg/session/redis/adapter_integration_test.go)
    - [`pkg/persistence/relational/postgres`](../pkg/persistence/relational/postgres/adapter_integration_test.go)
  - Current issue:
    - full `go test ./...` can fail or panic on hosts without Docker instead of reporting a clean skip
  - Expected outcome:
    - local full test runs fail only for real code issues, not missing container runtime
    - integration coverage still runs in CI environments that provide Docker

- `RT3.2` Add regression tests for all corrected review findings.
  - Required areas:
    - idempotency concurrency
    - session `AutoCreate=false`
    - HTTP signature boolean normalization
    - shutdown context propagation
    - jobs/eventbus config validation
    - relational `QueryRowContext` wrapper behavior
    - generic CRUD dialect compatibility
    - MIME header injection rejection

- `RT3.3` Define the pre-merge verification command set.
  - Expected baseline:
    - `go vet ./...`
    - targeted package tests for corrected areas
    - integration tests behind explicit environment capability checks
  - Expected outcome:
    - there is one documented, repeatable pre-merge verification path for this repo
  - Current pre-merge command set:
    - `env GOCACHE=.cache/go-build go vet ./...`
    - `env GOCACHE=.cache/go-build go test ./pkg/config ./pkg/cli`
    - `env GOCACHE=.cache/go-build go test ./pkg/http/session ./pkg/http/server ./pkg/http/httpsignature`
    - `env GOCACHE=.cache/go-build go test ./pkg/jobs ./pkg/scheduler ./pkg/http/openapi`
    - `env GOCACHE=.cache/go-build go test ./pkg/reliability/idempotency ./internal/emailkit`
    - `env GOCACHE=.cache/go-build go test ./pkg/persistence/relational ./pkg/persistence/relational/mysql ./pkg/persistence/relational/migrate ./pkg/persistence/keyvalue/dynamodb`
    - `env GOCACHE=.cache/go-build go test ./pkg/cache ./pkg/session`
    - `env GOCACHE=.cache/go-build go test ./pkg/cache/redis -run 'TestAdapter_MapGetError_NotFoundMapsToCacheMiss|TestAdapter_WithOperationTimeout|TestAdapter_Integration'`
    - `env GOCACHE=.cache/go-build go test ./pkg/session/redis -run 'TestAdapter_MapGetError_NotFoundMapsToSessionErrNotFound|TestAdapter_WithOperationTimeout|TestAdapter_Integration'`
    - `env GOCACHE=.cache/go-build go test ./pkg/persistence/relational/postgres -run TestAdapter_Integration`
    - `env GOCACHE=.cache/go-build go test ./...`

### Done

- `go test ./...` is informative on a normal development machine.
- Integration-only requirements do not surface as opaque runtime panics.
- Every high-severity fix ships with a regression test.

## Exit Criteria

This review backlog is complete when:

- Milestone R0 is complete with tests
- Milestone R1 and Milestone R2 are complete, or any remaining item has an explicit waiver recorded before merge
- Milestone R3 establishes a reliable verification path for local and CI environments
- the branch can be described as production-ready without relying on undocumented operator knowledge or fragile startup ordering
