# pkg/reliability/outbox

## Purpose

`pkg/reliability/outbox` owns generic durable handoff contracts for publish-after-write workflows.

## Owned Contracts

- `Entry`
- `Record`
- `Store`
- transactional `Writer` and `TxExecutor`
- `PostgresTxExecutor` bridge for `pkg/persistence/relational/postgres` transaction managers
- `Publisher` and relay lifecycle

## Composition And Wiring

Transport families adapt `Publisher` to their own broker or delivery model. This package owns the generic outbox state machine, not broker-specific publishing.

## Non-goals

- broker-specific producers
- event bus adapter ownership
- persistence adapter ownership

## Validation, Lifecycle, And Runtime Semantics

- outbox entries move through explicit pending, retry, and published states
- failed publish attempts do not mark delivery success
- relay behavior assumes at-least-once delivery and explicit retry/backoff

## Testing Expectations

- build and fast tests for this package
- contract-style tests for transactional atomicity and relay retry behavior

## Status

Target-state package.
