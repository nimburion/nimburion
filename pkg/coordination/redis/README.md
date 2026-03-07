# pkg/coordination/redis

## Purpose

`pkg/coordination/redis` provides a Redis-backed implementation of `coordination.LockProvider`.

## Owned Contracts

- `RedisLockProvider`
- `RedisLockProviderConfig`

## Composition And Wiring

This package is a concrete adapter. It is selected by application wiring or feature composition, not by scheduler internals.

## Non-goals

- scheduler runtime ownership
- generic Redis helper reuse across unrelated roles
- jobs, cache, or session semantics

## Validation, Lifecycle, And Runtime Semantics

- validates Redis URL and logger at construction
- uses lease tokens plus Redis TTL semantics for acquire, renew, and release
- implements health and close hooks for framework lifecycle integration

## Testing Expectations

- build and fast tests for this package
- backend integration tests when Redis-backed lock behavior changes

## Status

Target-state package.
