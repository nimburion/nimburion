# Refactoring Plan

> Current branch state: several source package roots mentioned below as move origins are already removed on this branch.
> Historical mapping: package-move tables and milestone references preserve refactor provenance and execution order.

This document turns the target architecture in [refactoring-requirements.md](./refactoring-requirements.md) into an execution plan.
The target package boundaries and runtime model are described in [refactoring-design.md](./refactoring-design.md).
Actors and workflow-level goals are described in [refactoring-user-stories.md](./refactoring-user-stories.md).
Story-to-milestone traceability is maintained in [refactoring-traceability.md](./refactoring-traceability.md).
Cross-repo alignment for `nimbctl` is tracked in [nimbctl-alignment-plan.md](./nimbctl-alignment-plan.md).
The current and target framework error model is described in [error-model.md](./error-model.md).

## Working Assumptions

- Backward compatibility is not a primary constraint for this refactor.
- The preferred strategy is a clean move toward the target package model, not a long-lived compatibility layer.
- Current packages remain accurate for the running codebase until they are replaced, but they should be deleted once their target replacements are stable.
- New framework code must follow this plan, even while old packages still exist in the tree.

## Current Branch State Versus Historical Mapping

- The package-move tables in this document are historical execution mappings, not a statement that every source package still exists on the current branch.
- On the current branch, `pkg/server`, `pkg/controller`, `pkg/repository`, `pkg/migrate`, and `pkg/store` have already been removed.
- When this document references those roots in move tables or milestone text, read them as refactor provenance, not as active package locations.

## End-State Overview

The target package model is:

- `pkg/core`
  - app lifecycle
  - feature model
  - core error contracts
  - minimal shared runtime contracts
- `pkg/cli`
  - shared CLI framework for user applications
  - `run`, `config`, `version`, `healthcheck`
  - optional feature commands such as `migrate`
- `pkg/config`
  - configuration loading, merge, validation orchestration, flags binding
- `pkg/config/schema`
  - schema generation and schema composition
- `pkg/http`
  - transport-specific HTTP contracts and features
- `pkg/grpc`
  - transport-specific gRPC contracts and features
- `pkg/persistence`
  - explicit persistence families instead of a generic `store`
- top-level operational or cross-cutting packages that remain public where the naming is already good:
  - `pkg/auth`
  - `pkg/audit`
  - `pkg/email`
  - `pkg/eventbus`
  - `pkg/featureflag`
  - `pkg/jobs`
  - `pkg/scheduler`
  - `pkg/health`
  - `pkg/i18n`
  - `pkg/observability`
  - `pkg/policy`
  - `pkg/reliability`
  - `pkg/resilience`
  - `pkg/tenant`
  - `pkg/version`
  - `pkg/cache`
  - `pkg/session`
  - `pkg/coordination`
  - `pkg/pubsub`

## Package Migration Map

