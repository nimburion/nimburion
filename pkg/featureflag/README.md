# `pkg/featureflag`

## Purpose

`pkg/featureflag` owns rollout-safe feature gating and explicit runtime posture contracts.

## Owned Contracts

- typed feature flag definitions and evaluation
- safe-default evaluation semantics when providers are unavailable
- runtime posture for startup, readiness, liveness, and degraded mode
- health-registry attachment points for runtime posture

## Composition And Wiring

- applications and features can register typed flag definitions in one shared registry
- provider lookup remains optional; default values stay authoritative when a provider is missing or failing
- `pkg/core/app` owns the shared runtime instances and wires posture health checks into `pkg/health`
- management surfaces and CLI health flows consume the same posture-backed health attachment points

## Non-Goals

- deployment-system progressive delivery
- vendor-specific flag providers
- region and disaster-recovery posture metadata owned by later operational tasks

## Status

Target-state package introduced by Wave 5 Task `T5.6`.


## Breaking Change Note

`RegisterBool`, `RegisterString`, and `RegisterInt` return `error`.
Because Go allows ignored return values, existing call sites that do not check
these errors may silently skip failed registrations.

### Migration

- Prefer explicit error handling when registering flags.
- Use `MustRegisterBool`, `MustRegisterString`, and `MustRegisterInt` only
  during bootstrap/startup paths where registration failures are programmer
  errors and should panic fast.
