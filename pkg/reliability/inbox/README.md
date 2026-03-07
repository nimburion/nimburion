# pkg/reliability/inbox

## Purpose

`pkg/reliability/inbox` owns generic inbox contracts for recorded-before-rehandle semantics on message consumption.

## Owned Contracts

- `Entry`
- `Store`
- transactional `Writer` and `TxExecutor`

## Composition And Wiring

Eventing or jobs consumers adapt transport metadata into inbox entries and use the transactional contract when they need recorded-before-rehandle behavior.

## Non-goals

- broker-specific consumer code
- event bus handler registration
- jobs worker runtime ownership

## Validation, Lifecycle, And Runtime Semantics

- inbox entries move through explicit `received` and `handled` states
- duplicate replays must not bypass duplicate protection once an entry is handled
- transactional helpers keep inbox state and business effects in one boundary when supported

## Testing Expectations

- build and fast tests for this package
- contract-style tests for transactional inbox behavior

## Status

Target-state package.
