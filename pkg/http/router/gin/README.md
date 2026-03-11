# `pkg/http/router/gin`

## Purpose

`pkg/http/router/gin` provides the Gin adapter for the shared `pkg/http/router` contract.

## Owned Contracts

The package owns:

- the Gin-backed implementation of `router.Router`
- adapter glue between Gin request handling and the shared router contract

## Composition And Wiring Expectations

- Use this adapter when an application intentionally chooses Gin at composition time.
- The rest of the framework should still depend on `pkg/http/router` contracts rather than Gin types.

## Non-Goals

- framework ownership of Gin-specific middleware stacks
- HTTP server lifecycle ownership
- a Gin-first public contract for generic framework code

## Validation And Runtime Semantics

- adapter behavior must remain contract-compatible with the other router implementations
- shared middleware ordering and handler semantics must not diverge from the common suite

## Testing Expectations

- adapter tests and the shared contract suite must pass

## Status

Target-state HTTP router adapter finalized in Wave 3 Task `T3.1`.
