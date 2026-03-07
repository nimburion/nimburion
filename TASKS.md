# Refactoring Tasks

This document turns the refactoring docs into an implementation backlog that can be executed task by task without leaving design decisions to the implementer.

It complements:

- [docs/refactoring-requirements.md](./docs/refactoring-requirements.md)
- [docs/refactoring-design.md](./docs/refactoring-design.md)
- [docs/refactoring-plan.md](./docs/refactoring-plan.md)
- [docs/refactoring-user-stories.md](./docs/refactoring-user-stories.md)
- [docs/refactoring-traceability.md](./docs/refactoring-traceability.md)
- [docs/refactoring-operational-specs.md](./docs/refactoring-operational-specs.md)
- [docs/testing.md](./docs/testing.md)
- [docs/test-infrastructure.md](./docs/test-infrastructure.md)
- [docs/nimbctl-alignment-plan.md](./docs/nimbctl-alignment-plan.md)
- [docs/nimbctl-service-descriptor-design.md](./docs/nimbctl-service-descriptor-design.md)

## Global Execution Rules

1. Every implementation task must reference the relevant story IDs from [docs/refactoring-traceability.md](./docs/refactoring-traceability.md).
2. No task is complete until:
   - the code builds
   - the task-local verification gate passes
   - obsolete wiring replaced by the task is removed or explicitly deleted in the same wave
   - required docs and READMEs are written
3. Backward compatibility is not a goal. Temporary bridges are allowed only inside the same task or wave to keep the repository buildable.
4. No new framework work may extend:
   - `pkg/store`
   - `pkg/server`
   - `pkg/controller`
   - `pkg/configschema`
5. New gRPC work must target `pkg/grpc/*` directly and must not be added under `pkg/http`, `pkg/server`, or `pkg/controller`.
6. Every task that creates, migrates, or finalizes a package or subpackage under `pkg/*` must end by creating or updating a `README.md` in that package or subpackage.
7. Every `README.md` introduced or updated by this refactor must include:
   - package purpose
   - owned contracts
   - composition and wiring expectations
   - non-goals
   - validation, lifecycle, or runtime semantics where relevant
   - testing expectations
   - whether the package is target-state or transitional
8. Internal helper packages under `internal/*` created by the refactor also get a short `README.md`.
9. Documentation updates are part of the task, not a later cleanup pass.
10. Every implementation PR created from this file must declare:
    - task ID
    - story IDs
    - milestone / wave
    - touched packages
    - deleted legacy code
    - verification commands actually run
    - README / docs changed
11. A task may create a temporary compatibility shim only if the shim is deleted in the same wave and the PR states where that deletion occurs.
12. Cross-repo work must not block in-repo work unless the task explicitly says so.

## Verification Gates

Use these gates per task:

- `Build gate`
  - affected packages compile
- `Fast gate`
  - fast tests for the touched area pass
- `Integration gate`
  - run when adapters, external-service roles, or transport runtimes are touched
- `Contract gate`
  - run or add the relevant family contract suite
- `Non-functional gate`
  - required only for tasks in Wave 5 and later, or earlier if the task changes promised runtime semantics

## Canonical Verification Commands

The repository may use `make`, `mage`, `task`, or plain `go test`. The concrete command names can differ, but every task must end with stable command entry points equivalent to the following shapes:

- `build repo or package`
  - `go test ./path/... -run '^$'`
- `fast package lane`
  - `go test ./path/... -short`
- `integration lane`
  - `go test ./path/... -run Integration`
- `contract lane`
  - `go test ./path/... -run Contract`
- `non-functional lane`
  - `go test ./path/... -run 'Performance|Load|Soak|Resilience|Security|Compatibility|Race|Ordering'`

If the repo later adds wrappers such as `make test-fast-http-router`, `make test-contract-grpc`, or `make test-nonfunctional`, those wrapper names become the preferred commands and should be backfilled into the relevant task descriptions.

## Minimum Expected Suite Mapping

- `pkg/core`, `pkg/cli`, `pkg/config`
  - build + fast + targeted integration only if config sources or startup contracts require it
- `pkg/http/router`, `pkg/http/server`
  - build + fast + router contract suite
- `pkg/grpc/*`
  - build + fast + interceptor and lifecycle suites
- `pkg/persistence/*`
  - build + integration + family contract suites
- `pkg/eventbus`, `pkg/jobs`, `pkg/scheduler`, `pkg/coordination`, `pkg/pubsub`
  - build + integration + behavior contract suites
- `pkg/reliability`, `pkg/policy`, `pkg/tenant`, `pkg/audit`, `pkg/featureflag`
  - build + fast + targeted integration and non-functional suites where the package owns runtime promises
- descriptor / CLI contract surfaces
  - build + fast + descriptor-shape tests + mixed-app contract tests
- cross-repo `nimbctl`
  - build + descriptor compatibility + orchestration behavior tests in the `nimbctl` repo

