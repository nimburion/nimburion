# `pkg/session`

Shared session role for non-HTTP and HTTP consumers.

Owns backend store contracts and shared implementations such as in-memory, Redis, and Memcached.

Non-goals: cookie semantics, request binding, HTTP middleware orchestration.
