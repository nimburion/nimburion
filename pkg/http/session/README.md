# `pkg/http/session`

HTTP session middleware and HTTP-facing session helpers for the HTTP family.

Owns request/session binding, cookie handling, and HTTP session store integration.

Use from HTTP routes and handlers; backend stores now live in [`pkg/session`](/Users/giuseppe/Workspace/github.com/nimburion/nimburion-refactoring-worktree/pkg/session/README.md).

Non-goals: transport-neutral session-store ownership, runtime lifecycle ownership.

Testing: fast tests cover cookie/session behavior and store integrations.