## Task Format

Every task below is decision-complete and must be executed in the given order unless the task explicitly says it can run in parallel.

Each task contains:

- `Stories`
- `Depends on`
- `Outputs`
- `Implementation`
- `Verification`
- `Docs`
- `Exit condition`

## PR Checklist Template

Every implementation PR created from this file should include the following checklist in the PR body:

- `Task`: `T?.?`
- `Stories`: `...`
- `Wave`: `...`
- `Touched packages`: `...`
- `Legacy code deleted in this PR`: `...`
- `Verification commands run`: `...`
- `Docs / READMEs updated`: `...`
- `Temporary shims introduced`: `none` or explicit removal target in same wave

## Wave 0 - Guardrails And Baseline

### T0.1 Architectural Guardrails

- Stories: `1.1`, `3.1`, `3B.1`, `5.1`
- Depends on: none
- Outputs:
  - enforced no-new-code policy for legacy package roots
  - enforced placement rules for gRPC
- Implementation:
  - codify package-boundary rules used during review and lint:
    - no new code in `pkg/store`, `pkg/server`, `pkg/controller`, `pkg/configschema`
    - `pkg/core` must not import transport families
    - gRPC code must live in `pkg/grpc`
    - provider-specific validation logic must stay out of `pkg/core`
  - align review guidance, lint guidance, and refactor plan text
- Verification:
  - build gate: no code changes required
  - fast gate: not required
  - contract gate: not required
- Docs:
  - keep [docs/linter.md](./docs/linter.md) and [docs/comment-templates.md](./docs/comment-templates.md) aligned
- Exit condition:
  - implementers can tell from repo docs where new code is forbidden and where it belongs

### T0.2 Refactor Task Workflow

- Stories: all
- Depends on: `T0.1`
- Outputs:
  - repeatable task structure for issues and PRs
- Implementation:
  - standardize task metadata:
    - story IDs
    - milestone
    - touched packages
    - verification gate
    - README impact
    - deletions performed
  - use this `TASKS.md` as the source of truth for execution order
  - require the PR checklist template in every implementation PR
- Verification:
  - build gate: not required
- Docs:
  - this file is the output
- Exit condition:
  - every later implementation task can be opened without inventing scope or done criteria

### T0.3 Test Lane Baseline

- Stories: `12.1` to `12.6`
- Depends on: `T0.2`
- Outputs:
  - named verification lanes for all later waves
  - canonical command shapes for build / fast / integration / contract / non-functional lanes
- Implementation:
  - lock the meaning of build, fast, integration, contract, and non-functional gates
  - map family-level suites to the families described in [docs/testing.md](./docs/testing.md)
  - decide the first stable command names or wrappers for all lanes, even if they shell out to `go test`
- Verification:
  - build gate: not required
- Docs:
  - keep [docs/testing.md](./docs/testing.md) and [docs/test-infrastructure.md](./docs/test-infrastructure.md) aligned
- Exit condition:
  - later tasks can point to exact test expectations instead of vague "run relevant tests"

## Wave 1 - Core Runtime And CLI

### T1.1 Create `pkg/core/app`

- Stories: `1.1`, `1.2`, `6.1`
- Depends on: `T0.1` to `T0.3`
- Outputs:
  - `pkg/core/app`
  - application lifecycle model
  - shutdown orchestration
- Implementation:
  - introduce the `App` runtime model described in [docs/refactoring-design.md](./docs/refactoring-design.md)
  - move lifecycle ownership out of HTTP-specific bootstrap code
  - define phases for:
    - config resolution
    - observability baseline
    - feature registration
    - health and introspection registration
    - feature service construction
    - runtime execution
    - graceful shutdown
- Verification:
  - build gate: `pkg/core/...`
  - fast gate: lifecycle tests for startup and shutdown phases
- Docs:
  - write `pkg/core/app/README.md`
- Exit condition:
  - the runtime can exist without any HTTP bootstrap package

### T1.2 Create `pkg/core/feature`

- Stories: `1.1`, `1.3`, `6.3`
- Depends on: `T1.1`
- Outputs:
  - `pkg/core/feature`
  - feature contribution interfaces
- Implementation:
  - introduce feature contribution points for:
    - config extensions
    - command registration
    - health contributors
    - startup hooks
    - shutdown hooks
    - instrumentation hooks
    - introspection contributors
  - ensure the core runtime depends on feature contracts, not concrete transports or backends
- Verification:
  - build gate: `pkg/core/...`
  - fast gate: feature registration and hook-order tests
- Docs:
  - write `pkg/core/feature/README.md`
- Exit condition:
  - one feature can contribute behavior without editing a base runtime implementation

### T1.3 Create `pkg/core/errors`

- Stories: `1.1`, `3.3`
- Depends on: `T1.1`
- Outputs:
  - `pkg/core/errors`
  - transport-neutral error contracts
