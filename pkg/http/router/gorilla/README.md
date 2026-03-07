# `pkg/http/router/gorilla`

## Purpose

`pkg/http/router/gorilla` provides the Gorilla Mux adapter for the shared `pkg/http/router` contract.

## Owned Contracts

The package owns:

- the Gorilla-backed implementation of `router.Router`
- adapter glue between Gorilla request handling and the shared router contract

## Composition And Wiring Expectations

- Use this adapter when an application intentionally chooses Gorilla Mux at composition time.
- Generic framework code should continue to depend on `pkg/http/router`, not Gorilla-specific types.

## Non-Goals

- HTTP server lifecycle ownership
- Gorilla-specific public APIs for unrelated framework packages
- a second routing contract outside the shared HTTP router family

## Validation And Runtime Semantics

- shared middleware ordering and handler semantics must remain adapter-neutral
- route grouping and parameter behavior must stay aligned with the common contract suite

## Testing Expectations

- adapter tests and the shared contract suite must pass

## Status

Target-state HTTP router adapter finalized in Wave 3 Task `T3.1`.
