# `pkg/http/openapi`

## Purpose

`pkg/http/openapi` owns OpenAPI spec generation, spec serving, and Swagger UI serving as HTTP-family capabilities.

## Owned Contracts

The package owns:

- route collection for OpenAPI generation
- minimal OpenAPI spec assembly and file writing
- HTTP handlers for serving generated specs
- Swagger UI serving for management surfaces

## Composition And Wiring Expectations

- HTTP management routers may mount these handlers when the OpenAPI capability is included.
- CLI and tooling code may use the generator helpers without importing `pkg/http/server`.
- Swagger UI should remain an explicitly enabled management capability, not a public API default.

## Non-Goals

- transport-neutral runtime lifecycle ownership
- router contract ownership
- request validation provider ownership beyond exposing OpenAPI assets

## Validation And Runtime Semantics

- generated specs derive from routes collected through `pkg/http/router`
- Swagger UI remains disabled unless explicitly enabled
- management surfaces should expose OpenAPI routes separately from public API routes

## Testing Expectations

- fast tests cover spec generation, route collection, file serving, and Swagger serving

## Status

Target-state HTTP capability finalized in Wave 3 Task `T3.3`.
