# pkg/reliability/dedup

## Purpose

`pkg/reliability/dedup` owns generic deduplication contracts for replay windows where callers only need duplicate detection and marker retention.

## Owned Contracts

- `Store`
- `Deduplicator`
- cleanup lifecycle for expired dedup markers

## Composition And Wiring

This package is transport-neutral. Event consumers, jobs workers, and other runtimes adapt their own scope and key choices.

## Non-goals

- HTTP request middleware
- durable inbox or outbox state
- backend-specific persistence adapters

## Validation, Lifecycle, And Runtime Semantics

- duplicate detection is explicit and retention-window based
- dedup is separate from business execution and transaction boundaries

## Testing Expectations

- build and fast tests for this package

## Status

Target-state package.
