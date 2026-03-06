# `pkg/cli`

## Purpose

`pkg/cli` provides the shared application CLI used by Nimburion-based services and workers. During the refactor it is moving from an HTTP-service-oriented bootstrap to an application-oriented CLI where `run` is the primary entrypoint and transport-specific commands such as `serve` are feature concerns.

## Owned Contracts

The package owns:

- `AppCommandOptions` as the target-state application CLI assembly contract
- `NewAppCommand` for application-oriented command trees
- base commands such as `version`, `config`, and `healthcheck`
- framework-managed jobs and scheduler command wiring until those areas become fully feature-driven

## Composition And Wiring Expectations

- New CLI assembly should prefer `AppCommandOptions` and `NewAppCommand`.
- `run` is the primary application command; `serve` remains an alias for HTTP-oriented compatibility.
- Feature command and config contributions flow through `pkg/core/feature` and are merged into the CLI builder when they can be expressed as command or config descriptors.
- Custom application commands may still be added directly while the refactor is in transition.

## Non-Goals

- transport runtime ownership
- application lifecycle orchestration
- direct ownership of HTTP bootstrap semantics
- permanent retention of service-first command naming

## Validation And Runtime Semantics

- config loading remains centralized through the existing config provider path
- healthcheck runs framework dependency health checks plus optional application checks
- scheduler and jobs commands keep explicit signal handling and graceful shutdown behavior

## Testing Expectations

- fast tests should cover command registration, policy annotations, `run`/`serve` behavior, and feature-contributed commands
- later waves should add integration coverage once feature-driven command registration fully replaces hardcoded optional command wiring

## Status

Target-state package in Wave 1. `AppCommandOptions` and `NewAppCommand` are the supported CLI assembly contracts for the refactored runtime.
