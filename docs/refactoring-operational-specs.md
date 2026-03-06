# Refactoring Operational Specs

This document turns a few enterprise-grade refactoring goals into minimum operational contracts.

It complements:

- [refactoring-requirements.md](./refactoring-requirements.md)
- [refactoring-design.md](./refactoring-design.md)
- [refactoring-plan.md](./refactoring-plan.md)

These specs are intentionally narrow. They define the minimum framework contract that must exist before implementation can claim enterprise posture in these areas.

## Tamper-Aware Audit Minimum Contract

Tamper-aware audit is not ordinary structured logging.

The framework contract must provide:

- an append-only audit record model
- explicit audit streams
- record integrity metadata
- verification semantics
- required-vs-best-effort emission policy

### Audit Record Shape

Each framework-managed audit record must include at least:

- `stream_id`
- `sequence`
- `record_id`
- `occurred_at`
- `recorded_at`
- `category`
- `action`
- `outcome`
- `actor`
- `subject`
- `tenant_id` when tenant-aware
- `region` when runtime region identity exists
- `retention_class`
- `classification`
- `prev_digest`
- `record_digest`
- `key_id` when signing or keyed hashing is enabled

The record digest must be computed over a canonical serialized representation of the record excluding `record_digest` itself.

### Stream Semantics

The contract must define how audit streams are partitioned. A valid default is:

- one logical stream per application plus sink-defined partitions
- monotonic `sequence` inside a stream
- `prev_digest` chaining inside a stream

If a sink partitions further by tenant, region, or workload, the stream identity must stay observable.

### Sink Contract

`pkg/audit` must expose a sink contract with these minimum guarantees:

- append-only write semantics in the framework contract path
- ability to reject or fail writes explicitly
- ability to preserve `stream_id`, `sequence`, and digest metadata
- ability to surface verification or checkpoint metadata where supported

The framework does not need to guarantee that every vendor sink is physically immutable, but it must guarantee that framework-managed audit writes are modeled as append-only records with integrity metadata.

### Verification Contract

The framework must provide a verifier contract or verifier-compatible artifact model that can detect at least:

- missing records within a stream
- out-of-order records within a stream
- modified records within a stream
- stream restarts without an explicit checkpoint or rollover marker

Verification may be online, offline, or exported to tooling, but it must be defined as a shared contract rather than left to sink-specific interpretation.

### Failure Policy

Audit emission must support at least these modes:

- `required`
- `best_effort`

Security-sensitive framework actions should default to `required`.

When a `required` audit write fails, the contract must define one of these outcomes explicitly:

- fail the protected action closed
- surface the runtime as degraded or not-ready
- both, when policy requires it

The framework must not silently downgrade a required audit write to best-effort logging.

## Supply-Chain Posture Minimum Contract

Supply-chain posture must be treated as a release contract, not only a documentation concern.

The framework must define:

- required release artifacts
- dependency policy gates
- provenance expectations
- failure behavior for missing evidence

### Required Release Artifacts

For every official framework release, the minimum release artifact set should include:

- source revision identity
- build metadata
- dependency manifest evidence
- SBOM or equivalent dependency inventory
- vulnerability scan result or policy evaluation result
- checksum manifest
- provenance or attestation artifact when the release process supports it

The exact tooling can evolve. The artifact categories cannot disappear silently.

### Dependency Policy

The framework release process must define policy for:

- direct dependency review
- transitive dependency visibility
- vulnerability severity thresholds
- exception and allowlist recording
- toolchain version pinning policy

The framework does not need to solve organization-wide policy, but it must not ship with no dependency posture at all.

### Release Gating

A framework release must fail policy evaluation when required supply-chain evidence is missing, stale, or out of policy.

Minimum gating examples:

- missing SBOM or equivalent inventory
- missing checksum manifest
- missing provenance artifact when provenance is required for that release path
- unresolved vulnerability findings above the accepted policy threshold without a documented exception

### Runtime And Tooling Visibility

At minimum, the framework should make these values observable:

- framework version
- build metadata
- descriptor compatibility metadata where relevant

This allows downstream tooling such as `nimbctl`, deployment pipelines, or internal catalogs to reason about release posture without scraping arbitrary files.

## Multi-Region And Disaster-Recovery Minimum Contract

Multi-region and DR posture must be explicit capability metadata, not only narrative guidance.

The framework must define:

- supported posture levels
- runtime recovery states
- replay and idempotency assumptions
- validation rules for unsupported topologies

### Posture Levels

The shared contract should distinguish at least:

- `single_region_only`
- `single_writer_multi_region_failover`
- `multi_region_active_passive`
- `multi_region_active_active_subset`

Not every feature must support every level. Unsupported levels must stay explicit.

### Runtime Metadata

Where the runtime exposes deployment posture or publishes orchestration metadata, it must be able to express at least:

- region scope
- writer topology
- replay requirement after failover
- external durable dependency requirement
- leader-lock locality requirement

