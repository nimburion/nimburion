# `pkg/http/response`

## Purpose

`pkg/http/response` owns HTTP response helpers and HTTP-specific mapping from core application errors to response payloads.

## Owned Contracts

- success response helpers
- HTTP error body shape
- HTTP mapping from `pkg/core/errors`

## Composition And Wiring Expectations

- HTTP handlers use this package instead of `pkg/controller`
- application and domain layers return `pkg/core/errors`
- transport code maps those errors here

## Non-Goals

- input validation
- transport-neutral error ownership
- router implementation ownership

## Validation And Runtime Semantics

- request ids are propagated from context when present
- unknown errors map to `500 internal_server_error`
- localized translation remains optional and fallback-message aware

## Testing Expectations

- fast tests cover success helpers, error mapping, and constructor semantics

## Status

Target-state package introduced ahead of Wave 3 cleanup.
