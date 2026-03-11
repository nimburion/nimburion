# `pkg/http/cache`

HTTP response-cache middleware for the HTTP family.

Owns cache-key construction, HTTP response caching, and HTTP cache middleware behavior.

Use from HTTP routes; backend stores now live in [`pkg/cache`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/cache/README.md).

Non-goals: generic cache capability ownership, store backend ownership, server lifecycle.

Testing: fast tests cover cache hits/misses, vary behavior, and backend integrations.
