# `pkg/core/feature`

## Purpose

`pkg/core/feature` defines the feature contribution contracts consumed by the core runtime. It is the target-state seam between `pkg/core/app` and transport or capability families such as future HTTP, gRPC, jobs, scheduler, or migration features.

## Owned Contracts

The package owns:

- the `Feature` interface
- grouped `Contributions`
- lifecycle `Hook` and runtime `Runner` contracts
- feature-owned config and command contribution descriptors
- the minimal `Runtime` interface visible to feature implementations

## Composition And Wiring Expectations

- `pkg/core/app` collects `Feature` values during app construction and flattens their contributions into lifecycle phases.
- Feature implementations should contribute behavior through `Contributions()` instead of mutating global registries directly during package init.
- The `Runtime` contract intentionally exposes shared registries and service registration without leaking transport-specific bootstrap details.
- CLI command wiring and config composition can consume `CommandContribution` and `ConfigExtension` in later waves without changing feature registration shape.

## Non-Goals

- concrete HTTP, gRPC, jobs, or persistence features
- Cobra-specific command APIs
- config loading or schema generation infrastructure
- transport bootstrap ownership

## Runtime And Validation Semantics

- startup hooks run in registration order
- shutdown hooks run in reverse registration order through `pkg/core/app`
- health, instrumentation, and introspection contributions register against shared runtime-owned registries
- command and config contributions are descriptors only until later CLI and config waves consume them directly

## Testing Expectations

- fast tests should verify that one feature can contribute startup, health, service, runtime, shutdown, and introspection behavior without editing the base runtime
- later waves should add integration coverage once CLI and transport families consume command and config contributions

## Status

Target-state package introduced by Wave 1 Task `T1.2`.
