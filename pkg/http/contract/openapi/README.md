# `pkg/http/contract/openapi`

## Purpose

`pkg/http/contract/openapi` owns the OpenAPI-backed HTTP request validation provider.

## Owned Contracts

The package owns:

- OpenAPI request validation middleware
- provider-specific validation wiring for HTTP requests

## Composition And Wiring Expectations

- HTTP applications may attach this provider to routers when OpenAPI request validation is desired.
- The package is one HTTP contract-validation provider, not the definition of HTTP validation as a whole.

## Non-Goals

- generic HTTP binding ownership
- DTO/input validation ownership
- transport-neutral validation contracts

## Validation And Runtime Semantics

- this package validates HTTP requests against OpenAPI contracts
- provider selection should stay explicit and replaceable

## Testing Expectations

- fast tests cover request-validation behavior and provider attachment semantics

## Status

Target-state HTTP contract-validation provider finalized in Wave 3 Task `T3.3`.