- Implementation:
  - move `AppError` and related contracts out of `pkg/controller` and `pkg/i18n`
  - define error shape and transport-neutral semantics
  - leave HTTP response mapping outside `pkg/core/errors`
- Verification:
  - build gate: packages using the error contracts compile without importing `pkg/controller`
  - fast gate: error classification and wrapping tests
- Docs:
  - write `pkg/core/errors/README.md`
- Exit condition:
  - transport adapters consume core errors instead of defining application error semantics

### T1.4 Refactor `pkg/cli` to `Run`

- Stories: `1.1`, `1.2`, `1.3`, `6.2`
- Depends on: `T1.1`, `T1.2`, `T1.3`
- Outputs:
  - `AppCommandOptions`
  - `Run` as primary entrypoint
  - feature-driven optional commands
- Implementation:
  - replace `ServiceCommandOptions` with `AppCommandOptions`
  - replace `RunServer` with `Run`
  - keep only base commands in the shared CLI:
    - `run`
    - `config`
    - `version`
    - `healthcheck`
  - move `serve`, `migrate`, `jobs`, `scheduler`, `openapi` to feature-driven registration
  - expose CLI lifecycle hooks using Cobra execution hooks
  - keep an opinionated bootstrap path for logging, metrics, tracing, signal handling, and graceful shutdown, but make it optional
- Verification:
  - build gate: `pkg/cli/...`
  - fast gate: command registration and lifecycle-hook tests
  - contract gate: no command from an absent feature is exposed
- Docs:
  - write or update `pkg/cli/README.md`
  - update `pkg/cli/migrate/README.md` if that package is kept
- Exit condition:
  - a worker or consumer can run on the shared CLI without importing HTTP runtime packages

### T1.5 Shared Health And Debug Introspection

- Stories: `6.1`, `6.3`, `9.2`
- Depends on: `T1.2`, `T1.4`
- Outputs:
  - feature-driven health aggregation
  - debug-gated framework introspection
- Implementation:
  - make the shared health registry the only health aggregation source
  - ensure CLI `healthcheck` and management surfaces use the same registry
  - add a global debug option for framework introspection
  - keep introspection disabled by default
- Verification:
  - build gate: `pkg/health`, `pkg/core`, `pkg/cli`
  - fast gate: health contribution and debug-gating tests
- Docs:
  - update [pkg/health/README.md](./pkg/health/README.md)
  - write README for any new introspection package introduced
- Exit condition:
  - health only reflects included and enabled features, and framework introspection is not exposed unless debug is enabled

## Wave 2 - Config, Schema, And Secrets

### T2.1 Move schema ownership to `pkg/config/schema`

- Stories: `2.1`, `2.2`
- Depends on: `T1.2`
- Outputs:
  - `pkg/config/schema`
  - no new work in `pkg/configschema`
- Implementation:
  - move schema generation and composition under `pkg/config/schema`
  - keep `pkg/config` focused on loading, merge, precedence, validation orchestration, and binding
  - retire `pkg/configschema` from new code paths
- Verification:
  - build gate: `pkg/config/...`
  - fast gate: schema-generation and config-loading tests
- Docs:
  - write `pkg/config/schema/README.md`
  - update config documentation where package references change
- Exit condition:
  - schema generation is a capability under config, not a side package

### T2.2 Feature-Owned Config Sections

- Stories: `2.1`, `2.2`
- Depends on: `T2.1`, `T1.2`
- Outputs:
  - core config plus feature-owned config sections
- Implementation:
  - remove the assumption of one monolithic framework config root
  - make each feature own:
    - config struct
    - validator
    - schema fragment
  - use config extensions as the primary composition mechanism
- Verification:
  - build gate: `pkg/config/...` and one migrated feature
  - fast gate: feature composition tests
- Docs:
  - update package READMEs for each feature migrated in this task
- Exit condition:
  - an application assembles only the config sections it includes

### T2.3 Sensitive Metadata And Redaction

- Stories: `2.3`, `2.4`, `7.7`
- Depends on: `T2.1`, `T2.2`
- Outputs:
  - field metadata for sensitivity
  - metadata-driven redaction
- Implementation:
  - define metadata fields for:
    - `classification`
    - `redaction`
    - `render_target`
    - `allowed_sources`
    - sanitized provenance
  - keep sensitivity separate from winning source
  - propagate metadata to:
    - schema
    - config render
    - config show
    - logging
    - panic and debug dumps
    - framework introspection
- Verification:
  - build gate: `pkg/config/...`
  - fast gate: redaction and mixed-source tests
  - contract gate: redaction never leaks raw secret values in framework-owned outputs
- Docs:
  - update config docs and package README sections for sensitivity behavior
- Exit condition:
  - redaction is field-metadata-driven everywhere the framework emits config-derived output

### T2.4 Config Commands And Secret-Provider Interface

- Stories: `2.2`, `2.3`
- Depends on: `T2.2`, `T2.3`, `T1.4`
- Outputs:
  - standardized `config render`
  - standardized `config validate`
  - future-ready secret-provider interface
