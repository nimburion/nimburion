# pkg/scheduler

## Purpose

`pkg/scheduler` owns task registration, schedule evaluation, dispatch runtime behavior, and scheduler observability.

## Owned Contracts

- `Task`
- scheduler runtime config
- scheduler health and metrics helpers

## Composition And Wiring

`pkg/scheduler` depends on `coordination.LockProvider` for distributed singleton behavior.
Concrete lock implementations are injected from `pkg/coordination/*`.

## Non-goals

- concrete distributed lock backends
- generic coordination primitives
- jobs backend ownership
- transport-specific delivery concerns

## Validation, Lifecycle, And Runtime Semantics

- scheduler runtime dispatches jobs through a `jobs.Runtime`
- multi-instance safety is enforced via lock acquisition, renewal, and release through `pkg/coordination`
- no scheduler-owned provider selection or backend factory remains in this package

## Testing Expectations

- build and fast tests for `pkg/scheduler`
- contract coverage for task validation, lock coordination, and multi-instance execution behavior

## Status

Target-state package.
