# nimbctl Alignment Plan

> Current branch state: references to legacy roots such as `pkg/server`, `pkg/store`, or `pkg/configschema` describe coupling to remove, not active integration points that should still be used.
> Historical mapping: those references remain to explain why `nimbctl` must align to the target contracts instead of the old package layout.

This document defines how `nimbctl` should evolve to match the new Nimburion framework pattern.

It is intentionally cross-repo in scope:

- Nimburion remains the framework and runtime foundation.
- `nimbctl` remains a separate DX/orchestration tool.
- The integration between them must be based on explicit published contracts, not on internal package layout assumptions.

This document complements:

- [refactoring-requirements.md](./refactoring-requirements.md)
- [refactoring-design.md](./refactoring-design.md)
- [refactoring-plan.md](./refactoring-plan.md)
- [nimbctl-service-descriptor-design.md](./nimbctl-service-descriptor-design.md)

## Migration Strategy

This plan assumes a **clean break**.

- `nimbctl` does not need to preserve current contracts.
- New contracts may replace the current ones directly.
- Legacy compatibility paths are not required and should not drive the design.

## Main Decision

`nimbctl` should align with the refactor as an **external consumer of framework contracts**.

It should not be treated as:

- a package inside Nimburion
- a consumer of Nimburion internal packages
- a tool coupled to `pkg/server`, `pkg/store`, `pkg/configschema`, or the old monolithic config model

## Recommended Integration Model

The recommended integration model is:

1. `nimbctl` consumes a **machine-readable service/app descriptor**
2. `nimbctl` consumes **standard CLI contracts** exposed by applications built with Nimburion
3. `nimbctl` consumes **config generation/validation commands**, not JSON Schema as its primary integration mechanism
4. JSON Schema remains **optional**, useful for docs, editors, or external tooling, but not the primary driver of `nimbctl`

This is the preferred direction because it matches the new framework pattern:

- explicit app composition
- explicit features
- `Run` as the primary app entrypoint
- modular config ownership
- feature-driven commands

## Current Coupling To Remove

Today `nimbctl` is coupled to several legacy assumptions:

- it defaults service execution to `serve`
- it passes `--config-file` and `--secret-file` directly to service CLIs
- it discovers services by filesystem conventions such as `go.mod` and `cmd/main.go`
- it derives command policies by parsing source code for `BootstrapRun`
- it generates service config from JSON Schema, `schema.version`, and `x-nimbctl` metadata

These assumptions are acceptable as a transition baseline, but they are not the right long-term contract.
They should be removed rather than preserved.

## Target Contracts Between Nimburion And nimbctl

The target contracts should be explicit and versioned.

### 1. App Descriptor Contract

Each Nimburion-based application should expose a machine-readable descriptor.

The descriptor shape is defined in [nimbctl-service-descriptor-design.md](./nimbctl-service-descriptor-design.md).

Purpose:

- identify application kind
- expose compatibility policy
- describe supported commands
- describe config generation and validation capabilities
- describe runtime dependencies, management semantics, deployment capabilities, migration policy, transport-family metadata, and relevant runtime features

Possible delivery forms:

- preferred: CLI command such as `describe --format json`
- optional cached artifact: `nimburion.app.yaml` or `nimburion.app.json`

The exact file/command name can still be finalized, but the contract itself should exist and be mandatory for `nimbctl` integration.

The descriptor should include at least:

- descriptor version
- app kind
  - `gateway`
  - `service`
  - `consumer`
  - `producer`
  - `worker`
  - `scheduler`
- application version and tenancy mode
- compatibility policy for:
  - descriptor version
  - framework version
  - `nimbctl` supported range
- default executable command
- supported commands and policies
- config capabilities
  - generate/render support
  - validate support
  - secrets support
- config input model
  - regular inputs
  - sensitive inputs
  - environment profiles
- sensitivity model
  - field classification policy
  - redaction policy
  - sanitized provenance visibility
- runtime dependency declarations
- management endpoint semantics
- transport family declarations
  - included transport families
  - gRPC exposed services
  - gRPC reflection support
  - gRPC health-service support
  - gRPC transport-security mode
  - proto/package ownership metadata where useful
- deployment capabilities
- migration policy metadata
- feature stability metadata
- optional schema artifact location, if exported
- enabled feature families

### 2. CLI Contract

Applications generated with the new Nimburion pattern should expose stable CLI semantics.

Required direction:

- `run` is the default application command
- `serve` is optional and HTTP-specific
- `config validate` remains a standard capability
- feature commands are explicit and discoverable

