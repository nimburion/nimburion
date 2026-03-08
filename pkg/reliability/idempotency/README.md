# pkg/reliability/idempotency

## Purpose

`pkg/reliability/idempotency` owns generic idempotency contracts for replay-safe execution under at-least-once delivery.

## Owned Contracts

- `Store`
- `Guard`
- transactional `TxStore` and `TxExecutor`
- cleanup lifecycle for stale idempotency markers

## Composition And Wiring

This package is transport-neutral. HTTP, event consumers, and jobs workers adapt their own metadata into `scope` and `key` values.

## Non-goals

- HTTP middleware
- event bus consumer wiring
- jobs worker wiring
- backend-specific persistence adapters

## Validation, Lifecycle, And Runtime Semantics

- duplicate control is explicit and scope-based
- the contract assumes at-least-once upstream delivery
- non-transactional execution requires a store that explicitly provides atomic duplicate suppression under contention
- transactional helpers keep business effects and idempotency markers in one boundary when supported

## Testing Expectations

- build and fast tests for this package
- property coverage for repeated-key semantics

## Status

Target-state package.