| Current package | Target package | Notes |
| --- | --- | --- |
| `pkg/server` | `pkg/http/server` + `pkg/core` integration points | Server bootstrap becomes HTTP-specific instead of a universal runtime entrypoint. |
| `pkg/server/router` | `pkg/http/router` | Keep the common router contract; move `nethttp`, `gin`, and `gorilla` adapters under it. |
| `pkg/server/openapi` | `pkg/http/openapi` | Treat OpenAPI as an HTTP capability, not a `pkg/server` extension point. |
| `pkg/realtime/sse` | `pkg/http/sse` | SSE remains valid, but belongs to the HTTP family. |
| `pkg/realtime/ws` | `pkg/http/ws` | WebSocket support belongs to the HTTP family. |
| `pkg/controller` | `pkg/core/errors`, `pkg/http/response`, `pkg/http/input` | Split error contracts, HTTP response helpers, and input validation helpers. |
| `pkg/configschema` | `pkg/config/schema` | Schema generation becomes a capability under config. |
| `pkg/store/postgres` | `pkg/persistence/relational/postgres` | Add dialect-aware abstractions above the adapter. |
| `pkg/store/mysql` | `pkg/persistence/relational/mysql` | Same as Postgres; portability lives in the relational family, not in `pkg/store`. |
| `pkg/repository` | `pkg/persistence/relational` | SQL repository contracts move into the relational family. |
| `pkg/repository/document` | `pkg/persistence/document` | Document repository contracts move into the document family. |
| `pkg/store/mongodb` | `pkg/persistence/document/mongodb` | Document-family adapter. |
| `pkg/store/dynamodb` | `pkg/persistence/keyvalue/dynamodb` | Primary family is key-value; document-style bridges stay optional. |
| `pkg/store/opensearch` | `pkg/persistence/search/opensearch` | Search engine adapter, not generic store. |
| `pkg/store/s3` | `pkg/persistence/object/s3` | Object storage family. |
| `pkg/store/redis` | `pkg/cache/redis`, `pkg/session/redis`, `pkg/coordination/redis`, `pkg/pubsub/redis`, `pkg/jobs/redis` | Split Redis by role instead of keeping a generic adapter. |
| `pkg/store/memcached` | `pkg/cache/memcached`, `pkg/session/memcached` | Split Memcached by role. |
| `pkg/middleware/cache` | `pkg/cache` + `pkg/http/cache` | Separate cache model from HTTP response-cache integration. |
| `pkg/middleware/session` | `pkg/session` + `pkg/http/session` | Separate session role from HTTP integration. |
| `pkg/middleware/authz` | `pkg/http/authentication` + `pkg/http/authorization` | Split authentication and authorization concerns. |
| `pkg/middleware/cors` | `pkg/http/cors` | HTTP-only policy. |
| `pkg/middleware/csrf` | `pkg/http/csrf` | HTTP-only security capability. |
| `pkg/middleware/httpsignature` | `pkg/http/httpsignature` | HTTP-only security capability. |
| `pkg/middleware/securityheaders` | `pkg/http/securityheaders` | HTTP-only security capability. |
| `pkg/middleware/openapivalidation` | `pkg/http/contract/openapi` | One contract-validation provider under HTTP. |
| `pkg/middleware/i18n` | `pkg/http/i18n` | HTTP locale negotiation stays transport-specific. |
| `pkg/middleware/logging` | `pkg/http/middleware/logging` | HTTP-specific middleware. |
| `pkg/middleware/metrics` | `pkg/http/middleware/metrics` | HTTP-specific middleware. |
| `pkg/middleware/tracing` | `pkg/http/middleware/tracing` | HTTP-specific middleware. |
| `pkg/middleware/recovery` | `pkg/http/middleware/recovery` | HTTP-specific middleware. |
| `pkg/middleware/requestid` | `pkg/http/middleware/requestid` | HTTP-specific middleware. |
| `pkg/middleware/requestsize` | `pkg/http/middleware/requestsize` | HTTP-specific middleware. |
| `pkg/middleware/timeout` | `pkg/http/middleware/timeout` | HTTP-specific middleware. |
| `pkg/middleware/compression` | `pkg/http/middleware/compression` | HTTP-specific middleware. |
| `pkg/middleware/static` | `pkg/http/static` | Static asset serving is HTTP-specific. |
| `pkg/middleware/ratelimit` | `pkg/http/ratelimit` with backend contracts in `pkg/cache` or `pkg/coordination` as needed | Keep HTTP policy separate from backend capability. |
| `pkg/security` | `internal/safepath` plus HTTP/security-specific target packages | Completed on this branch. The old root is removed; filepath safety now lives in `internal/safepath`. |
| `pkg/migrate` | `pkg/persistence/relational/migrate` | Migration engine and migrate CLI contribution now live in the relational family. |
| `pkg/eventbus/factory` | removed | Use explicit constructors for Kafka, RabbitMQ, and SQS adapters. |
| `pkg/jobs/factory` | removed | Use explicit constructors for jobs runtimes and backends. |
| `pkg/scheduler` lock-provider implementations | `pkg/coordination/postgres`, `pkg/coordination/redis` | `pkg/scheduler` keeps scheduler runtime/task logic; distributed locks move to coordination. |

