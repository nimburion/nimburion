# `pkg/session/redis`

Redis adapter for the session role.

Owns Redis connectivity reused by session backends and session-oriented storage semantics.

Non-goals:
- cache semantics
- generic legacy umbrella ownership

Testing:
- fast, property, and integration tests live in this package

Status: target-state role adapter package.
