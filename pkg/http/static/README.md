# `pkg/http/static`

Static asset serving support for the HTTP family.

Owns HTTP-specific filesystem/static-file serving behavior.

Use from HTTP routes and servers; keep generic filesystem helpers elsewhere.

Non-goals: CDN/origin strategy ownership, server lifecycle.

Testing: fast tests cover filesystem routing and asset responses.

Status: target-state HTTP package.
