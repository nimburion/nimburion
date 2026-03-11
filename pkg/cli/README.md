# `pkg/cli`

## Purpose

`pkg/cli` provides the shared application CLI used by Nimburion-based services and workers. The target-state CLI is application-oriented: `run` is the primary entrypoint and transport-specific commands are contributed explicitly by the relevant family.

## Owned Contracts

The package owns:

- `AppCommandOptions` as the target-state application CLI assembly contract
- `NewAppCommand` for application-oriented command trees
- base commands such as `version`, `config`, and `healthcheck`
- the debug-gated `introspect` command for framework-owned runtime data

## Composition And Wiring Expectations

- New CLI assembly should prefer `AppCommandOptions` and `NewAppCommand`.
- `run` is the primary application command.
- Feature command and config contributions flow through `pkg/core/feature` and are merged into the CLI builder when they can be expressed as command or config descriptors.
- `healthcheck` should use the shared runtime health registry instead of a separate ad hoc aggregation path.
- Custom application commands may be added directly when they do not fit a feature contribution yet.

## Non-Goals

- transport runtime ownership
- application lifecycle orchestration
- direct ownership of HTTP bootstrap semantics
- permanent retention of service-first naming conventions

## Validation And Runtime Semantics

- config loading remains centralized through the existing config provider path
- healthcheck builds the shared runtime, registers framework and optional application checks in the shared health registry, and evaluates that registry once
- `introspect` prints framework introspection data only when `--debug` is enabled
- family-contributed commands such as `cache`, `migrate`, `openapi`, `jobs`, and `scheduler` are materialized only when their owning features are included

## Testing Expectations

- fast tests should cover command registration, policy annotations, `run` behavior, and feature-contributed commands

## Status

Target-state package in Wave 1. `AppCommandOptions` and `NewAppCommand` are the supported CLI assembly contracts for the refactored runtime.
