# `pkg/http/i18n`

HTTP locale negotiation middleware for the HTTP family.

Owns request-local locale selection from headers, query params, and related HTTP inputs.

Use from HTTP routes only.

Non-goals: domain translation ownership, transport-neutral i18n contracts.

Testing: fast tests cover locale negotiation and fallback behavior.

Status: target-state HTTP package.
