# `pkg/cache/memcached`

Memcached adapter for the cache role.

Owns Memcached text-protocol connectivity reused by cache backends.

Non-goals:
- session semantics
- generic legacy umbrella ownership

Testing:
- fast adapter tests live in this package

Status: target-state role adapter package.