- Implementation:
  - standardize app-level config generation and validation commands
  - preserve source precedence:
    - defaults
    - config file
    - secrets file
    - env
    - flags
  - keep `secrets file` as the mandatory current implementation
  - add provider interfaces for future:
    - AWS Secrets Manager
    - AWS SSM Parameter Store
    - Azure Key Vault
    - Google Secret Manager
- Verification:
  - build gate: `pkg/config`, `pkg/cli`
  - fast gate: precedence tests and CLI command tests
- Docs:
  - update config docs
  - write README for any provider-interface package if introduced
- Exit condition:
  - services own config generation and validation behavior; external secret providers are designed in, not yet required

## Wave 3 - HTTP Family

### T3.1 Extract `pkg/http/router`

- Stories: `3.1`, `3.2`
- Depends on: `T1.2`, `T1.4`
- Outputs:
  - `pkg/http/router`
  - adapters under `nethttp`, `gin`, `gorilla`
- Implementation:
  - move the common router contract to `pkg/http/router`
  - migrate the three router adapters
  - keep adapter-specific context types out of the common application path
  - preserve and move the router contract suite
- Verification:
  - build gate: `pkg/http/router/...`
  - fast gate: router unit tests
  - contract gate: router contract suite, including middleware ordering and request-cancellation semantics
- Docs:
  - write `pkg/http/router/README.md`
  - write adapter READMEs for `pkg/http/router/nethttp`, `pkg/http/router/gin`, `pkg/http/router/gorilla`
- Exit condition:
  - swapping router implementation changes only construction and wiring

### T3.2 Extract `pkg/http/server`, `response`, and `input`

- Stories: `3.1`, `3.3`, `4.6`
- Depends on: `T3.1`, `T1.3`
- Outputs:
  - `pkg/http/server`
  - `pkg/http/response`
  - `pkg/http/input`
  - dismantled `pkg/controller`
- Implementation:
  - move HTTP bootstrap and graceful shutdown to `pkg/http/server`
  - move response helpers and HTTP error mapping to `pkg/http/response`
  - move request input validation helpers to `pkg/http/input`
  - keep `pkg/core/errors` transport-neutral
  - leave no new work under `pkg/controller`
- Verification:
  - build gate: `pkg/http/...`
  - fast gate: request/response helper tests
  - contract gate: HTTP apps still build through the common runtime model
- Docs:
  - write `pkg/http/server/README.md`
  - write `pkg/http/response/README.md`
  - write `pkg/http/input/README.md`
- Exit condition:
  - HTTP bootstrap and controller concerns live in `pkg/http`, not in legacy generic packages

### T3.3 Move HTTP capabilities under `pkg/http`

- Stories: `3.3`, `3.4`, `4.6`, `7.1`
- Depends on: `T3.2`
- Outputs:
  - `pkg/http/openapi`
  - `pkg/http/sse`
  - `pkg/http/ws`
  - HTTP middleware/security packages
- Implementation:
  - move OpenAPI serving and validation provider wiring under `pkg/http/openapi` or `pkg/http/contract/openapi`
  - move SSE and WebSocket support under `pkg/http/sse` and `pkg/http/ws`
  - move HTTP-only middleware under `pkg/http/*`
  - split `pkg/middleware/authz` into:
    - `pkg/http/authentication`
    - `pkg/http/authorization`
  - formalize HTTP validation pipeline:
    - bind/parse
    - contract validation
    - DTO/input validation
  - keep OpenAPI, JSON Schema, CUE, and custom implementations as providers, not family definition
- Verification:
  - build gate: `pkg/http/...`
  - fast gate: provider-attachment tests
  - contract gate: changing contract-validation provider does not change router/server contracts
- Docs:
  - write READMEs for every finalized HTTP package and subpackage touched here
- Exit condition:
  - HTTP is a complete first-class family that can be included or ignored as a unit

## Wave 3B - gRPC Family

### T3B.1 Create `pkg/grpc/server`, `interceptor`, `status`, `validation`, `metadata`

Status on this branch: implemented.

- Stories: `3B.1` to `3B.5`
- Depends on: `T1.2`, `T1.4`, `T0.1`
- Outputs:
  - first-class gRPC family root
  - core runtime contracts for gRPC
- Implementation:
  - introduce:
    - `pkg/grpc/server`
    - `pkg/grpc/interceptor`
    - `pkg/grpc/status`
    - `pkg/grpc/validation`
    - `pkg/grpc/metadata`
  - keep gRPC as a service transport, not an event-bus replacement
  - model unary and streaming separately
  - implement layered validation:
    - transport decode and metadata
    - contract validation
    - domain validation
  - keep contract validation provider-driven:
    - Protobuf descriptors
    - Buf-compatible providers
    - Protovalidate-style providers
    - custom validators
