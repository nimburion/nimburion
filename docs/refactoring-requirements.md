# Refactoring Requirements

This document defines the target architecture for the ongoing refactor. It exists to avoid treating the current package layout as the design target for new code.

Execution order, package moves, and milestone scope are tracked in [refactoring-plan.md](./refactoring-plan.md).
Target package boundaries, contracts, and composition model are described in [refactoring-design.md](./refactoring-design.md).
User interactions and workflow goals are tracked in [refactoring-user-stories.md](./refactoring-user-stories.md).
Story-to-requirements/design/milestone mapping is tracked in [refactoring-traceability.md](./refactoring-traceability.md).

## Status

- The packages under `pkg/server`, `pkg/store`, `pkg/configschema`, `pkg/migrate`, and `pkg/controller` describe the current implementation.
- They are still valid for the running codebase, but they are not the preferred extension points for new framework code.
- Until the refactor is complete, current implementation docs must stay accurate and must also be marked as transitional when they would otherwise guide new code in the wrong direction.

## Architectural Direction

- Nimburion moves toward compile-time composition, not runtime plugin loading.
- `pkg/core` owns lifecycle, feature model, health aggregation, base observability, and minimal shared config.
- `pkg/cli` remains a shared CLI framework for user projects, not the runtime core.
- The primary application entrypoint becomes `Run`; HTTP-specific `serve` is a transport concern, not the universal default.
- `healthcheck` remains a standard command, but it must be registry-driven: the core aggregates checks and features/modules contribute them.

## Packaging Rules For New Code

- Do not add new generic capabilities under `pkg/store`.
- Do not expand `pkg/configschema`; target package is `pkg/config/schema`.
- Do not add new string-based factory selection as a framework default when direct constructor injection is possible.
- Do not add new responsibilities to `pkg/controller`; that package is being dismantled into more specific core and HTTP packages.
- When a capability is transport-specific, keep it under the transport family instead of a generic umbrella package.

## Persistence And Roles

- Persistence families are modeled explicitly instead of through a single generic `store` concept.
- Target persistence families are:
  - `persistence/relational`
  - `persistence/keyvalue`
  - `persistence/document`
  - `persistence/widecolumn`
  - `persistence/search`
  - `persistence/graph`
  - `persistence/timeseries`
  - `persistence/object`
- Operational roles remain distinct from persistence families, for example:
  - `cache`
  - `session`
  - `coordination`
  - `jobs`
  - `pubsub`

## Validation Model

- Transport parsing/binding, contract validation, and domain/input validation are separate layers.
- For HTTP:
  - low-level bind/parse validation stays transport-specific
  - request/response contract validation is a capability with providers such as OpenAPI, JSON Schema, CUE, or custom implementations
  - DTO/input validation remains a separate application-level concern
- For events:
  - envelope validation, contract validation, and domain validation remain separate
  - contract validation providers can include Avro, Protobuf, JSON Schema, CUE, or custom implementations

## Configuration And Secrets

- `pkg/config` remains the configuration infrastructure package.
- `pkg/config/schema` is the target home for schema generation and schema composition.
- Configuration is moving from one monolithic root struct toward composition by core + features/modules.
- `secrets file` remains the current first-class secrets source.
- Sensitive-field handling must be metadata-driven at the field level, not inferred from which source provided the final value.
- The framework must define a precise metadata model for config fields that can express at least:
  - classification such as `public`, `sensitive`, or `secret`
  - redaction policy such as `none`, `mask`, or `full`
  - render target such as `config` or `secrets`
  - allowed input sources such as `default`, `config_file`, `secrets_file`, `env`, `flag`, or external secret provider
- The framework must track value provenance separately from sensitivity so mixed-source resolution keeps:
  - one effective value
  - one stable field classification
  - one sanitized source provenance record
- Future external secret providers must be supported by design, but they are evolutive requirements rather than mandatory first-milestone scope:
  - AWS Secrets Manager
  - AWS SSM Parameter Store
  - Azure Key Vault
  - Google Secret Manager
- New code must not assume that secrets can only come from a local secrets file.
- Sensitive metadata must propagate consistently to:
  - schema generation
  - config render output
  - config show output
  - framework-managed logging
  - panic or debug dumps
  - framework introspection payloads
