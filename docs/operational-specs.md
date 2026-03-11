# Operational Specs

This document defines the minimum operational contracts that Nimburion-owned runtime features should satisfy.

It is intentionally narrower than [architecture.md](./architecture.md): architecture describes structure and boundaries, while this document defines the minimum production posture expected from the framework.

Related documents:

- [architecture.md](./architecture.md)
- [testing.md](./testing.md)
- [error-model.md](./error-model.md)

## Audit Minimum Contract

Framework-managed audit capabilities should provide:

- append-only audit record semantics
- explicit audit streams or sinks
- integrity-related metadata when tamper awareness is promised
- a clear required-vs-best-effort failure policy
- verification behavior that can be exercised in tests or tooling

## Release And Supply-Chain Contract

Framework releases should support:

- reproducible builds and pinned toolchain expectations
- dependency scanning and vulnerability review in CI
- documented release gating before tagging
- enough runtime metadata to identify framework and dependency posture in deployed services

## Runtime Lifecycle Contract

Runtime features should provide:

- explicit startup and shutdown phases
- graceful shutdown that preserves caller deadlines and cancellation budgets
- readiness and liveness behavior that can be wired into orchestrators
- degraded-mode behavior where the framework claims to support it

## gRPC Runtime Minimum Contract

When gRPC is enabled, the runtime should define:

- listener startup and readiness semantics
- graceful drain behavior
- deadline and cancellation propagation rules
- reflection exposure policy
- health reporting behavior
- observability dimensions expected from framework-owned gRPC paths

## Multi-Region And Disaster-Recovery Contract

Where the framework exposes multi-region or recovery-aware features, it should make explicit:

- posture levels and supported modes
- runtime metadata required to understand the current posture
- recovery states and their transitions
- validation rules that reject unsupported combinations early

## Failure-Injection And Resilience Contract

Framework-owned resilience features should make room for:

- failure-injection testing where runtime guarantees depend on dependency behavior
- explicit retry, poison, quarantine, and backpressure semantics
- repeatable verification for degraded dependencies and partial outages

## Success Criteria

The framework is operating within its intended contract when:

- runtime lifecycle behavior is explicit and testable
- release posture is observable and gated
- security and audit promises are backed by concrete runtime behavior
- transport and adapter behavior expose predictable timeout, shutdown, and error semantics
- the documented operational contracts are covered by stable verification lanes in [testing.md](./testing.md)