Future gRPC runtime work has no legacy source package to migrate. New gRPC transport code must target `pkg/grpc/*` directly instead of `pkg/server` or `pkg/http`.

## Milestones

### Milestone 0: Guardrails And Baseline

### Scope

- Freeze the target architecture in docs.
- Mark legacy/current-implementation docs that would otherwise guide new code incorrectly.
- Define package-level no-new-code rules for `pkg/store` and `pkg/configschema`, and explicitly steer former `pkg/server` and `pkg/controller` responsibilities into their target-state packages.

### Tasks

- Keep [refactoring-requirements.md](./refactoring-requirements.md) as the normative target architecture document.
- Keep current implementation docs accurate, but add transition markers where needed.
- Add this plan and link it from the root docs.

### Done

- New contributors can distinguish current implementation docs from target architecture docs.
- New framework work no longer uses `pkg/store`, `pkg/configschema`, or factory-based router selection as the default pattern, and former `pkg/server` / `pkg/controller` responsibilities are handled by `pkg/http/*` and `pkg/core/errors`.

### Milestone 1: Core Runtime And CLI

### Scope

- Introduce the new runtime composition model.
- Rebuild `pkg/cli` around application execution rather than HTTP service execution.
- Make health orchestration feature-driven.

### Tasks

- Add `pkg/core/app` for lifecycle and application runtime assembly.
- Add `pkg/core/feature` for feature registration and contribution points.
- Add `pkg/core/errors` and move `AppError` and related message contracts there.
- Replace `ServiceCommandOptions` with `AppCommandOptions`.
- Replace `RunServer` with `Run`.
- Introduce feature contribution points for:
  - config extensions
  - CLI commands
  - health contributors
  - startup and shutdown hooks
  - instrumentation hooks
- Keep `version`, `config`, and `healthcheck` in the base CLI.
- Keep optional commands, but move `migrate`, `jobs`, `scheduler`, and `serve` to feature-driven registration instead of hardcoded base behavior.
- Add CLI lifecycle hooks using Cobra execution hooks where appropriate.
- Add an opinionated runtime bootstrap path for logging, metrics, and tracing, but keep it optional so user applications can override or bypass it.
- Make `healthcheck` registry-driven and shared with HTTP management endpoints when HTTP is present.
- Add a global debug/introspection option that exposes only framework introspection data when enabled.

### Done

- A non-HTTP worker can be built with `pkg/cli` and `Run` without importing HTTP packages.
- `healthcheck` runs only checks contributed by included and enabled features.
- The base CLI no longer assumes every application starts an HTTP server.

### Milestone 2: Config, Schema, And Secrets

### Scope

- Break the monolithic config model into composable sections.
- Keep `secrets file` support as a first-class source.
- Make the config system ready for future external secret providers.

### Tasks

- Move schema generation to `pkg/config/schema`.
- Keep `pkg/config` focused on:
  - config loading
  - source merge
  - env/file/flags binding
  - validation orchestration
  - redaction
  - source provenance where useful
- Replace the single giant root config with core config plus feature-owned config sections.
- Keep `ConfigExtensions` as the composition mechanism, but make it the primary path instead of an add-on.
- Make secrets redaction metadata-driven instead of only tracking values loaded from the secrets file.
- Preserve the current source precedence:
  - defaults
  - config file
  - secrets file
  - environment variables
  - CLI flags
- Design, but do not require in the first milestone, provider interfaces for future external secrets sources:
  - AWS Secrets Manager
  - AWS SSM Parameter Store
  - Azure Key Vault
  - Google Secret Manager
- Remove central validation that hardcodes all supported technology types in one place.

### Done

- The core config can be loaded without bringing in HTTP, database, event bus, or email settings.
- Each feature owns its own config struct, schema fragment, and validator.
- `secrets file` still works end-to-end.
- The design no longer assumes that secrets can only come from a local file.

