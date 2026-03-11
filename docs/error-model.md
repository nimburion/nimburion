# Error Model

> Current repository state: this document captures the current error-handling model across the codebase and the target convergence model expected from framework-owned packages.
> Source of truth: findings here come from package inspection, review-driven cleanup, and transport-mapping behavior implemented in HTTP and gRPC layers.

This document maps the current error model package by package and defines the standard target for convergence.
It is intentionally pragmatic: the goal is not to redesign every package, but to make error semantics predictable across runtime, CLI, transport, and infrastructure boundaries.

Related documents:

- [architecture.md](./architecture.md)
- [operational-specs.md](./operational-specs.md)
- [testing.md](./testing.md)

## Working Assumptions

- Error handling is part of the public framework contract, not only an implementation detail.
- Wrapped errors must remain machine-interrogable with `errors.Is` and `errors.As`.
- Not every package needs a custom typed error, but every stable failure category needs one stable way to be recognized.
- Transport layers should map a canonical application error contract instead of inferring semantics from arbitrary strings.
- Existing exported sentinel errors may remain when they model a real package-level contract.

## Current Model Overview

The current codebase uses four patterns in parallel:

| Pattern | Current usage | Assessment |
| --- | --- | --- |
| Canonical typed application error | `pkg/core/errors.AppError` | Strong base model. |
| Transport mapping on canonical type | `pkg/http/response`, `pkg/grpc/status` | Strong and already useful. |
| Family-level sentinel sets | `pkg/jobs`, `pkg/scheduler`, `pkg/coordination`, package-local sentinels | Useful, but not yet federated. |
| Plain `errors.New` / `fmt.Errorf` wrapping | config, adapters, CLI, infrastructure packages | Readable, but semantically inconsistent. |

The result is not chaos, but it is not yet one coherent framework-wide model.

## Current Branch Progress

The current branch has already completed the first convergence steps:

- `E0` is complete:
  - [`pkg/core/errors`](../pkg/core/errors/errors.go) is the canonical root application error contract
  - [`pkg/i18n`](../pkg/i18n/contract.go) now keeps compatibility aliases instead of owning a competing root `AppError`
- `E1` is complete:
  - [`pkg/core/errors`](../pkg/core/errors/errors.go) exposes shared constructors and canonical codes for common categories
  - [`pkg/http/response`](../pkg/http/response/errors.go) already uses those constructors
- `E2` is materially complete for public family-owned validation surfaces:
  - jobs, session, HTTP family config, cache, email, eventbus, auth, observability, scheduler, persistence, SSE, and search config validators now return `*coreerrors.AppError` with stable validation codes
- `E3` is materially complete for the primary family and boundary surfaces:
  - [`pkg/core/errors`](../pkg/core/errors/errors.go) now canonicalizes family-level sentinels from jobs, scheduler, coordination, cache, session, and HTTP session middleware into `*coreerrors.AppError` through a shared registry
  - [`pkg/http/response`](../pkg/http/response/response.go) and [`pkg/grpc/status`](../pkg/grpc/status/status.go) now normalize those family-level sentinels through `pkg/core/errors` instead of only recognizing already-canonical errors
  - [`pkg/cli`](../pkg/cli/main.go) now canonicalizes jobs/scheduler healthcheck wiring failures and emits typed unavailable errors for failing health checks
  - [`pkg/jobs`](../pkg/jobs/errors.go) and [`pkg/scheduler`](../pkg/scheduler/errors.go) now raise canonical app errors at the family-helper layer while preserving `errors.Is(..., Err*)`
  - [`pkg/jobs/feature`](../pkg/jobs/feature/feature.go), [`pkg/scheduler`](../pkg/scheduler/feature.go), [`pkg/jobs/provider`](../pkg/jobs/provider.go), [`pkg/coordination/redis`](../pkg/coordination/redis/lock.go), and [`pkg/coordination/postgres`](../pkg/coordination/postgres/lock.go) now expose canonical app errors at their public entrypoints

What remains open after this progress is mostly:

- classifying infrastructure and adapter failures more consistently
- enforcing the new model in remaining non-family-specific call sites over time

## Package Map

### 1. Canonical Core Error Contract

Primary package:

- [`pkg/core/errors`](../pkg/core/errors/errors.go)

Current state:

- Defines `AppError` with:
  - stable `Code`
  - structured `Params`
  - structured `Details`
  - transport hint `HTTPStatus`
  - wrapped `Cause`
- Exposes `Unwrap()`, so `errors.As` and `errors.Is` work correctly through the cause chain.
- Is already consumed by transport adapters.

Assessment:

- This is the best candidate to be the one canonical framework error type.

### 2. Transport Boundaries Already Aligned

Primary packages:

- [`pkg/http/response`](../pkg/http/response/response.go)
- [`pkg/grpc/status`](../pkg/grpc/status/status.go)

Current state:

- HTTP maps `*coreerrors.AppError` to response shape, status code, code, message, and details.
- gRPC maps `*coreerrors.AppError` and validation-layer errors to gRPC status codes.

Assessment:

- This is a strength of the current branch.
- The transport side already expects one canonical application error model.
- The remaining convergence work is mostly on the producer side, not on the transport side.

### 3. i18n Compatibility Layer

Primary package:

- [`pkg/i18n`](../pkg/i18n/contract.go)

Current state:

- Keeps compatibility aliases for `AppMessage` and `AppError`.
- No longer owns an independent root error implementation.

Assessment:

- The previous duplication has been removed on this branch.
- The remaining task is deprecation follow-through, not semantic redesign.

Target direction:

- `pkg/core/errors.AppError` should be canonical.
- `pkg/i18n` should own translation and message contracts, not a competing error root type.

### 4. Family-Level Sentinel Taxonomies

Primary packages:

- [`pkg/jobs`](../pkg/jobs/errors.go)
- [`pkg/scheduler`](../pkg/scheduler/errors.go)
- [`pkg/coordination`](../pkg/coordination/errors.go)

Current state:

- These packages expose coherent family-level sentinels such as:
  - `ErrValidation`
  - `ErrConflict`
  - `ErrNotFound`
  - `ErrRetryable`
  - `ErrInvalidArgument`
  - `ErrNotInitialized`
  - `ErrClosed`
- They use helpers like `jobsError(...)` or `schedulerError(...)` to wrap a sentinel with a message.

Assessment:

- This is a good intermediate model.
- The problem is not the presence of these sentinels.
- The problem is that these families do not yet converge on a shared higher-level taxonomy in `pkg/core/errors`.

### 5. Package-Specific Sentinel Errors

Representative packages:

- [`pkg/cache`](../pkg/cache/cache.go)
- [`pkg/session`](../pkg/session/session.go)
- [`pkg/http/session`](../pkg/http/session/oauth_tokens.go)
- [`pkg/resilience`](../pkg/resilience/circuitbreaker.go)
- [`pkg/resilience`](../pkg/resilience/timeout.go)
- [`internal/safepath`](../internal/safepath/safepath.go)
- [`pkg/reliability/idempotency`](../pkg/reliability/idempotency/idempotency.go)

Current state:

- These packages define local sentinels for specific contract conditions:
  - cache miss
  - session not found
  - missing session middleware
  - circuit breaker open
  - timeout
  - invalid path / traversal
  - atomic execution required

Assessment:

- This is healthy when the condition is genuinely package-owned and stable.
- These sentinels should remain where they model a real package contract.
- They should not be replaced blindly by one giant global enum.

### 6. Config Validation Layers

Representative packages:

- [`pkg/config`](../pkg/config/loader.go)
- [`pkg/jobs/config`](../pkg/jobs/config/config.go)
- [`pkg/session/config`](../pkg/session/config/config.go)
- [`pkg/http/server/config`](../pkg/http/server/config/config.go)
- [`pkg/http/cors/config`](../pkg/http/cors/config/config.go)
- [`pkg/http/csrf/config`](../pkg/http/csrf/config/config.go)
- [`pkg/http/httpsignature/feature.go`](../pkg/http/httpsignature/feature.go)

Current state:

- Mostly return:
  - `errors.New("... is required")`
  - `fmt.Errorf("invalid ...: %s", value)`
- Messages are generally readable.
- Failure categories are not typed beyond string content.

Assessment:

- This is one of the weakest areas in the current model.
- Validation failures are stable, expected, and user-facing, so they are exactly the errors that should have the strongest structured contract.

### 7. Infrastructure / Adapter Layers

Representative packages:

- Redis adapters
- relational adapters
- MongoDB adapter
- DynamoDB adapter
- email providers
- eventbus adapters

Current state:

- The dominant pattern is:
  - `fmt.Errorf("operation: %w", err)`
- This preserves cause chains correctly in many places.
- Some package-specific conditions are mapped to sentinels, such as cache/session not-found behavior.

Assessment:

- Wrapping is often technically correct.
- Semantic classification is inconsistent.
- Stable conditions like `not found`, `closed`, `timeout`, `retryable`, or `invalid configuration` are not normalized across families.

### 8. CLI Layer

Primary package:

- [`pkg/cli`](../pkg/cli/main.go)

Current state:

- Aggregates errors from config, health checks, jobs, scheduler, and runtime wiring.
- Uses `fmt.Errorf("context: %w", err)` reasonably well.
- Still largely depends on underlying string-based validation and adapter errors.

Assessment:

- CLI is a consumer and aggregator of the inconsistency beneath it.
- It should not invent a separate error model.
- It should increasingly surface canonical typed errors coming from lower layers.

## Current Strengths

- One strong transport-neutral typed error already exists in [`pkg/core/errors`](../pkg/core/errors/errors.go).
- HTTP and gRPC transport mapping already use a structured model.
- Several families already expose stable sentinel semantics.
- Wrapping with `%w` is used frequently enough that convergence is realistic.
- Most public family-owned config validators now return canonical typed validation errors.

## Current Gaps

- No single shared taxonomy for common categories like validation, conflict, not found, timeout, closed, unavailable, retryable.
- Some config and validation surfaces outside the family-owned packages still return plain strings.
- Infrastructure adapters preserve causes but often do not classify stable conditions.
- Review and lint cleanup still uncover `errorlint` issues because the repo has not fully standardized on `errors.Is` / `errors.As`.