### 3. Config Generation Contract

`nimbctl` should stop using JSON Schema as the primary source for generating service config.

Instead, applications should expose a standard config-generation capability, for example through a CLI command such as:

- `config render`
- or `config init`
- or another standardized command name chosen by the framework

That command should allow `nimbctl` to obtain:

- generated base config
- generated secrets file when applicable
- optionally minimal vs full/default-rich output

The service itself should remain the source of truth for:

- defaults
- feature-owned config sections
- validation
- redaction semantics
- sensitive input expectations
- environment profile semantics

### 4. Config Validation Contract

`nimbctl` should use the application CLI for config validation instead of reproducing validation logic externally.

Required behavior:

- service CLI validates its own config
- `nimbctl` orchestrates validation
- `nimbctl` does not need to understand framework-internal config structures

### 5. Command Metadata Contract

`nimbctl` should not depend long-term on AST parsing of `BootstrapRun`.

Command metadata should instead come from the app descriptor, including:

- command name
- run policy
- whether it is default
- whether it is HTTP-specific
- whether it is migration-related

### 6. Runtime Dependency And Deployment Metadata Contract

`nimbctl` should not infer orchestration posture from app kind alone.

The descriptor should publish:

- runtime dependencies with hard vs optional readiness semantics
- management endpoint availability and semantics
- transport-family metadata, including:
  - included transport families
  - gRPC exposed services
  - gRPC reflection support
  - gRPC health-service support
  - gRPC transport-security mode
  - proto/package ownership metadata where useful
- deployment capabilities such as:
  - `stateless`
  - `stateful`
  - `leader_lock_required`
  - `requires_durable_store`
- migration policy metadata
- tenant mode and environment profile semantics
- feature stability metadata

### 7. Optional Schema Contract

JSON Schema may still be published by applications, but as a secondary artifact.

Good uses:

- editor tooling
- documentation
- UI-driven configuration tools
- external integrations that explicitly want schema documents

Not the preferred primary use:

- `nimbctl` config generation

If schema artifacts are kept, their location should be declared by the descriptor or catalog rather than assumed from a hardcoded path.

## Development Plan For nimbctl

### Phase 0: Freeze External Contracts

Goal:

- make the Nimburion-to-`nimbctl` integration explicit before implementation diverges further

Tasks:

- define `nimbctl` as an external ecosystem tool in architecture docs
- define stable CLI and descriptor contracts
- define whether schema publication remains supported and at what level

Done:

- both repos refer to the same contract model
- no new work assumes `pkg/server`, `pkg/store`, or `pkg/configschema` as public integration points for `nimbctl`
- no compatibility work is planned around the old contracts

### Phase 1: Runner Realignment

Goal:

- make `nimbctl run/debug` use the new app CLI pattern only

Tasks:

- change the default service command from `serve` to `run`
- stop treating `serve` as the default execution path
- define the new standardized runtime invocation contract for app config and secrets inputs
- let `nimbctl` choose the default command from descriptor metadata

Done:

- new applications run correctly through `nimbctl` without custom flags
- `nimbctl` no longer assumes that an application is HTTP-first

### Phase 2: Introduce The App Descriptor

Goal:

- replace source-tree heuristics with a published machine-readable contract

Tasks:

- add a standard descriptor capability to Nimburion-generated applications
- update `nimbctl` discovery to read the descriptor first
- remove source-tree and AST heuristics as the primary integration path

Done:

- `nimbctl` no longer needs filesystem and AST heuristics
- application kind and command inventory come from the descriptor

### Phase 3: Enterprise Orchestration Metadata

Goal:

- make the descriptor strong enough for enterprise orchestration instead of only service discovery

Tasks:

- add compatibility metadata for:
  - application version
  - framework version
  - descriptor version
  - `nimbctl` supported range
- add runtime dependency declarations with hard vs optional readiness semantics
- add management endpoint metadata for:
  - liveness
  - readiness
  - health
  - metrics or introspection where present
- add transport-family metadata so `nimbctl` can reason about HTTP and gRPC runtimes without assuming they coexist
- detect gRPC feature presence from descriptor metadata instead of inferring it from command names alone
- surface gRPC reflection and health-service support in runtime inspection and orchestration flows
- understand management metadata over gRPC instead of assuming every management surface is HTTP path-based
- add deployment capabilities such as:
  - stateless vs stateful
  - horizontal scaling posture
  - leader-lock requirements
  - durable-store requirements
  - gRPC management exposure
  - gRPC public transport exposure
  - mTLS requirements where declared
