# `pkg/http/middleware`

## Purpose

`pkg/http/middleware` owns shared HTTP middleware helpers and package-local test support for the HTTP family.

## Owned Contracts

- shared HTTP middleware context keys
- integration and ordering tests for HTTP middleware composition
- test helpers used by HTTP middleware packages

## Composition And Wiring Expectations

- concrete HTTP middleware lives in subpackages under this directory
- generic framework code should depend on specific middleware packages, not this root, unless shared keys or test support are required

## Non-Goals

- transport-neutral middleware contracts
- HTTP server lifecycle ownership

## Validation And Runtime Semantics

- ordering and integration tests here protect composition semantics across the HTTP middleware family

## Testing Expectations

- keep integration and ordering coverage here as middleware packages move

## Status

Target-state HTTP family root.
