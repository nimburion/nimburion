# `pkg/http/csrf`

HTTP CSRF protection middleware for the HTTP family.

Owns CSRF token issuance and validation for HTTP requests.

Use from HTTP routes; keep transport-neutral code out of this package.

Non-goals: session store ownership, server lifecycle.

Testing: fast tests cover token validation and exempt-path behavior.

Status: target-state HTTP package.
