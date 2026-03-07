# pkg/coordination/postgres

## Purpose

`pkg/coordination/postgres` provides a Postgres-backed implementation of `coordination.LockProvider`.

## Owned Contracts

- `PostgresLockProvider`
- `PostgresLockProviderConfig`

## Composition And Wiring

This package is a concrete coordination adapter injected by application wiring. `pkg/scheduler` depends only on the coordination contract, not on this package directly.

## Non-goals

- relational repository ownership
- scheduler runtime ownership
- generic SQL abstraction reuse across unrelated families

## Validation, Lifecycle, And Runtime Semantics

- validates logger, URL, and table name at construction
- creates or checks the lock table during provider startup
- uses lease tokens and expiry timestamps to preserve multi-instance singleton execution

## Testing Expectations

- build and fast tests for this package
- integration coverage when Postgres-backed locking behavior or table semantics change

## Status

Target-state package.
