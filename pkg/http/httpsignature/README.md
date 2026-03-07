# `pkg/http/httpsignature`

HTTP signature verification middleware for the HTTP family.

Owns signature-header parsing and request verification for signed HTTP requests.

Use from HTTP routes when request signing is required.

Non-goals: generic key-management ownership, server lifecycle.

Testing: fast tests cover signature validation and replay/clock-skew behavior.

Status: target-state HTTP package.
