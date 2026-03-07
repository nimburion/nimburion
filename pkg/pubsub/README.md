# pkg/pubsub

## Purpose

`pkg/pubsub` is the target-state family root for ephemeral fan-out that does not promise durable event bus semantics and does not model lease-aware jobs.

## Owned Contracts

No stable root-level contracts exist yet on this branch.
The family root is reserved so future pub/sub adapters do not leak back into `pkg/eventbus`, `pkg/jobs`, or transport packages.

## Composition And Wiring

Applications should use this family for ephemeral cross-instance fan-out only.
Durable messaging stays in `pkg/eventbus`.
Lease-aware work stays in `pkg/jobs`.
HTTP-specific realtime integrations such as SSE and WebSocket remain under `pkg/http/*` until a shared pub/sub contract is actually extracted.

## Non-goals

- durable broker semantics
- outbox or retry/dlq ownership
- jobs leasing semantics
- HTTP connection management

## Validation, Lifecycle, And Runtime Semantics

- ephemeral delivery semantics only
- no durability promise is implied by this family root
- concrete contracts should be introduced only when at least one non-HTTP consumer exists

## Testing Expectations

- build gate must pass for the family root
- adapter-specific fast and integration suites will be required once concrete pub/sub adapters land here

## Status

Target-state family root, currently reserved and intentionally minimal.
