# `pkg/http/sse`

## Purpose

`pkg/http/sse` owns Server-Sent Events support for the HTTP family.

## Owned Contracts

The package owns:

- SSE HTTP handler wiring through `router.Context`
- local connection management and fan-out
- replay stores
- in-memory, Redis, and event-bus-backed broadcast integrations

## Composition And Wiring Expectations

- HTTP applications should wire SSE endpoints through this package rather than a generic realtime root.
- The package may be combined with existing HTTP middleware for auth, rate limiting, metrics, and security headers.
- Multi-instance deployments may add Redis or event-bus-backed fan-out without changing the handler contract.

## Non-Goals

- generic websocket ownership
- transport-neutral pub/sub ownership
- HTTP server lifecycle ownership

## Validation And Runtime Semantics

- SSE streams use the shared HTTP router contract
- replay support remains store-backed and optional
- distributed fan-out remains optional through Redis or event-bus adapters

## Testing Expectations

- fast tests cover handler behavior, manager fan-out, stores, and bus adapters

## Status

Target-state HTTP capability finalized in Wave 3 Task `T3.3`.
