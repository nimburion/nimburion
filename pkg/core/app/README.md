# `pkg/core/app`

## Purpose

`pkg/core/app` owns the transport-agnostic application lifecycle for the refactor target runtime. It gives the framework one place for startup sequencing, shared runtime state, runtime execution, and graceful shutdown without depending on `pkg/server` or any HTTP bootstrap package.

## Owned Contracts

The package owns:

- the `App` runtime orchestrator
- ordered lifecycle phases for config resolution, observability baseline, feature registration, health and introspection registration, service construction, runtime execution, and graceful shutdown
- reusable startup preparation via `Prepare` for health and introspection flows that should not start runtime runners
- shared runtime state exposed through `Runtime`
- named lifecycle hooks and runtime runners
- the framework-owned `IntrospectionRegistry`

## Composition And Wiring Expectations

- Application wiring creates one `App` with `Options`, then calls `Run`.
- Call `Prepare` when a caller needs startup-owned registries and services without starting long-running workloads.
- Shared registries such as health, metrics, tracing, and introspection may be injected or allowed to default.
- Health aggregation should happen through the shared `HealthRegistry` owned by the app runtime.
- Transport families such as future `pkg/http/*` and `pkg/grpc/*` should plug in through lifecycle hooks and runners instead of owning the top-level runtime.
- Feature-owned services may be stored in `Runtime.Services` until `pkg/core/feature` defines the more explicit contribution contracts.

## Non-Goals

- HTTP or gRPC bootstrap
- router, broker, database, or mail provider selection
- feature discovery or registration policy beyond ordered hook execution
- user-facing CLI command assembly

## Lifecycle Semantics

- Startup phases run sequentially in target-state order.
- Runtime runners start after service construction and share a cancellable runtime context.
- The first runner error cancels peer runners.
- Graceful shutdown runs hooks in reverse registration order and also shuts down the injected tracer provider when present.
- Startup phase failures stop the lifecycle before runtime execution begins.
- Introspection contributors run only when `Options.Debug` is enabled.

## Testing Expectations

- fast tests must cover startup ordering, cancellation behavior, and graceful shutdown ordering
- later waves should add feature registration and CLI integration coverage once `pkg/core/feature` and the CLI refactor land

## Status

Target-state package introduced by Wave 1 Task `T1.1`.
