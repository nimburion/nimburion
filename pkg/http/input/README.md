# `pkg/http/input`

## Purpose

`pkg/http/input` owns HTTP-adjacent DTO and request input validation helpers.

## Owned Contracts

- `Validator`
- `ValidateDTO`

## Composition And Wiring Expectations

- HTTP handlers and binding layers validate DTOs through this package
- validation failures return `pkg/http/response` application-aware HTTP errors

## Non-Goals

- HTTP response rendering
- transport-neutral error ownership
- schema generation or contract validation providers

## Validation And Runtime Semantics

- `Validator` implementations are trusted as the primary validation path
- non-validator structs get minimal required-tag checks
- validation failures are wrapped into structured application errors

## Testing Expectations

- fast tests cover DTO nil handling, wrapping, required-tag checks, and preservation of existing application errors

## Status

Target-state package introduced ahead of Wave 3 cleanup.
