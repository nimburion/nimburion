# Refactoring Design

This document describes the target architecture behind the refactor. It complements:

- [refactoring-requirements.md](./refactoring-requirements.md) for constraints and rules
- [refactoring-plan.md](./refactoring-plan.md) for execution order and milestones
- [refactoring-user-stories.md](./refactoring-user-stories.md) for actors, workflows, and acceptance-oriented interactions
- [refactoring-traceability.md](./refactoring-traceability.md) for story-to-design traceability

It focuses on package boundaries, runtime composition, contracts, and interaction between modules.
External tooling alignment for `nimbctl` is tracked in [nimbctl-alignment-plan.md](./nimbctl-alignment-plan.md).

## Goals

- Move from a package layout centered around `pkg/server`, `pkg/store`, and a monolithic config root to a modular architecture with explicit families and roles.
- Keep composition compile-time and explicit.
- Let application code depend on stable contracts instead of concrete transports or vendors.
- Keep the framework opinionated where useful, but never mandatory for the final user.

## Non-Goals

- No runtime plugin loader.
- No `init()`-based magical registration as the default extension mechanism.
- No single generic `Store` abstraction for all persistence and infrastructure backends.
- No permanent compatibility layer that preserves the old package layout indefinitely.

## Design Principles

- Composition over selection strings.
- Family-oriented packages over generic umbrellas.
- Transport concerns stay with their transport family.
- Vendor reuse happens in `internal/*kit`, not through oversized public interfaces.
- Validation is layered: transport, contract, domain.
- Health, config, and introspection are contribution-driven.

## Target Package Topology

The target public topology is:

```text
pkg/core/
  app/
  feature/
  errors/
  config/

pkg/cli/

pkg/config/
  schema/

pkg/http/
  server/
  router/
    nethttp/
    gin/
    gorilla/
  openapi/
  sse/
  ws/
  response/
  input/
  authentication/
  authorization/
  cors/
  csrf/
  httpsignature/
  securityheaders/
  cache/
  session/
  i18n/
  middleware/

pkg/persistence/
  relational/
    postgres/
    mysql/
    migrate/
  keyvalue/
    dynamodb/
  document/
    mongodb/
  search/
    opensearch/
  object/
    s3/
  widecolumn/
  graph/
  timeseries/

pkg/cache/
  redis/
  memcached/

pkg/session/
  redis/
  memcached/

pkg/coordination/
  redis/
  postgres/

pkg/pubsub/
  redis/

pkg/eventbus/
  kafka/
  rabbitmq/
  sqs/
  contract/

pkg/jobs/
  redis/

pkg/scheduler/

pkg/reliability/
  idempotency/
  inbox/
  outbox/
  dedup/
  retry/
  saga/

pkg/policy/
pkg/tenant/
pkg/audit/
pkg/featureflag/

pkg/auth/
pkg/email/
pkg/health/
pkg/i18n/
pkg/observability/
pkg/resilience/
pkg/version/
```

The target internal helper topology is:

```text
internal/
  rediskit/
  sqlkit/
  awskit/
  safepath/
```

These internal packages exist only for vendor-specific plumbing reuse and must not define the public semantics of cache, session, jobs, locks, or persistence families.

## Runtime Composition Model

The core runtime is centered on an application plus optional features.

### Core Concepts

- `App`
  - owns lifecycle
  - owns shutdown orchestration
  - owns shared runtime state
- `Feature`
  - contributes capabilities to the app
  - may add config sections, health checks, hooks, and introspection data
- `Runtime`
  - the live, resolved application context
  - contains logger, metrics, tracer, health registry, debug state, and feature-owned services

### Runtime Responsibilities

`pkg/core` owns:

- application lifecycle
- feature registration
- startup and shutdown hooks
- health aggregation
- introspection registry
- shared runtime contracts
- core error model

`pkg/core` does not own:

- HTTP bootstrap
- broker selection
- database selection
- mail provider selection
- scheduler lock provider selection

Those belong to feature families or application wiring.

### Lifecycle Phases

The target lifecycle is:

1. resolve config sources
2. build typed application config
3. initialize observability baseline
4. register enabled features
5. register health contributors and introspection contributors
6. build feature services and transports
7. run the application
8. handle graceful shutdown

