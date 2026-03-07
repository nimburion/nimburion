# pkg/reliability/retry

## Purpose

`pkg/reliability/retry` owns shared retry budget, failure classification, poison-handling, dead-letter, and quarantine semantics.

## Owned Contracts

- `Budget`
- `Classification`
- `Disposition`
- `Policy`
- `Decision`
- `QuarantineRecord`
- `QuarantineSink`

## Composition And Wiring

Transport families and runtimes keep their own delivery glue, but they should use this package for retry taxonomy and retry-vs-dead-letter-vs-quarantine decisions.
Quarantine sinks can be injected by consumers that want a concrete quarantine path instead of dead-letter fallback.

## Non-goals

- broker-specific dead-letter publishing
- jobs backend ownership
- concrete quarantine persistence implementations

## Validation, Lifecycle, And Runtime Semantics

- the shared model assumes at-least-once delivery
- poison failures can bypass retry and move directly to quarantine or dead-letter handling
- quarantine is an explicit disposition even when a caller currently maps it to dead-letter as an operational fallback

## Testing Expectations

- build and fast tests for this package
- retry-budget and poison-handling behavior should remain covered through contract-style tests in consumers

## Status

Target-state package.
