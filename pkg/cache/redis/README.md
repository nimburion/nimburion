# `pkg/cache/redis`

Redis adapter for the cache role.

Owns Redis connectivity and cache-oriented operations used by cache backends.

Non-goals:
- session semantics
- generic legacy umbrella ownership

Testing:
- fast, property, and integration tests live in this package

Status: target-state role adapter package.