This separates configuration, runtime assembly, and transport startup.

## CLI Design

`pkg/cli` remains a shared application CLI framework, not the runtime core.

### Base Model

The base CLI owns:

- `run`
- `config`
- `version`
- `healthcheck`

Optional commands are contributed by features:

- `serve`
- `migrate`
- `jobs`
- `scheduler`
- `openapi`

The command model must not assume that every application is an HTTP service.

### Naming

- replace `ServiceCommandOptions` with `AppCommandOptions`
- replace `RunServer` with `Run`
- keep `serve` as an HTTP-specific command or alias

### Opinionated Bootstrap

The CLI framework should offer an opinionated bootstrap path that wires:

- logger
- metrics
- tracing
- signal handling
- graceful shutdown

This bootstrap path is the preferred default, but it must remain optional so applications can override parts of the runtime assembly when needed.

### CLI Lifecycle

CLI lifecycle hooks are supported through Cobra and should be exposed explicitly for:

- before app bootstrap
- after config resolution
- before command execution
- after command execution
- before shutdown

### Debug And Introspection

When framework debug is enabled, the CLI should be able to expose:

- enabled features
- contributed health checks
- registered routes when HTTP is present
- middleware chain when HTTP is present
- registered event handlers, job handlers, or scheduler tasks when present

This data belongs to framework introspection, not to business telemetry.

## Config And Schema Design

`pkg/config` remains the infrastructure package for configuration.

### Responsibilities

`pkg/config` owns:

- loading from files
- env binding
- flags binding
- merge precedence
- validation orchestration
- redaction
- secrets source integration

`pkg/config/schema` owns:

- schema generation
- schema merge
- feature-owned schema fragments
- defaults metadata exposure where useful

### Config Ownership

The framework moves away from a single monolithic framework-owned config struct.

The target model is:

- core exposes minimal shared config
- each feature exposes its own config struct
- the application composes only the feature configs it uses

Example:

```go
type GatewayConfig struct {
	Core   coreconfig.Config `mapstructure:"core"`
	HTTP   httpserver.Config `mapstructure:"http"`
	Auth   auth.Config       `mapstructure:"auth"`
	Cache  cache.Config      `mapstructure:"cache"`
	OpenAPI httpopenapi.Config `mapstructure:"openapi"`
}
```

The exact struct composition can evolve, but ownership must remain feature-local.

### Secrets

The official current source remains `secrets file`.

Config precedence remains:

1. defaults
2. config file
3. secrets file
4. environment variables
5. CLI flags

The design must already allow future external secret providers:

- AWS Secrets Manager
- AWS SSM Parameter Store
- Azure Key Vault
- Google Secret Manager

The important rule is that modules declare sensitive fields, but they do not know where the secret value came from.

### Sensitive Field Metadata Model

Sensitive handling is field metadata, not source inference.

The target model for each config field should be able to express:

- `classification`
  - `public`
  - `sensitive`
  - `secret`
- `redaction`
  - `none`
  - `mask`
  - `full`
- `render_target`
  - `config`
  - `secrets`
- `allowed_sources`
  - `default`
  - `config_file`
  - `secrets_file`
  - `env`
  - `flag`
  - `secret_provider`

The exact Go API can evolve, but the design target is explicit metadata close to the field definition, for example:

