# `internal/testharness/nonfunctional`

## Purpose

Provides the shared harness helpers for standard non-functional verification families.

## Owned Contracts

- canonical categories for:
  - performance
  - load
  - soak
  - resilience
  - security
  - compatibility
  - race
  - ordering
- explicit opt-in gating through `NIMB_NONFUNCTIONAL`

## Usage

- use `Run` or `RequireEnabled` from package-local non-functional suites
- keep long-running or environment-heavy suites off the default fast path
- use `make test-nonfunctional-lane TEST_PKG=./path/...` as the stable command entry point
