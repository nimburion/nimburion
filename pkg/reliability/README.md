# pkg/reliability

## Purpose

`pkg/reliability` owns cross-transport reliability contracts for duplicate control and durable handoff.

## Owned Contracts

- idempotency
- deduplication
- inbox
- outbox
- retry
- saga

## Composition And Wiring

This family defines reusable reliability semantics. Transport-specific adapters stay with their transport family:

- HTTP idempotency integration belongs under `pkg/http/*`
- event-consumer glue belongs under `pkg/eventbus/*`
- jobs-specific duplicate control belongs under `pkg/jobs/*`

## Non-goals

- broker-specific transport code
- HTTP request handling
- jobs backend ownership
- persistence adapter ownership

## Validation, Lifecycle, And Runtime Semantics

- the framework default remains at-least-once for distributed transports
- idempotency, deduplication, inbox, and outbox make replay and duplicate control explicit
- retry, poison-handling, dead-letter, and quarantine decisions are modeled explicitly
- concrete backend choice stays outside the shared contract packages

## Testing Expectations

- build gate for `pkg/reliability/...`
- fast tests for idempotency and dedup
- contract-style tests for inbox and outbox transactional behavior

## Status

Target-state package.