- Verification:
  - build gate: `pkg/grpc/...`
  - fast gate: package-local tests
  - contract gate:
    - unary interceptor suites
    - streaming interceptor suites
    - status mapping suites
    - validation-layer distinction suites
- Docs:
  - write README for each created `pkg/grpc/*` package
- Exit condition:
  - a gRPC-only service can run through `Run` without importing HTTP runtime packages

### T3B.2 Add gRPC health, reflection, auth, and stream semantics

Status on this branch: implemented.

- Stories: `3B.5` to `3B.7`
- Depends on: `T3B.1`, `T1.5`
- Outputs:
  - optional gRPC runtime capabilities
- Implementation:
  - add:
    - `pkg/grpc/health`
    - `pkg/grpc/reflection`
    - `pkg/grpc/auth`
    - `pkg/grpc/stream`
  - integrate startup, readiness, liveness, degraded mode, and graceful shutdown with the shared runtime model
  - keep reflection and gRPC health optional and independently controllable
  - keep mTLS, peer identity, and metadata credentials inside `pkg/grpc`
  - reuse shared:
    - policy
    - tenant context
    - audit context
    - resilience
    - observability
- Verification:
  - build gate: `pkg/grpc/...`
  - contract gate:
    - health enable/disable suites
    - reflection enable/disable suites
    - deadline and cancellation suites
    - tenant/audit propagation suites
    - metadata credential and mTLS suites
- Docs:
  - write README for every optional gRPC package
- Exit condition:
  - gRPC is fully modeled as an opt-in transport family with optional runtime capabilities

## Wave 4 - Cache, Session, Persistence, Eventing

### T4.1 Create `pkg/cache`, `pkg/session`, `pkg/http/cache`, `pkg/http/session`

- Stories: `5.4`, `5.5`
- Depends on: `T3.3`
- Outputs:
  - role-based cache and session contracts
- Implementation:
  - define cache model with:
    - `Store`
    - `Policy`
    - `Invalidator`
    - optional tag invalidation
    - optional stale-while-revalidate
  - define session role separately
  - move HTTP-specific integration under `pkg/http/cache` and `pkg/http/session`
  - split Redis and Memcached implementations by role
- Verification:
  - build gate: `pkg/cache/...`, `pkg/session/...`, `pkg/http/cache`, `pkg/http/session`
  - fast gate: cache/session unit tests
  - contract gate: cache and session backend suites
- Docs:
  - write READMEs for `pkg/cache`, `pkg/session`, `pkg/http/cache`, `pkg/http/session`, and backend packages touched
- Exit condition:
  - cache and session are no longer modeled through generic store packages

### T4.2 Create `pkg/persistence/relational`

- Stories: `5.1`, `5.2`
- Depends on: `T2.2`
- Outputs:
  - relational family root
  - dialect abstraction
  - migrated SQL repositories
- Implementation:
  - move SQL repository contracts into `pkg/persistence/relational`
  - add dialect abstraction so generic SQL code is not PostgreSQL-shaped
  - migrate adapters:
    - `pkg/persistence/relational/postgres`
    - `pkg/persistence/relational/mysql`
  - move migration engine to `pkg/persistence/relational/migrate`
- Verification:
  - build gate: relational packages compile
  - integration gate: Postgres and MySQL adapters
  - contract gate: relational repository and dialect behavior suites
- Docs:
  - write READMEs for relational root, adapter packages, and `migrate`
- Exit condition:
  - SQL portability belongs to the relational family, not a generic store root

### T4.3 Create remaining persistence families

- Stories: `5.1`, `5.3`
- Depends on: `T4.2`
- Outputs:
  - `pkg/persistence/document`
  - `pkg/persistence/keyvalue`
  - `pkg/persistence/search`
  - `pkg/persistence/object`
  - reserved roots for `widecolumn`, `graph`, `timeseries`
- Implementation:
  - move:
    - `pkg/repository/document` -> `pkg/persistence/document`
    - `pkg/store/mongodb` -> `pkg/persistence/document/mongodb`
    - `pkg/store/dynamodb` -> `pkg/persistence/keyvalue/dynamodb`
    - `pkg/store/opensearch` -> `pkg/persistence/search/opensearch`
    - `pkg/store/s3` -> `pkg/persistence/object/s3`
  - keep document-oriented bridges for DynamoDB explicit and optional
- Verification:
  - build gate: touched persistence families
  - integration gate: migrated adapters
  - contract gate: family-local suites where a common subset is promised
- Docs:
  - write READMEs for each new family root and each migrated adapter package
- Exit condition:
  - there is no new code that treats all data backends as one `store` family

### T4.4 Refactor `pkg/eventbus`

- Stories: `4.1`, `4.2`, `4.3`
- Depends on: `T2.2`
- Outputs:
  - event bus without central factories
  - stable event validation layering
