# pkg/audit

## Purpose

`pkg/audit` owns structured audit records, sink contracts, chain verification, and shared masking helpers for framework-managed surfaces.

## Owned Contracts

- `Record`
- `Sink`
- `Writer`
- chain verification via `VerifyChain`
- shared masking helpers for config-like map structures

## Composition And Wiring

Framework families emit structured audit records through injected sinks. Masking helpers are reusable across config rendering, logs, audit payloads, and transport-facing debug output.

## Non-goals

- concrete storage backends
- transport-local audit implementations
- ad-hoc package-specific masking logic

## Validation, Lifecycle, And Runtime Semantics

- audit records are append-only at the contract level
- each stream uses monotonic sequence plus `prev_digest` chaining
- required vs best-effort sink policy is explicit in the record
- masking helpers are shared and should be reused instead of reimplemented per package

## Testing Expectations

- build and fast tests for this package
- contract-style tests for digest chaining and verifier behavior
- masking tests for recursive config-like structures

## Status

Target-state package.