```go
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username" nimb:"classification=sensitive,redaction=mask,render_target=config,allowed_sources=config_file,env,flag"`
	Password string `mapstructure:"password" nimb:"classification=secret,redaction=full,render_target=secrets,allowed_sources=secrets_file,env,flag,secret_provider"`
}
```

Modules own field classification, but they do not implement source resolution, redaction walkers, or schema emission themselves.

### Source Precedence And Provenance

Mixed-source resolution is normal and must be modeled explicitly.

For a given field, the framework resolves:

1. field metadata
2. effective value by precedence
3. sanitized provenance

That means a field may have:

- metadata declared by the owning module
- a default value in code
- an override from config file
- an override from `secrets file`
- an override from env
- an override from flags
- an override from an external secret provider

The classification does not change when the winning source changes.

The runtime may keep sanitized provenance such as:

- `default`
- `config_file`
- `secrets_file`
- `env`
- `flag`
- `secret_provider:aws-secrets-manager`

but provenance must never downgrade redaction rules or expose the winning value in debug output.

### Redaction

Redaction must be metadata-driven.

Do not rely only on “this value came from the secrets file” because that fails once the same secret can come from:

- secrets file
- env vars
- future external secret providers

### Redaction Propagation

The same redaction engine must drive all framework-managed surfaces.

At minimum, sensitive metadata must propagate to:

- `pkg/config/schema`
  - mark secret-like fields with schema metadata such as `writeOnly` and framework-specific sensitivity annotations where needed
- `config render`
  - place `render_target=secrets` fields in the secrets output when split-output render is enabled
- `config show`
  - render secret fields redacted and sensitive fields masked according to policy
- framework-managed logging
  - structured config fields must pass through the same redaction walker before emission
- panic and debug dumps
  - config snapshots must be serialized through the redaction engine instead of raw `%+v` or direct marshaling
- framework introspection
  - introspection may expose field names, classification, and sanitized provenance, but not the underlying secret value

### Error And Debug Output Policy

Error messages and debug output must follow the same field metadata model.

Rules:

- validation errors may name the field path, rule, and sanitized source provenance
- validation errors must not include raw values for `sensitive` or `secret` fields
- panic handlers must redact config snapshots recursively before logging or returning debug payloads
- debug dumps may expose:
  - field name
  - classification
  - render target
  - winning source
- debug dumps must not expose:
  - raw `secret` values
  - unmasked `sensitive` values

## HTTP Family Design

HTTP becomes a family under `pkg/http`, not the default shape of the framework.

### Router Contract

The current router abstraction remains valid and moves to `pkg/http/router`.

Goals:

- interchangeable adapters
- route grouping
- middleware composition
- transport-neutral context abstraction

The application code must never depend directly on:

- `*gin.Context`
- `*mux.Router`
- adapter-specific middleware signatures

If it does, router replaceability is already lost.

### HTTP Server

`pkg/http/server` owns:

- public and management server bootstrap
- graceful shutdown
- management endpoint exposure when enabled
- HTTP-specific health endpoint integration

It does not own application lifecycle or generic runtime bootstrap; that remains in `pkg/core` and `pkg/cli`.

### HTTP Validation Pipeline

The HTTP pipeline has three distinct layers:

1. bind/parse validation
2. contract validation
3. DTO/input validation

#### Bind/Parse Validation

This is adapter-level:

- request body decode
- path/query/header extraction
- content-type and shape errors

#### Contract Validation

This is provider-driven and belongs to the HTTP family.

Providers may include:

- OpenAPI
- JSON Schema
- CUE
- custom validators

#### DTO/Input Validation

This is application-level semantics:

- field relationships
- business preconditions
- normalization and semantic rules

It must not be confused with transport parsing or contract validation.

### HTTP Capability Packages

The HTTP family should own:

- authentication
- authorization
- security headers
- CSRF
- HTTP signatures
- CORS
- OpenAPI
- request/response helpers
- SSE
- WebSocket
- HTTP-specific middleware

## Persistence Design

There is no single public `store` abstraction in the target design.

### Persistence Families

Persistence families are:

- relational
- keyvalue
- document
- widecolumn
- search
- graph
- timeseries
- object

These are families, not vendors.

### Family Contracts

Each family has a small minimum contract and optional capabilities.

Examples:

- relational
  - repositories
  - transactions
  - dialect
- document
  - document repository
  - document query
- search
  - index
  - search request
  - search result
- object
  - put/get/delete/list/presign

The framework must promise portability only for the shared contract subset.

### Relational Design

The relational family is where SQL portability lives.

It owns:

- relational repository contracts
- transaction manager
- unit of work
- dialect abstraction

The generic relational repository must stop being Postgres-shaped. Placeholder, quoting, pagination, and other SQL differences belong in the dialect.

### Document Design

The document family owns:

- document repository contracts
- document query options
- adapters such as MongoDB

Document portability is limited to the common subset. Provider-specific aggregation or query languages must remain isolated.

### KeyValue Design

Key-value is its own family.

Examples:

- Redis
- Memcached
- DynamoDB in its primary family classification

Key-value family contracts should remain small:

- get
- put
- delete
- optional ttl
- optional batch

### Search Design

Search is its own family.

Application code should not depend on raw vendor DSL blobs by default. The contract should be centered around:

- indexing
- deleting
- searching with a typed request object

Vendor-specific DSL can exist in specialized repositories, but it should not define the public search contract.

### Future Families

The design reserves package space for:

- wide-column stores
- graph stores
- time-series stores

They do not need first-milestone implementations, but the taxonomy must leave room for them.

## Cache, Session, Coordination, And Other Roles

These are not persistence families. They are operational roles.

### Cache

`pkg/cache` owns the cache model:

- store
- policy
- invalidation
- optional stale-while-revalidate
- optional tag-based invalidation

`pkg/http/cache` is the HTTP integration layer above it.

### Session

`pkg/session` owns the session role. `pkg/http/session` owns the cookie and request integration.

### Coordination

`pkg/coordination` owns cross-instance primitives such as:

- distributed locks
- leases

Scheduler lock providers should move here, while the scheduler runtime remains under `pkg/scheduler`.

### Vendor Reuse

Redis and Memcached implementations are split by role, for example:

- `pkg/cache/redis`
- `pkg/session/redis`
- `pkg/coordination/redis`
- `pkg/jobs/redis`
- `pkg/pubsub/redis`

Common low-level Redis code belongs in `internal/rediskit`, not in a shared public Redis interface.

## Eventing Design

The design distinguishes clearly between:

- durable event transport
- ephemeral pub/sub
- jobs backends
- scheduler runtime

### Event Bus

`pkg/eventbus` remains the durable messaging family.

Its public contract should stay close to the least common denominator:

- publish
- subscribe
- unsubscribe
- health

The business layer must not depend on broker-specific details such as:

- partition mechanics
- exchange types
- routing keys
- queue groups

### Event Validation

Event processing also has layered validation:

1. envelope validation
2. contract validation
3. domain validation

Contract validation providers include:

- Avro
- Protobuf
- JSON Schema
- CUE
- custom validators

Broker selection and contract validation providers must remain independent axes.

### Pub/Sub

`pkg/pubsub` is reserved for ephemeral fan-out patterns that do not have durable event bus semantics.

### Jobs

`pkg/jobs` remains a separate concept from event bus:

- jobs are lease-aware work items
- event bus is durable messaging
- pub/sub is ephemeral fan-out

Jobs backends must remain explicit and runtime factories must be removed in favor of direct constructors.

### Scheduler

`pkg/scheduler` owns:

- task registration
- run loop
- execution policy
- scheduler observability

It depends on `pkg/coordination` for distributed singleton behavior across instances.

Multi-instance behavior is a hard design requirement and must be preserved through lock-provider extraction.

## Distributed Reliability Design

Distributed reliability is a first-class framework concern, not only an application convention.

### Delivery Model

The framework default model is:

- at-least-once delivery for distributed transports unless a backend proves stronger semantics
- no exactly-once claim at the application contract level
- explicit idempotency and deduplication at the framework boundary

HTTP commands, event consumers, and jobs workers must expose their delivery assumptions clearly.

### Idempotency, Deduplication, And Inbox/Outbox

`pkg/reliability` owns the generic contracts for:

- idempotency keys
- deduplication
- inbox tracking
- outbox publishing

The target public contracts should stay small and backend-agnostic, for example:

- `IdempotencyStore`
- `Deduplicator`
- `InboxStore`
- `OutboxStore`

Transport-specific glue belongs with the transport family:

- HTTP idempotency integration belongs under `pkg/http`
- event-consumer inbox and dedup integration belongs under `pkg/eventbus`
- jobs dedup or lease-aware idempotency integration belongs under `pkg/jobs`

The framework should prefer explicit scopes and ownership over magical global deduplication.

### Retry, Poison, And Backpressure

Retry behavior must be standardized at the framework level.

The target taxonomy is:

- immediate retry
- delayed retry
- handoff to dead-letter or quarantine
- terminal failure with explicit operator visibility

Retry contracts must also include:

- retry budget by attempt count or time window
- poison message policy
- backpressure visibility
- consumer lag visibility where the transport exposes it

These contracts belong to the reliability model even when concrete execution lives in `pkg/eventbus` or `pkg/jobs`.

### Saga And Process Managers

Long-running cross-service workflows must be explicit.

`pkg/reliability/saga` is reserved for:

- saga orchestration contracts
- process-manager state transitions
- compensation coordination

This must remain an optional capability. The framework should not hide distributed workflow state inside unrelated event handlers.

## Security, Policy, Tenant, And Audit Design

The framework must separate authentication, authorization, tenant context, and audit concerns instead of collapsing them into one broad security layer.

### Authorization Providers

`pkg/policy` owns authorization decision contracts.

The goal is to support:

- RBAC
- ABAC
- policy-engine-backed decisions

Providers can include:

- OPA-style engines
- Cedar-style engines
- custom in-process providers

`pkg/http/authorization` adapts HTTP request context to these providers, but the decision engine itself must remain transport-independent.

### Tenant Context And Isolation

`pkg/tenant` owns tenant identity and tenant-scoped runtime context.

Tenant handling must be explicit in:

- HTTP request context
- event and job metadata
- cache keys
- session state
- persistence access patterns

The design must make tenant-blind state access a visible exception, not the default path.

### Secret And Certificate Rotation

Rotation is an operational lifecycle, not only a config concern.

The design must allow:

- secrets refresh without changing feature-owned config contracts
- certificate and mTLS material rotation for HTTP transports
- observable rotation state and last-refresh health data where relevant

This primarily affects `pkg/config`, `pkg/http/server`, and future secret-provider integrations.

### Security Events And Audit

`pkg/audit` owns structured audit records and audit sinks.

The design must distinguish:

- operational logs
- business telemetry
- security events
- tamper-aware audit trails

Security-sensitive framework actions should be able to emit structured security events without coupling application code to a specific sink.

#### Tamper-Aware Audit Minimum Mechanism

The minimum framework mechanism is:

- append-only audit records
- explicit `stream_id` and monotonic `sequence`
- canonical-record digest plus `prev_digest` chaining inside a stream
- verifier-compatible exports or verifier hooks
- required-vs-best-effort sink policy per audit class

The framework does not need to promise physically immutable storage for every sink, but it must promise that framework-managed audit writes preserve chainable integrity metadata and cannot silently collapse into ordinary logs.

Security-sensitive framework actions should default to `required` audit policy. When a required audit write fails, the outcome must be explicit:

- fail the protected action
- mark the runtime degraded or not-ready according to feature policy
- or both

See [refactoring-operational-specs.md](./refactoring-operational-specs.md) for the concrete minimum contract.

### PII Classification And Masking

PII classification and masking must be metadata-driven and reusable across:

- config rendering
- logs
- audit sinks
- transport responses

Masking rules must not be reimplemented separately in each package.

### Supply-Chain And Release Posture

The framework's own release process must treat supply-chain posture as part of the design envelope.

That includes:

- dependency hygiene expectations
- provenance or signing hooks where adopted
- SBOM or dependency reporting hooks where useful

These concerns are mostly process-facing, but they must stay visible in the architecture documents because the framework promises enterprise readiness.

#### Minimum Supply-Chain Contract

The framework release process should produce a minimum artifact set and release gate set.

Minimum artifact categories:

- dependency inventory or SBOM
- vulnerability policy result
- checksum manifest
- build metadata and source revision identity
- provenance or attestation artifact where the release path supports it

Minimum gate categories:

- missing required artifact categories fail the release path
- unresolved policy-violating vulnerabilities fail the release path unless an explicit exception exists
- toolchain and dependency policy must be visible instead of implicit

This is a framework-level release contract, not a best-effort maintainer note. See [refactoring-operational-specs.md](./refactoring-operational-specs.md).

## Shared Capabilities

These packages remain public because the names are still accurate:

- `pkg/auth`
- `pkg/audit`
- `pkg/email`
- `pkg/featureflag`
- `pkg/i18n`
- `pkg/observability`
- `pkg/policy`
- `pkg/reliability`
- `pkg/resilience`
- `pkg/tenant`
- `pkg/version`
- `pkg/health`

### Auth

`pkg/auth` remains the place for authentication and credential-flow helpers.

HTTP authentication and authorization integration belong under `pkg/http`.

### Email

`pkg/email` remains the family for outbound mail delivery.

Concrete providers should move toward explicit provider packages instead of a central factory.

### Observability

`pkg/observability` remains public and owns:

- logger
- metrics
- tracing

Transport-specific instrumentation stays with the transport family.

### Feature Flags

`pkg/featureflag` owns rollout-safe feature gating.

It must support:

- typed flag evaluation
- runtime context-aware targeting
- safe defaults when providers are unavailable

Deployment systems still own progressive delivery, but the framework needs a reusable contract for feature-gated code paths and config rollout safety.

### Resilience

`pkg/resilience` remains public because it is reusable by application code and not only framework internals.

## Operations And Runtime Posture Design

Enterprise operations require standardized runtime semantics, not only optional dashboards.

### Operational Signals

The framework should define conventions for:

- SLI naming
- SLO attachment points
- golden signals for HTTP, eventing, jobs, and scheduler runtimes
- alertability metadata for critical dependencies

`pkg/observability` and `pkg/health` should expose shared conventions so features do not invent incompatible operational signals.

### Startup, Readiness, Liveness, And Degraded Mode

`pkg/core` and `pkg/health` must distinguish:

- startup
- readiness
- liveness
- degraded operation

This allows applications to tolerate partial dependency failure where the feature model permits it instead of reducing every issue to a binary up/down state.

### Rollout Safety

The framework should support rollout safety through:

- feature flags
- config validation before activation
- explicit degraded-mode policies
- optional compatibility checks during startup

Blue/green, canary, and progressive delivery remain deployment concerns, but the framework must expose safe runtime hooks for them.

### Failure Injection, Multi-Region, And Disaster Recovery

The design should leave room for:

- failure injection hooks in critical transports and backends
- multi-region-safe delivery and idempotency assumptions
- disaster recovery posture documentation and recovery testing hooks

These are runtime behaviors and test concerns, not only operational runbooks.

#### Multi-Region And Disaster-Recovery Contract

The shared runtime design must move beyond "leave room for" and define posture levels that features can claim explicitly:

- `single_region_only`
- `single_writer_multi_region_failover`
- `multi_region_active_passive`
- `multi_region_active_active_subset`

When a feature claims one of the multi-region-capable levels, the contract must also define:

- region scope
- writer topology
- replay requirement after failover
- ordering scope
- idempotency and deduplication expectations
- coordination or leader-lock locality rules

Recovery-sensitive features should be able to surface recovery-oriented states such as `failover_pending`, `replay_required`, `reconciling`, and `degraded_recovery`.

Unsupported combinations must fail validation explicitly instead of being left to rollout-time discovery. See [refactoring-operational-specs.md](./refactoring-operational-specs.md) for the minimum rules.

## Data Governance And Compliance Design

Governance and compliance must appear as explicit contracts where the framework manages state or emits evidence.

### Auditability And Retention

The framework should support:

- durable audit records for security-sensitive actions
- retention-aware storage and object lifecycle hooks
- retention-aware cache and session guidance where deletion semantics matter

### Erasure And Compliance Workflows

The design must leave room for:

- right-to-erasure workflows
- compliance-driven deletion orchestration
- durable evidence that deletion workflows were requested and processed

This does not mean the framework can erase arbitrary vendor-specific data automatically, but it must provide consistent orchestration hooks.

### Tenant-Aware State

Tenant isolation must extend to stateful roles, not only request context.

That includes:

- persistence repositories
- cache keys and invalidation scopes
- session storage
- job and event metadata where tenant identity affects routing or ownership

## Performance Engineering Design

The framework must make performance a design-time concern rather than a post-release exercise.

### Benchmark And Load Hooks

Critical public contracts should support:

- repeatable microbenchmarks
- transport-level load profiles
- backend family benchmark harnesses where portability is promised

### Latency Budgets And Regression Detection

The design should support latency-budget-aware instrumentation and regression testing for:

- HTTP request paths
- event publish and consume paths
- jobs scheduling and execution paths
- persistence hot paths where the framework provides shared abstractions

### Capacity And Saturation Signals

Features should be able to expose capacity-related metrics such as:

- queue depth
- consumer lag
- retry backlog
- saturation or contention signals

These signals belong in the framework observability model so operators can reason about system limits consistently.

## Verification Design

Non-functional verification must be treated as a first-class architecture concern rather than a best-effort CI add-on.

### Verification Families

The target verification model includes:

- performance tests
- soak tests
- resilience and failure-injection tests
- security tests
- compatibility and migration tests
- concurrency race and ordering suites

### Standard Entry Points

Each family should have a standard entry-point pattern even when the exact tooling differs by package:

- package-local microbenchmarks for hot paths
- repeatable load-test harnesses for transport families
- long-running soak scenarios for durable runtimes
- failure-injection harnesses for resilience and degraded-mode behavior
- security-focused suites for auth, masking, tenant isolation, and secret handling
- compatibility and migration suites for descriptor, config, and state-transition evolution
- race and ordering suites for concurrent handlers, schedulers, lock providers, queues, and other concurrent runtimes

### Scope Boundaries

The framework should not promise that every package needs every test family.

Instead:

- transport families should define load, failure-injection, and ordering expectations where relevant
- distributed runtimes should define soak, compatibility, and concurrency expectations where relevant
- security-sensitive features should define masking, authz, and tenant-isolation verification where relevant
- persistence and migration layers should define compatibility and migration verification where shared semantics are promised

### CI And Runtime Cost Model

Verification cost must be explicit.

The target operating model is:

- fast local feedback for unit and focused contract tests
- dedicated environments for external-service integration tests
- standard but potentially separate entry points for long-running soak, heavy load, security, and compatibility suites

This avoids forcing every expensive suite into the default fast path while still making those suites part of the framework quality model.

## Current-To-Target Wiring Model

The target bootstrap shape is:

```go
type GatewayConfig struct {
	Core  coreconfig.Config     `mapstructure:"core"`
	HTTP  httpserver.Config     `mapstructure:"http"`
	Auth  auth.Config           `mapstructure:"auth"`
	Cache cache.Config          `mapstructure:"cache"`
}