### Milestone 3: HTTP Family Extraction

### Scope

- Make HTTP a first-class transport family instead of the framework default runtime assumption.
- Preserve router replaceability through a stable common contract.

### Tasks

- Create `pkg/http/router` and move the current router contract there.
- Move router adapters under:
  - `pkg/http/router/nethttp`
  - `pkg/http/router/gin`
  - `pkg/http/router/gorilla`
- Keep the shared router contract test suite and adapt it to the new package path.
- Remove runtime string-based router selection as the preferred wiring path.
- Introduce `pkg/http/server` for HTTP runtime bootstrap, public/management servers, and graceful shutdown.
- Move OpenAPI serving and Swagger-related capabilities to `pkg/http/openapi`.
- Move SSE and WebSocket support to `pkg/http/sse` and `pkg/http/ws`.
- Split controller responsibilities:
  - `pkg/http/response`
  - `pkg/http/input`
  - `pkg/core/errors`
- Formalize the HTTP validation pipeline:
  - low-level bind/parse validation
  - contract validation
  - DTO/input validation
- Keep HTTP contract validation provider-driven, with OpenAPI as the initial provider and room for JSON Schema, CUE, or custom implementations.

### Done

- Swapping `nethttp` for `gin` or `gorilla` only changes router construction and wiring.
- An application that does not use HTTP can ignore the entire `pkg/http` family.
- HTTP request validation is explicitly layered instead of being implicit utility behavior.

### Milestone 3B: gRPC Family Extraction

> Current branch state: the full Wave `T3B.1` / `T3B.2` gRPC family is implemented on this branch under `pkg/grpc/*`.

### Scope

- Make gRPC a first-class transport family instead of treating it as an HTTP-adjacent or generic server concern.
- Preserve transport-specific unary and streaming semantics while keeping runtime composition aligned with the shared core model.

### Tasks

- Create `pkg/grpc/server` for gRPC runtime bootstrap and graceful shutdown.
- Create `pkg/grpc/interceptor` for unary and streaming interceptor contracts and reusable transport integration.
- Create `pkg/grpc/status` for framework-to-gRPC error and status mapping.
- Create `pkg/grpc/validation` for contract-validation integration.
- Create optional packages for:
  - `pkg/grpc/health`
  - `pkg/grpc/reflection`
  - `pkg/grpc/auth`
  - `pkg/grpc/stream`
  - `pkg/grpc/metadata`
- Keep transport-specific auth, reflection, and health under `pkg/grpc` instead of `pkg/core` or `pkg/http`.
- Define the gRPC validation pipeline:
  - transport decode and metadata validation
  - contract or schema validation
  - domain or input validation
- Keep gRPC contract validation provider-driven, with Protobuf descriptors as the initial provider and room for Protovalidate, Buf, and custom integrations.
- Integrate gRPC runtime participation with:
  - shared lifecycle hooks
  - shared health registry
  - shared observability bootstrap
  - debug-gated framework introspection
- Add descriptor and CLI support for gRPC feature registration and runtime reporting.

### Done On This Branch

- An application that does not use gRPC can ignore the entire `pkg/grpc` family.
- A gRPC service can run through `Run` and feature-contributed commands without importing HTTP runtime packages unless it also uses HTTP.
- Unary and streaming RPCs are both supported without collapsing them into one generic handler abstraction.
- gRPC validation is explicitly layered.
- gRPC health and reflection are optional family capabilities, not universal runtime assumptions.
- Transport-specific auth glue, shared tenant propagation, and explicit stream helpers live under `pkg/grpc`, not under HTTP or core transport-agnostic packages.

### Milestone 4: HTTP Security, Middleware, Cache, And Session

### Scope

- Rehome HTTP-only middleware and policy packages under the HTTP family.
- Separate cache and session roles from transport-specific glue.

### Tasks

