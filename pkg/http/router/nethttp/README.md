# `pkg/http/router/nethttp`

## Purpose

`pkg/http/router/nethttp` provides the `net/http` adapter for the shared `pkg/http/router` contract.

## Owned Contracts

The package owns:

- the `net/http` implementation of `router.Router`
- path parameter extraction and route grouping for the standard library adapter
- request binding and response helpers needed to satisfy the shared router contract

## Composition And Wiring Expectations

- Use this adapter when you want the standard library router implementation with minimal external dependencies.
- It should be wired through `pkg/http/router` contracts, not consumed as an application-specific abstraction.

## Non-Goals

- HTTP server lifecycle ownership
- framework-wide middleware policy ownership
- a separate routing contract outside `pkg/http/router`

## Validation And Runtime Semantics

- path parameters use `:name` syntax
- middleware composition must match the shared contract suite semantics
- cancellation and request context must flow through the underlying `net/http` request

## Testing Expectations

- fast tests cover routing, params, query parsing, binding, grouping, and middleware
- contract and property tests must continue to pass for the adapter

## Status

Target-state HTTP router adapter finalized in Wave 3 Task `T3.1`.