## Target Standard

### Canonical Rule

The framework should have exactly one canonical typed application error contract:

- [`pkg/core/errors.AppError`](../pkg/core/errors/errors.go)

This type should be the standard for:

- stable error codes
- transport mapping
- localized/user-facing message resolution
- structured details
- wrapped causes

### Stable Recognition Rules

Across the repo:

- use `errors.Is` for sentinel conditions
- use `errors.As` for typed errors
- use `fmt.Errorf(... %w ...)` for single-cause wrapping
- use `errors.Join(...)` when preserving multiple failures matters
- never rely on `==` or `!=` against errors that may be wrapped

### Shared Category Set

The target shared category set should live conceptually under `pkg/core/errors` and cover:

- validation
- invalid_argument
- not_found
- conflict
- unauthorized
- forbidden
- timeout
- retryable
- unavailable
- closed
- not_initialized
- internal

These should map cleanly to:

- HTTP status codes
- gRPC status codes
- CLI exit/reporting behavior
- logging and metrics dimensions

### Local Sentinel Rule

Package-local sentinels should remain allowed when they represent a real package contract.

Examples:

- `cache.ErrCacheMiss`
- `session.ErrNotFound`
- `jobs.ErrClosed`
- `coordination.ErrConflict`
- `resilience.ErrTimeout`

But they should fit into the shared category model instead of becoming isolated one-off semantics.

### Config Validation Rule

Validation errors should converge toward structured canonical application errors instead of raw strings.

Target behavior:

- validation failures return stable codes
- callers can still print readable messages
- transport and CLI layers can distinguish validation from internal failure without string matching

### Infrastructure Rule

Adapters do not need custom typed errors for every failure.

They should:

- preserve causes
- classify stable conditions
- map expected package-owned semantics to sentinels
- surface canonical typed errors only when crossing an application boundary or when the category matters semantically

## Convergence Plan

### Phase E0: Canonicalization

Goals:

- Declare [`pkg/core/errors`](../pkg/core/errors/errors.go) as the single canonical application error type.
- Stop adding new typed error roots elsewhere.

Tasks:

- deprecate the duplicate `AppError` contract in [`pkg/i18n`](../pkg/i18n/contract.go)
- update docs to point to `pkg/core/errors` as source of truth

Status:

- Complete on this branch.

### Phase E1: Shared Constructors

Goals:

- Make the canonical model easy to use consistently.

Tasks:

- add helper constructors in `pkg/core/errors` for common categories such as:
  - `NewValidation`
  - `NewInvalidArgument`
  - `NewNotFound`
  - `NewConflict`
  - `NewTimeout`
  - `NewInternal`
- keep code-first semantics and optional fallback messages/details

Status:

- Complete on this branch.

### Phase E2: Validation Layer Migration

Goals:

- Convert the highest-value user-facing string errors first.

Priority areas:

- `pkg/config`
- `pkg/jobs/config`
- `pkg/session/config`
- `pkg/http/*/config`
- feature validation packages

Expected outcome:

- invalid config becomes structurally recognizable everywhere

Status:

- Substantially complete for public family-owned validation surfaces on this branch.
- Remaining follow-up is limited to peripheral or non-family-specific validation paths.

### Phase E3: Family Bridge Layer

Goals:

- Preserve family sentinels while connecting them to the canonical model.

Priority areas:

- `pkg/jobs`
- `pkg/scheduler`
- `pkg/coordination`
- `pkg/cache`
- `pkg/session`

Expected outcome:

- family-level sentinel APIs remain stable
- boundary layers can still lift them into canonical app errors when needed

### Phase E4: Infrastructure Classification

Goals:

- Normalize recurring backend conditions.

Priority areas:

- relational adapters
- Redis adapters
- MongoDB / DynamoDB adapters
- eventbus adapters
- email providers

Expected outcome:

- `not found`, `closed`, `timeout`, `retryable`, and similar conditions become consistently recognizable

### Phase E5: Enforcement

Goals:

- Keep the model coherent after migration starts.

Tasks:

- keep `errorlint` enabled
- prefer review guidance that rejects string-based semantic checks
- add focused tests for `errors.Is` / `errors.As` on exported package contracts

## Decision Rules For New Code

When adding or changing code:

1. If the error is only contextual decoration around another failure, wrap with `%w`.
2. If the error is a stable package contract, expose or reuse a sentinel.
3. If the error crosses application or transport boundaries, prefer `*coreerrors.AppError`.
4. If the error is user-facing validation, do not rely on plain text only.
5. If two failures matter, preserve both with `errors.Join`.

## Done Looks Like

The error model can be considered converged when:

- `pkg/core/errors.AppError` is the only framework-wide typed application error root
- transport layers map canonical errors without special-case duplication
- config and validation layers return structured categories instead of only strings
- exported package contracts are testable with `errors.Is` / `errors.As`
- local sentinels remain meaningful but no longer form isolated taxonomies
- repo-wide error handling no longer depends on textual comparisons for semantic decisions