- Implementation:
  - keep `pkg/eventbus` as durable messaging family
  - remove central factories
  - preserve layered event validation:
    - envelope
    - contract/schema
    - domain
  - keep provider choice independent from broker choice
- Verification:
  - build gate: `pkg/eventbus/...`
  - integration gate: broker adapters
  - contract gate: event validation sequencing and provider independence tests
- Docs:
  - update or write READMEs for `pkg/eventbus`, broker adapters, and contract/validation packages
- Exit condition:
  - event schema providers are independent from broker implementation

### T4.5 Refactor jobs, scheduler, coordination, pubsub

- Stories: `4.1`, `4.4`, `4.5`
- Depends on: `T4.4`
- Outputs:
  - clear separation between jobs, scheduler, coordination, and pubsub
- Implementation:
  - keep jobs distinct from event bus messages
  - remove jobs factories
  - move distributed lock implementations to `pkg/coordination`
  - make scheduler depend on `coordination.LockProvider`
  - introduce `pkg/pubsub` for ephemeral fan-out
- Verification:
  - build gate: `pkg/jobs`, `pkg/scheduler`, `pkg/coordination`, `pkg/pubsub`
  - integration gate: lock-provider adapters and backend integrations
  - contract gate: scheduler multi-instance and lock-provider suites
- Docs:
  - write READMEs for `pkg/jobs`, `pkg/scheduler`, `pkg/coordination`, `pkg/pubsub`, and touched adapters
- Exit condition:
  - scheduler multi-instance safety survives the move to coordination

## Wave 5 - Reliability, Security, Operations, Verification

### T5.1 Create `pkg/reliability/idempotency`, `inbox`, `outbox`, and `dedup`

- Stories: `8.1`, `8.2`, `8.3`
- Depends on: `T4.4`, `T4.5`
- Outputs:
  - base reliability contracts for duplicate control and durable handoff
- Implementation:
  - add `pkg/reliability` root and the first subpackages for:
    - idempotency
    - inbox
    - outbox
    - dedup
  - define explicit at-least-once semantics where these contracts are reused
  - document ownership boundaries between reliability contracts and eventing / jobs families
- Verification:
  - build gate: `pkg/reliability/...`
  - fast gate: idempotency and dedup contract tests
  - contract gate: inbox/outbox contract suites
- Docs:
  - write READMEs for `pkg/reliability` and created subpackages
- Exit condition:
  - duplicate control and durable handoff are reusable shared contracts with tests

### T5.2 Create `pkg/reliability/retry` and poison-handling semantics

- Stories: `8.3`, `8.4`
- Depends on: `T5.1`
- Outputs:
  - retry budget and quarantine contracts
- Implementation:
  - add `pkg/reliability/retry`
  - define retry budgets, failure classification, poison handling, dead-letter, and quarantine behavior
  - connect these semantics to jobs and eventing without making backend choice part of the contract
- Verification:
  - build gate: `pkg/reliability/...`
  - fast gate: retry classification tests
  - contract gate: retry-budget and poison-handling suites
- Docs:
  - update reliability READMEs with retry and failure-taxonomy semantics
- Exit condition:
  - retry and poison handling are explicit, reusable, and verified

### T5.3 Create `pkg/reliability/saga` and workflow-level consistency contracts

- Stories: `8.5`
- Depends on: `T5.1`, `T5.2`
- Outputs:
  - saga and process-manager contracts
- Implementation:
  - add `pkg/reliability/saga`
  - define saga lifecycle contracts, compensating-action hooks, and workflow-level status semantics
  - keep persistence and transport bindings outside the core saga contracts
- Verification:
  - build gate: `pkg/reliability/...`
  - fast gate: saga lifecycle tests
  - contract gate: compensation-ordering and failure-path suites
- Docs:
  - write `pkg/reliability/saga/README.md`
- Exit condition:
  - workflow-level consistency contracts exist without coupling to one backend or transport

### T5.4 Create `pkg/policy`, `pkg/tenant`, and authorization contracts

- Stories: `7.3`, `7.4`, `7.5`
- Depends on: `T5.1`
- Outputs:
  - reusable authz and tenant contracts
- Implementation:
  - add shared authorization provider contracts supporting:
    - RBAC
    - ABAC
    - policy-engine-backed decisions
  - add tenant identity and runtime-context contracts
  - define transport-reusable policy evaluation boundaries
- Verification:
  - build gate: `pkg/policy`, `pkg/tenant`
  - fast gate: policy and tenant-context tests
  - contract gate: authorization provider suites
- Docs:
  - write READMEs for all created packages
- Exit condition:
  - authorization and tenant identity are reusable shared contracts rather than transport-local glue

### T5.5 Create `pkg/audit` and shared masking / PII reuse

- Stories: `7.6`, `7.7`, `7.8`, `10.1`, `10.2`, `10.3`
- Depends on: `T5.4`, `T2.3`
- Outputs:
  - audit sink and record contracts
  - shared masking and PII classification reuse
