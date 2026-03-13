# pkg/pubsub

## Purpose

`pkg/pubsub` owns ephemeral in-process fan-out contracts and a default in-memory implementation.

## Owned Contracts

This package exposes the base contracts:

- `Topic`
- `Message`
- `Subscriber`
- `Bus`

And includes:

- `InMemoryBus`, a goroutine-safe non-durable implementation
- `RedisStore`, a best-effort replay store backed by `internal/rediskit`

## Delivery Semantics

`InMemoryBus` provides:

- in-process fan-out only (no broker persistence, no retry)
- optional publish history persistence via pluggable `Store` (for example `RedisStore`)
- per-subscriber buffered channels
- non-blocking publish for slow subscribers (message drop on full buffer)
- explicit unsubscribe and bus shutdown support

Messages are delivered in publish order for each individual subscriber/channel.

## Composition And Wiring

Use this package when application components need lightweight ephemeral broadcasts.
Durable messaging remains in `pkg/eventbus`.
Outbox/retry semantics remain in `pkg/reliability/*`.

## Non-goals

- durable broker semantics
- outbox or retry/dlq ownership
- jobs leasing semantics
- HTTP connection management

## Testing Expectations

- build gate must pass for the package
- fast tests include property-based checks for order, subscriber independence, and clean shutdown

## Status

Implemented: base contracts and `InMemoryBus` are available on this branch.
