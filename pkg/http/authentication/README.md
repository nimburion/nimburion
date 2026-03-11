# `pkg/http/authentication`

HTTP authentication middleware for the HTTP family.

Owns bearer-token authentication, claim storage, and HTTP identity propagation helpers.

Non-goals: authorization policy evaluation, server lifecycle.

Testing: fast tests cover JWT authentication and claim propagation.

Status: target-state HTTP package.
