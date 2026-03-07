# `pkg/http/securityheaders`

HTTP security-header middleware for the HTTP family.

Owns HTTP response header policies such as HSTS, CSP, and related browser protections.

Use from HTTP routes and servers; keep generic security contracts elsewhere.

Non-goals: authn/authz ownership, server lifecycle.

Testing: fast tests cover header application behavior.

Status: target-state HTTP package.