- add migration policy metadata:
  - command
  - strategy
  - startup-blocking behavior
  - mixed-version safety
- add config input metadata that distinguishes regular config inputs from sensitive inputs
- add sensitivity-model metadata so `nimbctl` can understand classification and redaction policy without reading raw secret values
- add tenant mode and environment profile semantics
- add feature stability metadata
- allow optional transport-contract artifacts such as descriptor sets, Buf images, or generated contract manifests
- avoid assuming OpenAPI is the only transport-contract artifact that external tooling may consume
- update `nimbctl` validation to reject incompatible or orchestration-incomplete descriptors when the chosen workflow requires those fields

Done:

- `nimbctl` can determine compatibility before orchestration
- `nimbctl` can classify hard and optional runtime dependencies without probing app code
- `nimbctl` can understand management surfaces and deployment posture without route scraping
- `nimbctl` can understand gRPC management surfaces, reflection support, and health-service support without assuming HTTP is canonical
- `nimbctl` can discover gRPC runtime participation, exposed services, reflection, health-service support, and transport-security mode from the descriptor
- `nimbctl` can consume optional gRPC descriptor artifacts when a workflow requires them without making those artifacts mandatory for all services
- `nimbctl` can reason about migration policy, secret inputs, tenant mode, environment profiles, and feature stability from the descriptor
- `nimbctl` can understand sensitivity and sanitized provenance policy without becoming a carrier of secret values

### Phase 4: Replace JSON-Schema-Driven Config Generation

Goal:

- stop using JSON Schema as the primary driver for `nimbctl init`

Tasks:

- standardize a service CLI capability for config generation/rendering
- standardize a service CLI capability for config validation
- make `nimbctl init` call the service contract instead of interpreting schema defaults
- preserve support for split config/secrets generation

Done:

- `nimbctl` can generate application config without reading JSON Schema
- service config generation is owned by the service itself
- `nimbctl` preserves the distinction between regular config inputs and sensitive inputs

### Phase 5: Catalog Evolution

Goal:

- make the catalog reflect the new application model

Tasks:

- let catalog entries describe app kind and feature capabilities where useful
- keep schema location optional instead of mandatory for core `nimbctl` flows
- let the catalog point to descriptor-aware services

Done:

- catalog generation works for gateways, workers, consumers, and scheduler-oriented apps
- schema references are optional metadata, not the core dependency

### Phase 6: Template And Workspace Generation Update

Goal:

- make generated workspaces follow the new Nimburion pattern

Tasks:

- update `nimbctl init` and templates for:
  - gateway projects
  - generic HTTP services
  - consumers
  - workers
  - scheduler-oriented applications
- ensure generated services use the standard app CLI model
- ensure generated services publish the descriptor contract

Done:

- new workspaces created by `nimbctl` match the new framework topology and command model
- `nimbctl` depends only on published service contracts
- JSON Schema is optional, not central

## Explicit Recommendation On JSON Schema

If the goal is to reduce coupling and keep the framework pattern clean, the better approach is:

- **do not use JSON Schema as the primary integration contract with `nimbctl`**
- use:
  - a descriptor contract for metadata and capabilities
  - a service CLI contract for config generation and validation

JSON Schema can still exist, but only as:

- optional published artifact
- documentation aid
- editor/tooling integration

This keeps the service code as the source of truth and avoids making `nimbctl` responsible for reconstructing framework semantics from schema documents.

## Cross-Repo Acceptance Criteria

The Nimburion refactor and `nimbctl` evolution are aligned when all of the following are true:

- `nimbctl` can initialize and run applications built on the new framework pattern
- `nimbctl` defaults to `run`
- `nimbctl` can discover app kind and supported commands without AST parsing
- `nimbctl` can generate and validate config without JSON Schema as its primary dependency
- `nimbctl` can reason about dependency criticality, management semantics, deployment posture, and migration policy from the descriptor
- `nimbctl` can detect gRPC-only applications and reason about their transport and management metadata without requiring HTTP metadata
- `nimbctl` can distinguish config inputs from sensitive inputs
- `nimbctl` can understand the service sensitivity model and sanitized provenance policy without receiving raw secret values
- `nimbctl` can read tenant mode, environment profiles, and feature stability without source-tree heuristics
- `nimbctl` can consume optional gRPC descriptor artifacts or ignore them cleanly when the chosen workflow does not need them
- schema publication, if kept, is optional and secondary
- `nimbctl` no longer depends on Nimburion internal package layout
