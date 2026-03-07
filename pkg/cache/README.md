# `pkg/cache`

Shared cache role for non-HTTP and HTTP consumers.

Owns backend store contracts and shared implementations such as in-memory, Redis, and Memcached.
Also owns the cache-management CLI contribution for applications that choose to expose cache operations.

Non-goals: HTTP cache-key composition, HTTP response semantics, middleware orchestration.