func main() {
	cmd := cli.NewAppCommand(cli.AppCommandOptions{
		Name: "gateway",
		LoadConfig: loadGatewayConfig,
		Run: func(ctx context.Context, cfg *GatewayConfig, rt *coreapp.Runtime) error {
			r := nethttp.NewRouter()
			srv := httpserver.New(cfg.HTTP, r)
			return srv.Run(ctx)
		},
		Features: []cli.Feature{
			httpfeature.New(),
			authfeature.New(),
			cachefeature.New(),
		},
	})

	_ = cmd.Execute()
}
```

The exact APIs can change during implementation, but the boundaries must remain:

- config is application-composed
- runtime assembly is explicit
- router/provider choice is explicit
- features contribute capabilities
- the core does not import concrete adapters

## Architecture Guardrails

The following are design violations in the target architecture:

- adding new concrete adapters under `pkg/store`
- adding new responsibilities to `pkg/controller`
- adding new features under `pkg/server` that are not strictly transitional
- introducing new framework-default string factories when constructor injection is possible
- making business code depend directly on adapter-specific transport or vendor types
- using secrets-source provenance as the only redaction mechanism
- implying exactly-once behavior where the framework only controls at-least-once delivery
- treating logs as the only audit mechanism for security-sensitive actions
- making tenant-aware features rely on tenant-blind cache, session, or state keys by default
- hardcoding authorization decisions into transport handlers instead of routing them through a provider contract

## Open Design Constraints

These constraints are already fixed even if exact APIs may evolve:

- compile-time composition only
- `Run` as the primary app entrypoint
- registry-driven health aggregation
- `secrets file` retained, external secret providers future-ready
- scheduler multi-instance support preserved
- no single generic `store` abstraction
- at-least-once distributed reliability model with explicit idempotency and deduplication
- enterprise controls must be modeled as contracts, not left to package placement only
