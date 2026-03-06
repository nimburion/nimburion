# `pkg/core/errors`

## Purpose

`pkg/core/errors` owns the transport-neutral application error contracts for the refactored runtime.

## Owned Contracts

- `AppError`
- `Message`
- `Params`
- constructors and helpers for fallback message, details, and status hints

## Composition And Wiring Expectations

- application and domain layers return `*errors.AppError`
- transports map core errors to HTTP, gRPC, or other protocol-specific surfaces
- localization layers may translate stable `Code` and `Params` without owning the error type

## Non-Goals

- HTTP response rendering
- gRPC status mapping
- localization lookup
- transport serialization

## Validation And Runtime Semantics

- `Code` is the stable machine-readable identifier
- `FallbackMessage` is optional for non-localized output
- `Cause` is preserved for `errors.Is` and `errors.As`
- `HTTPStatus` is only a mapping hint, not a transport dependency

## Testing Expectations

- fast tests cover wrapping, cloning, and helper semantics

## Status

Target-state package introduced by Wave 1 Task `T1.3`.