- Move HTTP-only middleware under `pkg/http`.
- Split authentication and authorization into separate HTTP packages.
- Move locale negotiation middleware to `pkg/http/i18n`.
- Create `pkg/cache` for the cache model:
  - `Store`
  - `Policy`
  - `Invalidator`
  - optional tag-based invalidation capability
  - optional stale-while-revalidate capability
- Create `pkg/http/cache` for HTTP response caching behavior.
- Create `pkg/session` for the session role and `pkg/http/session` for HTTP integration.
- Move Redis and Memcached implementations out of `pkg/store/*` and into role-specific packages.
- Keep internal vendor-specific helpers in `internal/*kit` packages where code reuse is real, but do not collapse public APIs into one generic Redis interface.

### Done

- Cache no longer means only `Get/Set/Delete`; it has explicit invalidation and policy concepts.
- Session and cache are distinct roles with their own contracts.
- Redis and Memcached are modeled by capability and role, not by a generic `store` adapter.

### Milestone 5: Persistence Families

### Scope

- Replace `pkg/store` with explicit persistence families.
- Keep application portability at the right abstraction layer.

### Tasks

- Create `pkg/persistence/relational` and move SQL-oriented repository contracts there.
- Introduce a dialect abstraction so the generic relational repository is not Postgres-shaped.
- Move SQL adapters to:
  - `pkg/persistence/relational/postgres`
  - `pkg/persistence/relational/mysql`
- Create `pkg/persistence/document` for document repository contracts and adapters.
- Move MongoDB to `pkg/persistence/document/mongodb`.
- Create `pkg/persistence/keyvalue` and move DynamoDB there as a primary key-value family adapter.
- Keep document-oriented bridges for DynamoDB optional and explicit.
- Create `pkg/persistence/search` and move OpenSearch there.
- Create `pkg/persistence/object` and move S3 there.
- Reserve package families for future additions without implementing them immediately:
  - `pkg/persistence/widecolumn`
  - `pkg/persistence/graph`
  - `pkg/persistence/timeseries`
- Remove `pkg/store` once all adapters have moved.

### Done

- There is no longer a generic top-level `pkg/store` abstraction.
- SQL portability is owned by the relational family and its dialect abstraction.
- Search, object storage, document storage, and key-value storage are modeled as separate families.

### Milestone 6: Eventing, Jobs, Scheduler, And Coordination

### Scope

- Keep transport, schema/contract validation, and domain validation separate.
- Preserve multi-instance scheduling.
- Keep jobs, scheduler, event bus, and pub/sub as distinct concepts.

### Tasks

- Keep `pkg/eventbus` as the durable messaging family.
- Remove central event bus factories and use explicit constructors for concrete brokers.
- Preserve the current event validation layering:
  - envelope validation
  - contract/schema validation
  - domain validation
- Keep event contract validation provider-driven and transport-independent.
- Keep `pkg/jobs` as a distinct runtime and backend contract.
- Remove jobs factories and build jobs runtimes/backends explicitly.
- Keep `pkg/scheduler` for scheduler runtime and task registration.
- Move distributed lock contracts and implementations into `pkg/coordination`.
- Update scheduler runtime to depend on `coordination.LockProvider`.
- Preserve multi-instance safety through contract tests for lock providers and runtime behavior.
- Introduce `pkg/pubsub` for ephemeral fan-out and delivery patterns that are not durable event bus semantics.

### Done

- Event bus, jobs, scheduler, and pub/sub are distinct packages with clear roles.
- Scheduler multi-instance execution still works through distributed locks after the move to `pkg/coordination`.
- Event schema providers are independent from the broker implementation.

### Milestone 7: Distributed Reliability And Consistency

### Scope

- Introduce the framework-level reliability model required by at-least-once delivery.
- Standardize idempotency, deduplication, retry, poison handling, and workflow orchestration semantics.

### Tasks

- Add `pkg/reliability` for cross-transport reliability contracts.
- Introduce explicit contracts for:
  - idempotency keys
  - deduplication
  - inbox/outbox patterns
  - retry policy and retry budgets
  - poison message handling
  - saga or process-manager orchestration
