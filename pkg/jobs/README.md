# pkg/jobs

## Purpose

`pkg/jobs` owns background work contracts, lease-aware backends, worker runtime behavior, and jobs observability.

## Owned Contracts

- `Job`
- `Runtime`
- `Backend`
- worker and retry policy contracts
- DLQ and lease semantics owned by jobs backends

## Composition And Wiring

Applications compose jobs explicitly through the family owner `pkg/jobs`. Config-driven construction lives in root APIs such as `jobs.NewRuntimeFromConfig(...)` and `jobs.NewBackendFromConfig(...)`; concrete backends remain direct packages such as the Redis backend or the event-bus bridge. The jobs CLI command tree is contributed by the family through `pkg/jobs/feature`.

## Non-goals

- event bus ownership
- scheduler ownership
- generic coordination primitives
- HTTP transport concerns

## Validation, Lifecycle, And Runtime Semantics

- jobs remain distinct from durable event bus messages and from ephemeral pub/sub
- runtimes and backends expose health and close hooks for app lifecycle integration
- workers own retries, lease renewal, and dead-letter behavior

## Testing Expectations

- build and fast tests for `pkg/jobs/...`
- integration tests for concrete backends
- behavior coverage for lease, retry, DLQ, and worker concurrency semantics

## Status

Target-state package.
