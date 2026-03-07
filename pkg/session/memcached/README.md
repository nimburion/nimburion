# `pkg/session/memcached`

Memcached adapter for the session role.

Owns Memcached text-protocol connectivity reused by session backends, including touch semantics for session TTL refresh.

Non-goals:
- cache semantics
- generic legacy umbrella ownership

Testing:
- fast adapter tests live in this package

Status: target-state role adapter package.
