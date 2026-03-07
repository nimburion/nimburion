# pkg/policy

## Purpose

`pkg/policy` owns transport-neutral authorization and policy decision contracts.

## Owned Contracts

- `Subject`
- scope requirements and evaluation
- claim-rule evaluation contracts
- `Authorizer`
- `Decision`

## Composition And Wiring

Transport families adapt request or message metadata into `policy.Subject` and evaluation inputs. Decision engines remain outside HTTP-specific middleware.

## Non-goals

- HTTP middleware ownership
- JWT validation
- tenant context storage

## Validation, Lifecycle, And Runtime Semantics

- supports RBAC-like scope checks
- supports ABAC-like claim and attribute evaluation
- supports external policy-engine providers through `Authorizer`

## Testing Expectations

- build and fast tests for this package
- authorization provider suites should build on these contracts

## Status

Target-state package.
