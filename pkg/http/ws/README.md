# `pkg/http/ws`

## Purpose

`pkg/http/ws` owns WebSocket support for the HTTP family.

## Owned Contracts

The package owns:

- lightweight WebSocket connection handling
- keepalive behavior and related runtime helpers

## Composition And Wiring Expectations

- HTTP applications that need WebSocket transport should depend on this package instead of a generic realtime root.
- Router and HTTP server ownership remain outside this package.

## Non-Goals

- SSE ownership
- HTTP server lifecycle ownership
- transport-neutral messaging contracts

## Validation And Runtime Semantics

- keepalive behavior remains explicit and package-local
- HTTP-family routing and upgrade wiring stay outside the common runtime core

## Testing Expectations

- fast tests cover connection handling and keepalive behavior

## Status

Target-state HTTP capability finalized in Wave 3 Task `T3.3`.