- Implementation:
  - add audit sink and record contracts with tamper-aware minimum semantics:
    - append-only streams
    - sequencing
    - digest chaining
    - verifier exports
    - required vs best-effort policy
  - add shared masking and PII classification reuse for framework-owned surfaces
- Verification:
  - build gate: `pkg/audit`, touched masking packages
  - fast gate: masking tests
  - contract gate: audit record and verifier behavior tests
- Docs:
  - write READMEs for all created packages
- Exit condition:
  - audit and masking semantics are reusable framework contracts, not just documentation

### T5.6 Create `pkg/featureflag` and explicit runtime posture contracts

Status on this branch: implemented.

- Stories: `9.1`, `9.2`, `9.3`
- Depends on: `T1.5`, `T5.4`, `T5.5`
- Outputs:
  - feature flags
  - explicit startup, readiness, liveness, and degraded-mode contracts
- Implementation:
  - add `pkg/featureflag`
  - formalize:
    - startup
    - readiness
    - liveness
    - degraded mode
  - define health and management attachment points for those states
- Verification:
  - build gate: touched runtime packages
  - fast gate: state-transition tests
  - contract gate: readiness / liveness contract suites
- Docs:
  - write `pkg/featureflag/README.md`
  - update relevant package READMEs for runtime posture semantics
- Exit condition:
  - runtime posture is explicit and testable across framework families

### T5.7 Add failure-injection, DR metadata, and operations hooks

Status on this branch: implemented.

- Stories: `9.4`, `9.5`, `11.1`, `11.2`, `11.3`
- Depends on: `T5.6`
- Outputs:
  - failure-injection hooks
  - DR / multi-region posture metadata
  - golden-signal and capacity attachment points
- Implementation:
  - add failure-injection hooks and target surfaces
  - define multi-region and DR posture metadata and validation rules
  - define capacity and golden-signal attachment points
  - keep the operational contracts transport-agnostic where possible
- Verification:
  - build gate: touched runtime packages
  - fast gate: hook registration tests
  - non-functional gate: failure-injection and degraded-mode suites
- Docs:
  - update relevant package READMEs and [docs/refactoring-operational-specs.md](./docs/refactoring-operational-specs.md)
- Exit condition:
  - operations posture is explicit, testable, and documented where the framework makes promises

### T5.8 Non-Functional Verification Harnesses

Status on this branch: implemented at the shared harness / lane level.

- Stories: `12.1` to `12.6`
- Depends on: `T5.2`, `T5.3`, `T5.7`
- Outputs:
  - standard entry points and harnesses for non-functional testing
- Implementation:
  - add standard hooks and documentation for:
    - performance tests
    - load tests
    - soak tests
    - resilience tests
    - security tests
    - compatibility and migration tests
    - race and ordering suites
  - keep long-running suites off the default fast path but give them stable commands and environments
- Verification:
  - non-functional gate: harnesses runnable and documented
- Docs:
  - update [docs/testing.md](./docs/testing.md) and [docs/test-infrastructure.md](./docs/test-infrastructure.md)
  - add README sections for any new test harness packages
- Exit condition:
  - promised runtime qualities have standard verification entry points

## Wave 6 - Descriptor, nimbctl, Shared Packages, Cleanup

### T6.1 Implement the service descriptor in Nimburion

Status on this branch: implemented.

- Stories: `3B.7`, `2.3`, `9.2`
- Depends on: `T1.4`, `T2.4`, `T3.3`, `T3B.2`
- Outputs:
  - service descriptor command and package
- Implementation:
  - implement the descriptor contract documented in [docs/nimbctl-service-descriptor-design.md](./docs/nimbctl-service-descriptor-design.md)
  - expose the descriptor through a standard CLI command
  - include:
    - application identity
    - compatibility
    - runtime
    - commands
    - config
    - dependencies
    - management
    - transports
    - deployment
    - migrations
    - features
    - artifacts
- Verification:
  - build gate: descriptor package and CLI integration
  - fast gate: descriptor-shape tests
  - contract gate: HTTP-only, gRPC-only, and mixed-app descriptor tests
- Docs:
  - write README for the package that owns descriptor generation
- Exit condition:
  - the framework can describe itself without AST or source-tree heuristics

### T6.2 Align `nimbctl` descriptor ingestion and command discovery

- Stories: cross-repo; use the `nimbctl` docs as the contract source
- Depends on: `T6.1`
- Outputs:
  - `nimbctl` uses descriptor-first discovery and command selection
- Implementation:
  - remove assumptions tied to:
    - `serve` as default
    - AST parsing
    - `go.mod` or `cmd/main.go` discovery
    - JSON Schema as primary config-generation driver
  - make `nimbctl` understand:
    - `run`
    - config render / validate
    - descriptor-provided command inventory
- Verification:
  - build gate: in `nimbctl` repo
  - contract gate: descriptor compatibility and command discovery behavior
- Docs:
  - update `nimbctl` docs in its repo