- Align event bus and jobs runtimes with the at-least-once model and stop implying exactly-once behavior at the framework level.
- Add backpressure and consumer-lag visibility hooks where transports can expose them.
- Standardize dead-letter, quarantine, and terminal failure semantics for eventing and jobs.

### Done

- Shared reliability contracts exist and are reusable across HTTP, event bus, and jobs flows.
- The framework explicitly documents and enforces at-least-once semantics instead of implying exactly-once behavior.
- Poison message handling, retry budgets, and consumer lag visibility are modeled consistently.

### Milestone 8: Enterprise Security, Policy, Tenancy, And Audit

### Scope

- Add the enterprise-grade controls that go beyond transport-specific HTTP security placement.
- Make authorization, tenant isolation, rotation, and audit contracts first-class.

### Tasks

- Add `pkg/policy` for transport-independent authorization decisions.
- Support authorization providers that can express:
  - RBAC
  - ABAC
  - policy-engine-backed decisions
- Add `pkg/tenant` for tenant identity and tenant-scoped runtime context.
- Make tenant-aware behavior explicit in:
  - persistence access
  - cache and session keys
  - event and job metadata where relevant
- Add `pkg/audit` for structured audit records and audit sinks.
- Define the tamper-aware audit minimum contract:
  - append-only streams
  - monotonic sequencing
  - digest chaining or equivalent integrity metadata
  - verifier-compatible exports
  - required-vs-best-effort audit policy
- Add structured security event emission hooks for framework-controlled security-sensitive actions.
- Add metadata-driven PII classification and field-level masking rules reusable by config, logs, audit, and transport layers.
- Extend config and transport lifecycle design to cover secret rotation and certificate or mTLS rotation concerns.
- Define minimum framework supply-chain posture expectations for dependency policy and release process:
  - required release artifacts
  - vulnerability policy gates
  - checksum or integrity artifacts
  - provenance or attestation hooks

### Done

- Authorization decisions are no longer tied only to HTTP middleware implementation details.
- Multi-tenant applications have explicit tenant-scoping contracts across runtime and stateful roles.
- Audit and security-event contracts exist separately from ordinary logs.
- Tamper-aware audit has a concrete minimum mechanism and verification contract instead of only a design concern.
- Sensitive-field masking is reusable and metadata-driven across framework surfaces.
- Release posture has explicit minimum artifacts and failure gates instead of an implicit maintainership convention.

### Milestone 9: Enterprise Operations, Governance, And Performance

### Scope

- Raise the runtime model to enterprise operational maturity.
- Add governance and performance requirements to the implementation plan instead of leaving them as documentation-only promises.

### Tasks

- Define framework conventions for:
  - SLI and SLO attachment points
  - golden signals
  - alertability metadata
- Formalize startup, readiness, liveness, and degraded-mode semantics across features.
- Add `pkg/featureflag` for rollout-safe feature gating and config rollout safety patterns.
- Introduce failure-injection hooks and chaos-testing entry points where transport and backend families can support them.
- Add multi-region and disaster-recovery posture guidance to runtime and test strategy where the framework makes guarantees.
- Define explicit posture levels, recovery states, and validation rules for unsupported multi-region or failover combinations.
- Add retention-aware and erasure-aware contracts or orchestration hooks for stateful features.
- Add benchmark, load-test, latency-budget, and performance-regression guidance for the main public families.
- Add capacity-planning hooks or conventions for saturation metrics such as queue depth, lag, and retry backlog.
- Standardize non-functional verification entry points for:
  - performance tests
  - soak tests
  - resilience and failure-injection tests
  - security tests
  - compatibility and migration tests
  - concurrency race and ordering suites

### Done