These values belong in runtime contracts and orchestration metadata, not only runbooks.

### Recovery States

The runtime model should support explicit recovery-oriented states when the feature participates in failover-sensitive flows:

- `normal`
- `failover_pending`
- `replay_required`
- `reconciling`
- `degraded_recovery`

Features that do not participate in recovery orchestration do not need to surface every state, but they must not imply full multi-region safety by omission.

### Reliability Coupling

When a feature claims a multi-region or failover-safe posture, the contract must also define:

- idempotency expectations for replayed requests, jobs, or events
- ordering scope, such as per key, per partition, or local-only
- deduplication boundary
- scheduler or leader-election behavior across regions

The framework must not imply cross-region exactly-once guarantees when only at-least-once replay with deduplication is available.

### Validation Rules

The framework must define validation failures for unsupported combinations, for example:

- a runtime marked multi-region safe while using a single-instance-only coordination model
- a failover-capable deployment without required durable state dependencies
- cross-region replay claims without an idempotency or deduplication contract

## gRPC Runtime Minimum Contract

gRPC must have an explicit operational contract, not only a package-level design.

The framework must define:

- listener startup and readiness semantics
- unary and streaming drain behavior
- deadline and cancellation behavior
- degraded-mode participation
- reflection exposure policy
- transport-security operational defaults
- health reporting strategy
- observability dimensions

### Listener Startup And Readiness

When the gRPC family is enabled, the runtime contract must define at least:

- when the listener is considered started
- when the listener is considered ready to serve traffic
- whether readiness depends only on bind success or also on required dependency readiness
- how gRPC runtime readiness composes with application-level readiness

A valid default is:

- listener bind success moves the transport into `started`
- readiness remains false until hard dependencies and required startup hooks are healthy
- optional dependency failures may move the runtime into `degraded` without forcing `not_ready`

### Graceful Drain

Graceful shutdown semantics must distinguish unary and streaming calls.

For unary calls, the minimum contract is:

- stop accepting new calls once shutdown begins
- allow in-flight calls to complete until deadline or shutdown timeout
- surface timeout-driven termination explicitly

For streaming calls, the minimum contract is:

- stop accepting new streams once shutdown begins
- define whether existing streams are allowed to drain, receive a cutoff signal, or are force-closed at timeout
- make the cutoff policy explicit for server-streaming, client-streaming, and bidirectional-streaming flows

The framework must not imply identical shutdown behavior for unary and streaming RPCs.

### Deadline And Cancellation

The runtime contract must define:

- propagation of client deadlines into request-scoped runtime context
- propagation of client cancellation into handler context
- framework behavior when downstream work ignores cancellation
- transport-classified outcomes for deadline-exceeded vs canceled requests

If the framework exposes helper abstractions around gRPC handlers, those abstractions must preserve deadline and cancellation semantics rather than masking them.

### Degraded Mode

When optional dependencies fail, the gRPC runtime contract must define whether:

- the gRPC listener remains ready
- the service reports degraded state through health or introspection
- only a subset of methods are operationally impaired

The framework must distinguish:

- `not_ready` because a hard dependency blocks safe service startup
- `degraded` because optional capabilities are unavailable

### Reflection Exposure Policy

Reflection is optional and environment-sensitive.

The operational contract must define at least:

- whether reflection is enabled by environment or profile
- whether reflection is disabled by default in production-oriented profiles
- whether reflection status is observable through debug or runtime introspection

Reflection must not become a mandatory runtime assumption for gRPC applications.

### Transport Security Defaults

The gRPC family must define minimum operational defaults for:

- `insecure`
- `tls`
- `mtls`
- `external_termination`

Where `mtls` or peer identity is enabled, the contract must define:

- how peer identity is extracted
- what happens when presented identity is missing or invalid
- how rotation of trust material affects readiness or degraded status

### Health Reporting Strategy

The health contract must allow management exposure through:

- HTTP only
- gRPC only
- both HTTP and gRPC

The shared health registry remains the source of truth. Transport-specific surfaces expose the same health model through different transport contracts.

The operational contract must define:

- when the gRPC health service is exposed
- how readiness and liveness are represented when gRPC is the only management surface
- how duplicated HTTP and gRPC health surfaces remain semantically aligned when both exist

### Observability Dimensions

When gRPC is enabled, the minimum observability contract should include dimensions for:

- application or service name
- gRPC service name
- gRPC method name
- status code
- stream type:
  - unary
  - client_stream
  - server_stream
  - bidi_stream

If the framework adds latency or error metrics for gRPC, those metrics must use stable labels that do not assume HTTP route semantics.

## Success Criteria

These operational specs are successful when:

- audit tamper-awareness means more than "structured logs sent somewhere"
- supply-chain posture can be checked by release automation rather than inferred informally
- multi-region and DR posture can be reasoned about from contracts and metadata instead of tribal knowledge
- gRPC runtime behavior can be operated through explicit listener, drain, health, security, and observability contracts instead of transport-specific folklore
