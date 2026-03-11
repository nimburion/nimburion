# `pkg/http/server`

## Purpose

`pkg/http/server` owns HTTP server bootstrap, public and management server wiring, TLS loading, and graceful startup/shutdown for the HTTP family.

## Owned Contracts

The package owns:

- `Server` and `Config`
- `PublicAPIServer`
- `ManagementServer`
- `BuildHTTPServers`
- `RunHTTPServers`
- TLS loading helpers used by the HTTP runtime

## Composition And Wiring Expectations

- HTTP applications should compose public and management servers through this package.
- The package depends on `pkg/http/router` for routing and on `pkg/core/app` for lifecycle orchestration.
- Public and management routers are injected explicitly; this package no longer selects router implementations from config.
- Management health and metrics surfaces should use the shared registries passed into `RunHTTPServersOptions`.
- Management `/health` and `/ready` surfaces consume the posture-backed health registry emitted by `pkg/core/app`.
- Transport-neutral lifecycle ownership remains in `pkg/core/app`; this package contributes HTTP runtime behavior.

## Non-Goals

- transport-neutral application lifecycle ownership
- HTTP response error mapping ownership
- DTO/input validation ownership
- OpenAPI capability ownership beyond server-side route hosting

## Validation And Runtime Semantics

- public and management servers are built independently and can be started together through the shared runtime model
- graceful shutdown is coordinated through `pkg/core/app`
- management routes keep readiness, liveness, metrics, and version exposure separate from the public API surface
- readiness now preserves explicit degraded runtime posture instead of collapsing every non-healthy state to unavailable
- TLS configuration is loaded explicitly from HTTP server settings

## Testing Expectations

- fast tests cover server construction, TLS loading, management/public route behavior, and lifecycle orchestration
- router contract coverage remains in `pkg/http/router`
- targeted integration/property coverage should continue to protect dual-server behavior and management security semantics

## Status

Target-state HTTP family package finalized by Wave 3 Task `T3.2`.