- Operational semantics are explicit instead of implicit health-check conventions.
- `pkg/featureflag` exists on this branch and owns rollout-safe feature gating plus explicit runtime posture contracts.
- Feature flags and rollout-safety concerns have a shared framework model.
- Governance concerns such as retention and erasure have real contracts or orchestration hooks.
- Multi-region and DR support is modeled through explicit posture metadata and validation, not only narrative guidance.
- Performance engineering is part of the refactor completion criteria, not a follow-up suggestion.

### Milestone 10: Shared Capabilities And Cleanup

### Scope

- Finish the move of packages that do not fit the new taxonomy.
- Delete obsolete factories and legacy package shells.

### Tasks

- Keep `pkg/auth`, `pkg/email`, `pkg/i18n`, `pkg/version`, `pkg/resilience`, and `pkg/observability` public where the existing names are still correct.
- Modularize concrete email providers into subpackages where useful.
- Move the current `pkg/security/filepath.go` logic into `internal/safepath`. Completed on this branch.
- Move relational migration engine code out of `pkg/migrate` and into `pkg/persistence/relational/migrate`.
- The migrate CLI contribution is owned by `pkg/persistence/relational/migrate`.
- Delete obsolete package roots after replacement:
  - `pkg/store`
  - `pkg/configschema`
  - `pkg/controller`
  - `pkg/security` (completed on this branch)
  - `pkg/migrate`
  - old factory packages
- Update package READMEs and top-level docs so they match the final taxonomy.

### Done

- Old package roots that no longer fit the architecture are removed.
- There are no remaining framework-default factories for router, store, event bus, email, or jobs backends.

## Testing Strategy

- Keep and migrate the router property/contract suite.
- Add contract suites for:
  - event bus behavior
  - scheduler lock providers
  - jobs backends
  - cache/session role contracts
  - persistence family contracts where a common subset is promised
- Add reliability-focused suites for:
  - idempotency and deduplication
  - inbox/outbox behavior
  - retry budget and poison-message policy handling
  - tenant isolation where stateful roles promise it
- Add failure-injection and degraded-mode coverage where the runtime exposes those hooks.
- Add benchmark and load-test coverage for the main public runtime families and protect critical paths from unnoticed performance regressions.
- Add soak-test coverage for long-running runtimes and retry-heavy paths.
- Add security-focused suites for authn, authz, masking, tenant isolation, and secret-handling behavior where the framework owns those contracts.
- Add compatibility and migration suites for descriptor evolution, config evolution, and migration-policy-sensitive runtime startup.
- Add race and ordering suites for routers, handlers, schedulers, queues, lock providers, and other concurrent framework contracts where ordering or exclusivity is promised.
- Prefer contract tests per family over giant adapter enumerations in one central package.
- Run package-local tests as code moves instead of waiting for one final big integration pass.

## Global Definition Of Done

The refactor is complete when all of the following are true:

- A non-HTTP application can use `pkg/cli` and `Run` without importing HTTP runtime packages.
- An API gateway application can use the HTTP family without importing persistence, event bus, jobs, scheduler, or email modules it does not use.
- `healthcheck` and HTTP management health both run only the checks contributed by included and enabled features.
- `secrets file` remains supported, and config design is ready for future external secret providers.
- No new framework code depends on `pkg/store`, `pkg/configschema`, or string-based factory selection as the default extension mechanism, and no code reintroduces `pkg/controller`-style umbrella ownership.
- The core runtime does not import concrete adapters.
- Scheduler multi-instance behavior remains verified after lock-provider extraction.
- Distributed reliability semantics are explicit, at-least-once by default, and supported by shared idempotency, deduplication, retry, and poison-handling contracts.
- Authorization, tenant isolation, auditability, and masking are modeled as reusable contracts, not only as package placement choices.
- Startup, readiness, liveness, degraded mode, rollout safety, and alertability conventions are defined and exercised by the runtime model.
- Governance and performance concerns have measurable hooks, tests, or conventions instead of remaining implicit promises.
- Standard non-functional verification entry points exist for performance, soak, resilience, security, compatibility, migration, and concurrency-sensitive framework behavior.
- Current implementation docs are either deleted or aligned with the final package model.
