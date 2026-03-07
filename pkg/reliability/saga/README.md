# pkg/reliability/saga

## Purpose

`pkg/reliability/saga` owns workflow-level consistency contracts for long-running distributed processes.

## Owned Contracts

- saga `Definition`
- `Step`
- lifecycle `Status`
- `Runner`
- observer `Event`

## Composition And Wiring

This package stays transport- and persistence-neutral. Applications or higher-level framework families can attach persistence, messaging, or orchestration behavior around these contracts.

## Non-goals

- persistence implementation
- transport-specific orchestration
- hidden workflow state inside unrelated handlers

## Validation, Lifecycle, And Runtime Semantics

- saga steps execute in order
- completed steps compensate in reverse order on failure
- workflow status transitions are explicit and observable

## Testing Expectations

- build and fast tests for this package
- compensation-ordering and failure-path behavior must remain covered

## Status

Target-state package.
