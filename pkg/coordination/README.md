# pkg/coordination

## Purpose

`pkg/coordination` owns cross-instance coordination primitives used by multiple runtime families.

## Owned Contracts

- `LockProvider`
- `LockLease`
- typed coordination errors

## Composition And Wiring

Applications and higher-level framework packages inject coordination providers explicitly.
`pkg/scheduler` depends on `coordination.LockProvider` but does not own concrete lock backends.
Provider-specific implementations live in subpackages such as `pkg/coordination/redis` and `pkg/coordination/postgres`.

## Non-goals

- scheduler task registration
- scheduler runtime orchestration
- generic Redis or SQL abstractions
- durable messaging or jobs semantics

## Validation, Lifecycle, And Runtime Semantics

- providers validate required connection and backend settings at construction time
- lease acquisition, renewal, and release are typed as coordination concerns rather than scheduler concerns
- providers must implement `HealthCheck` and `Close` so they can participate in runtime lifecycle and health surfaces

## Testing Expectations

- build and fast tests must pass for `pkg/coordination/...`
- lock-provider adapters should keep integration coverage for backend-specific behavior
- scheduler multi-instance behavior must stay covered from the scheduler side against the coordination contract

## Status

Target-state package.
