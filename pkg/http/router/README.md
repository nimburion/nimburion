# `pkg/http/router`

## Purpose

`pkg/http/router` owns the transport-level router contract for the HTTP family. It defines the shared abstractions used by HTTP servers, middleware, and handlers without binding the framework to a single router implementation.

## Owned Contracts

The package owns:

- `Router`
- `Context`
- `ResponseWriter`
- `HandlerFunc`
- `MiddlewareFunc`
- the shared router contract suite under `contract`

## Composition And Wiring Expectations

- HTTP runtime packages depend on this contract instead of adapter-specific router types.
- Adapter packages live under `pkg/http/router/nethttp`, `pkg/http/router/gin`, and `pkg/http/router/gorilla`.
- Runtime wiring should inject concrete routers explicitly at composition time.

## Non-Goals

- HTTP server lifecycle ownership
- request/response validation ownership
- HTTP error mapping
- adapter-specific middleware ecosystems beyond the shared router contract

## Validation And Runtime Semantics

- handlers return `error` through a router-agnostic signature
- middleware ordering must remain stable across adapters
- request cancellation and request-scoped context propagation must remain adapter-neutral
- adapter-specific request/response objects must not leak into the common contract

## Testing Expectations

- fast tests cover package-level interface expectations and adapter-specific behavior
- contract tests under `contract` must pass for every supported adapter
- property tests should continue to protect middleware ordering and interface compatibility

## Status

Target-state HTTP family package introduced by Wave 3 Task `T3.1`.