- Error messages and debug output must follow the same redaction policy as config rendering. They may name a field or source, but they must not emit the secret value itself.

## Distributed Reliability And Consistency

- The framework must define an explicit application-level reliability model for distributed systems that assume at-least-once delivery, not exactly-once guarantees.
- The architecture must include first-class support or contracts for:
  - idempotency keys
  - deduplication semantics
  - inbox/outbox patterns
  - poison message handling
  - retry taxonomy and retry budgets
  - backpressure and consumer lag visibility
- Exactly-once semantics must not be implied where the underlying transport cannot guarantee them.
- Saga or process-manager style orchestration must be treated as an explicit capability when workflows cross service or transaction boundaries.

## Enterprise Security Controls

- The framework must support an authorization provider model that can express RBAC, ABAC, and policy-engine-backed decisions.
- Multi-tenant isolation must be explicit in runtime context, transport handling, and stateful backend contracts.
- Secret rotation and certificate rotation lifecycles must be modeled as real operational concerns, not left implicit.
- The architecture must include contracts or conventions for:
  - tamper-aware audit logging
  - security event emission
  - PII classification
  - field-level masking/redaction
- The framework's own supply-chain and security posture must be treated as a first-class concern in release and dependency policy.
- Tamper-aware audit logging must have a minimum operational contract:
  - append-only audit streams
  - per-stream sequencing
  - digest chaining or equivalent integrity metadata
  - verifier-compatible artifacts
  - required-vs-best-effort emission policy
- Supply-chain posture must have a minimum release contract:
  - dependency inventory or SBOM
  - vulnerability policy evaluation
  - checksum or integrity artifact
  - provenance or attestation hook where the release path supports it
  - explicit release failure when required evidence is missing or out of policy
- See [refactoring-operational-specs.md](./refactoring-operational-specs.md) for the minimum operational envelope.

## Enterprise Operations

- The framework must define conventions for SLI, SLO, golden signals, and alertability.
- Startup, readiness, liveness, and degraded-mode semantics must be explicit and reusable across features.
- The framework must support rollout safety concerns such as feature flags and config rollout safety patterns.
- The operational model must include hooks or guidance for:
  - failure injection and chaos testing
  - partial dependency tolerance
  - multi-region posture
  - disaster recovery posture
- Multi-region and DR posture must define a shared minimum contract for:
  - supported posture levels
  - recovery-oriented runtime states
  - replay and idempotency assumptions
  - validation of unsupported topology and dependency combinations
- See [refactoring-operational-specs.md](./refactoring-operational-specs.md) for the minimum posture levels and validation rules.

## Data Governance And Compliance

- Auditability must be a cross-cutting concern, not an application-specific afterthought.
- The framework must define patterns or contracts for:
  - retention-aware data handling
  - tenant-aware storage, cache, and session behavior
  - erasure and compliance-driven deletion workflows
  - compliance-oriented logging and masking controls

## Performance And Capacity Engineering

- The framework must define a performance engineering posture beyond unit/integration/property testing.
- The target operating model must include:
  - benchmarking guidance or hooks
  - load testing guidance or hooks
  - soak testing guidance or hooks
  - latency budget awareness
  - performance regression detection
  - capacity planning hooks or conventions

## Non-Functional Verification

- The framework must define a standard non-functional verification model in addition to functional tests.
- That model must include:
  - performance tests
  - soak tests
  - resilience and failure-injection tests
  - security tests
  - compatibility and migration tests
  - concurrency race and ordering suites
- Long-running or expensive suites do not need to run on every local fast path, but they must have:
  - standard entry points
  - documented environments
  - explicit ownership in the plan and DoD
- Compatibility and migration testing must cover framework evolution where shared contracts or state transitions are promised.
- Concurrency verification must cover race detection, ordering guarantees where promised, and lock or contention behavior for distributed runtimes.

## Current Implementation Docs

When you update package-level or `docs/` documentation during the refactor:

- keep current implementation details accurate
- clearly mark them as current/transitional where they would otherwise be read as target architecture
- link back to this document when there is a risk of new code following the old layout
