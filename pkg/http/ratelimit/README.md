# `pkg/http/ratelimit`

HTTP rate-limiting middleware for the HTTP family.

Owns HTTP request throttling policy and HTTP-facing limiter behavior.

Use from HTTP routes; backend limiter/storage details stay replaceable.

Non-goals: generic coordination/cache ownership, server lifecycle.

Testing: fast tests cover limit enforcement and backend integration behavior.

Status: target-state HTTP package.
