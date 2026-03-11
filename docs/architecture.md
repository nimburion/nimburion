# Architecture

This document describes the stable architecture of Nimburion as it exists today and the guardrails that should continue to shape new framework code.

Historical refactor planning material lives under [archive/](./archive/README.md).

Related documents:

- [operational-specs.md](./operational-specs.md)
- [testing.md](./testing.md)
- [configuration.md](./configuration.md)
- [error-model.md](./error-model.md)

## Design Goals

- keep runtime composition explicit and compile-time
- keep `pkg/core` transport-agnostic
- expose replaceable adapters behind family-level contracts
- keep operational behavior observable and testable
- treat security, resilience, and lifecycle as framework contracts, not optional add-ons

## Non-Goals

- no runtime plugin loader
- no long-lived return to removed legacy roots such as `pkg/store` or `pkg/controller`
- no single generic storage abstraction that erases differences between relational, document, key-value, search, and object systems

## Package Topology

### Core

- `pkg/core/app` owns application lifecycle and runtime orchestration
- `pkg/core/errors` owns the canonical application error contract
- `pkg/featureflag`, `pkg/health`, and `pkg/resilience` provide shared runtime capabilities

### Configuration

- `pkg/config` owns loading, precedence, and repository-wide configuration assembly
- `pkg/config/schema` owns schema-oriented helpers and config metadata
- feature or family packages own validation for their own config surfaces

### Transport Families

- `pkg/http/*` owns HTTP server bootstrap, middleware, request/response handling, OpenAPI, SSE, and related transport concerns
- `pkg/grpc/*` owns gRPC server lifecycle, interceptors, validation, health, reflection, and status mapping
- transport families may depend on shared contracts, but must not redefine them

### Persistence And Operational Roles

- `pkg/persistence/*` is split by persistence family: relational, document, key-value, object, and search
- `pkg/cache/*`, `pkg/session/*`, and `pkg/coordination/*` are role-oriented packages, even when they reuse the same vendor underneath
- adapters stay close to the family or role that owns their contract

### Async And Reliability

- `pkg/eventbus` owns event transport concerns
- `pkg/jobs` owns background job runtime and worker behavior
- `pkg/scheduler` owns scheduled execution and lock-aware coordination
- `pkg/reliability/*` owns inbox, outbox, retry, idempotency, deduplication, and saga-style helpers

### Shared Platform Capabilities

- `pkg/auth`, `pkg/policy`, `pkg/tenant`, and `pkg/audit` own shared security and governance contracts
- `pkg/observability` owns logging, metrics, tracing, and related runtime signals
- `pkg/email` and other shared families expose reusable operational integrations

## Runtime Model

- applications start through explicit runtime composition, not hidden registration
- the primary lifecycle model is `Prepare -> Run -> Shutdown`
- HTTP bootstrap lives under `pkg/http/server`, including public and management servers with graceful shutdown
- CLI composition stays under `pkg/cli`, but the runtime core stays in `pkg/core`
- features contribute capabilities through explicit wiring instead of package-global side effects

## Configuration And Secrets

- configuration precedence is `ENV > secrets file > config file > defaults`
- secrets file support is first-class, but not the only allowed future secret source
- feature-owned validation must fail early and with typed errors
- redaction and sensitive-field handling are part of the framework contract, not only application policy

## Transport Rules

- HTTP and gRPC are distinct first-class families
- HTTP-specific code belongs under `pkg/http/*`
- gRPC-specific code belongs under `pkg/grpc/*`
- `pkg/core` must not import transport families
- transport layers map canonical application errors instead of inferring semantics from arbitrary strings

## Persistence And Role Rules

- choose packages by responsibility first, vendor second
- do not reintroduce shared vendor roots for unrelated roles
- family-local contract tests are preferred over generic package-bucket verification
- lifecycle semantics such as timeouts, shutdown, and closed-state handling must be explicit and testable

## Operational And Security Guardrails

- management and health endpoints are part of the runtime contract
- graceful shutdown must preserve caller deadlines and cancellation budgets
- audit, tenant, policy, and auth concerns stay in shared packages instead of being redefined per transport
- release, supply-chain, and disaster-recovery expectations are documented in [operational-specs.md](./operational-specs.md)

## Guardrails For New Code

- do not create new framework code under removed or superseded legacy roots
- place new code in the current target family directly
- prefer family-scoped contracts and tests over repo-wide generic abstractions
- when in doubt, keep lifecycle, observability, error semantics, and config ownership explicit
