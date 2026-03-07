# pkg/tenant

## Purpose

`pkg/tenant` owns tenant identity and tenant-scoped runtime context contracts.

## Owned Contracts

- `Identity`
- `Context`
- context propagation helpers

## Composition And Wiring

HTTP, jobs, and eventing families adapt their transport metadata into tenant context and propagate it through runtime context.

## Non-goals

- authorization decisions
- persistence-level multitenancy enforcement
- transport middleware ownership

## Validation, Lifecycle, And Runtime Semantics

- tenant identity is explicit, not implicit
- tenant-blind paths should remain a conscious exception

## Testing Expectations

- build and fast tests for this package
- tenant propagation suites in transport families should build on this contract

## Status

Target-state package.
