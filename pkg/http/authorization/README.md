# `pkg/http/authorization`

HTTP authorization middleware for the HTTP family.

Owns scope checks, claim-guard evaluation, and authorization policy helpers on the HTTP path.

Non-goals: token authentication, server lifecycle.

Testing: fast tests cover scope enforcement and claim-guard semantics.

Status: target-state HTTP package.
