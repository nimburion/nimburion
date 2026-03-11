# `pkg/http/cors`

HTTP-only CORS policy middleware for the HTTP family.

Owns CORS request/response policy handling through the shared HTTP router contract.

Use from HTTP routes and servers; do not treat it as a transport-neutral capability.

Non-goals: generic policy ownership, server lifecycle.

Testing: fast tests cover origin/method/header policy behavior.

Status: target-state HTTP package.