- Exit condition:
  - `nimbctl` discovery depends on published contracts, not internal package layout

### T6.3 Align `nimbctl` management and transport orchestration

- Stories: cross-repo; use the `nimbctl` docs as the contract source
- Depends on: `T6.2`
- Outputs:
  - `nimbctl` understands transport-family metadata and management over HTTP or gRPC
- Implementation:
  - make `nimbctl` understand:
    - management over HTTP or gRPC
    - transport-family metadata
    - optional gRPC artifacts
    - reflection and health-service support
    - mixed HTTP + gRPC applications without assuming both are always present
- Verification:
  - build gate: in `nimbctl` repo
  - contract gate: mixed-app orchestration behavior and management-surface compatibility
- Docs:
  - update `nimbctl` docs in its repo
- Exit condition:
  - `nimbctl` can orchestrate mixed-transport applications from published metadata alone

### T6.4 Normalize shared public packages

Status on this branch: implemented, including the `pkg/security` -> `internal/safepath` move.

- Stories: `7.2`, `10.1`
- Depends on: `T5.5`
- Outputs:
  - cleaned shared package ownership
- Implementation:
  - keep these public where the names still fit:
    - `pkg/auth`
    - `pkg/email`
    - `pkg/i18n`
    - `pkg/observability`
    - `pkg/resilience`
    - `pkg/version`
  - modularize concrete email providers into subpackages where appropriate
  - move filepath safety logic into `internal/safepath`
- Verification:
  - build gate: touched shared packages
  - fast gate: package-local tests
- Docs:
  - update or add READMEs for all touched shared packages and new subpackages
  - write `internal/safepath/README.md`
- Exit condition:
  - shared packages have clear, stable ownership and no misplaced transport-specific logic

### T6.5 Remove legacy roots and finalize documentation

- Stories: cleanup-linked stories across all milestones
- Depends on: `T4.5`, `T6.1`, `T6.4`
- Outputs:
  - removed legacy package roots
  - final package documentation
  - migration guide
- Implementation:
 - delete obsolete roots and factory packages:
    - `pkg/store`
    - `pkg/configschema`
    - `pkg/controller`
    - `pkg/security`
    - `pkg/migrate`
    - old factory packages
  - update package docs and top-level docs to the final taxonomy
  - write an explicit migration guide from old Nimburion layout to target architecture
  - perform a README sweep across all target public packages and subpackages under `pkg/*`
- Verification:
  - build gate: whole repo
  - fast gate: whole-repo fast path
  - integration gate: affected adapters and runtimes
  - contract gate: all family suites relevant to moved code
- Docs:
  - every target-state public package and subpackage under `pkg/*` must end with a `README.md`
- Exit condition:
  - no live code depends on legacy roots, and package documentation matches the final architecture

## Parallelism Rules

Parallel execution is allowed only in these cases:

- `T3.1`, `T3.2`, `T3.3` can overlap once `T1.4` and `T2.2` are stable
- `T3B.1` can run in parallel with the HTTP family tasks after `T1.4`
- `T4.2`, `T4.3`, `T4.4` can run in parallel once Wave 2 is stable
- `T5.2` and `T5.4` can overlap after `T5.1` if reliability interfaces are stable enough for shared security contracts to consume them
- `T5.6` can begin once `T1.5` and `T5.4` are stable even if `T5.5` is still finishing package-local details that do not change runtime posture contracts
- `T6.2` can begin as soon as `T6.1` emits stable descriptor payloads

Do not parallelize cleanup tasks that delete legacy roots until all consuming replacements are stable.

## Wave Exit Checklist

A wave is complete only when:

1. Every task in the wave has a merged implementation PR or an explicit defer note.
2. Every package created or finalized in that wave has its README.
3. Temporary shims introduced in that wave are either deleted or have an explicit same-wave deletion task.
4. The wave-local verification gates have stable commands recorded in PRs.
5. The affected docs listed in each task are updated.

## Final Completion Gate

The refactor is complete only when all of the following are true:

1. Every story in [docs/refactoring-user-stories.md](./docs/refactoring-user-stories.md) is implemented or explicitly deferred with a documented reason.
2. Every target public package and subpackage under `pkg/*` has a `README.md`.
3. No remaining framework code depends on:
   - `pkg/store`
   - `pkg/configschema`
   - `pkg/controller`
   - `pkg/security`
   - legacy factory packages
4. HTTP and gRPC are both first-class transport families.
5. The shared runtime model uses `Run`, feature contributions, shared health, and debug-gated introspection.
6. Config, schema, secrets, redaction, and descriptor generation follow the documented contracts.
7. Reliability, policy, tenant, audit, operations, and non-functional verification have real code and tests, not only docs.
8. `nimbctl` can consume the descriptor and standard CLI contracts without depending on internal Nimburion package layout.
9. The repository has stable verification entry points for build, fast, integration, contract, and non-functional lanes.
