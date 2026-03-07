# `pkg/health`

## Purpose

`pkg/health` owns the shared health contracts and registry used across runtime families. During the refactor it is the canonical health aggregation model for both CLI health checks and transport management surfaces.

## Owned Contracts

The package owns:

- `Registry` for health-check registration and aggregation
- `Checker`, `CheckResult`, and `AggregatedResult`
- built-in checker adapters and convenience constructors
- health status values `healthy`, `degraded`, and `unhealthy`

## Composition And Wiring Expectations

- `pkg/core/app` should own one shared `Registry` per application runtime.
- Features and runtime families register checks into that shared registry.
- CLI `healthcheck` and management/readiness surfaces should evaluate the same registry model instead of maintaining independent aggregation logic.
- Runtime posture checkers from `pkg/featureflag` attach to the same registry so startup, readiness, liveness, and degraded mode stay explicit.
- Package-local helpers may construct checkers, but they should not redefine health result shapes.

## Non-Goals

- liveness/readiness HTTP endpoint ownership
- debug or introspection ownership
- transport-specific status-code mapping
- adapter construction or dependency bootstrapping

## Validation And Runtime Semantics

- checks run concurrently when the registry is evaluated
- any unhealthy check makes the aggregate result unhealthy
- degraded results are preserved unless an unhealthy result takes precedence
- readiness-style consumers should treat degraded as explicit degraded service, not as identical to unhealthy
- context cancellation and timeouts flow into each checker

## Testing Expectations

- fast tests cover registration, aggregation, timeout behavior, and checker adapters
- runtime families that promise readiness behavior should add targeted tests against shared registry usage

## Status

Active shared package. In Wave 1 Task `T1.5`, the refactor treats this registry as the single health aggregation surface for both CLI and management flows.
