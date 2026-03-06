# Refactoring Traceability Matrix

This document maps user stories to the requirement areas, design sections, and primary implementation milestones that realize them.

It complements:

- [refactoring-user-stories.md](./refactoring-user-stories.md)
- [refactoring-requirements.md](./refactoring-requirements.md)
- [refactoring-design.md](./refactoring-design.md)
- [refactoring-plan.md](./refactoring-plan.md)

## Usage

- Use the `Story` column as the stable story identifier.
- The `Requirements` column references the relevant section names in [refactoring-requirements.md](./refactoring-requirements.md).
- The `Design` column references the relevant section names in [refactoring-design.md](./refactoring-design.md).
- The `Primary milestone` column identifies the main implementation milestone in [refactoring-plan.md](./refactoring-plan.md). Some stories will naturally span follow-up milestones too.

## Epic 1: Application Bootstrap And CLI

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `1.1` | Shared CLI standard for all projects | `Architectural Direction` | `Runtime Composition Model`, `CLI Design` | `Milestone 1: Core Runtime And CLI` |
| `1.2` | Non-HTTP component uses same CLI and `Run` model | `Architectural Direction`, `Packaging Rules For New Code` | `Runtime Composition Model`, `CLI Design` | `Milestone 1: Core Runtime And CLI` |
| `1.3` | Explicit CLI lifecycle hooks | `Architectural Direction` | `CLI Design` | `Milestone 1: Core Runtime And CLI` |

## Epic 2: Configuration, Schema, And Secrets

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `2.1` | Compose only the config sections used by the application | `Configuration And Secrets`, `Packaging Rules For New Code` | `Config And Schema Design` | `Milestone 2: Config, Schema, And Secrets` |
| `2.2` | Predictable config precedence | `Configuration And Secrets` | `Config And Schema Design` | `Milestone 2: Config, Schema, And Secrets` |
| `2.3` | Keep `secrets file` now, support managed providers later | `Configuration And Secrets` | `Config And Schema Design` | `Milestone 2: Config, Schema, And Secrets` |
| `2.4` | Reliable secret redaction | `Configuration And Secrets` | `Config And Schema Design` | `Milestone 2: Config, Schema, And Secrets` |

## Epic 3: HTTP Applications

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `3.1` | HTTP is opt-in | `Architectural Direction`, `Packaging Rules For New Code` | `Target Package Topology`, `HTTP Family Design` | `Milestone 3: HTTP Family Extraction` |
| `3.2` | Swap router implementation without rewriting handlers | `Architectural Direction` | `HTTP Family Design` | `Milestone 3: HTTP Family Extraction` |
| `3.3` | Layered request validation | `Validation Model` | `HTTP Family Design` | `Milestone 3: HTTP Family Extraction` |
| `3.4` | OpenAPI and similar as providers, not family definition | `Validation Model` | `HTTP Family Design` | `Milestone 3: HTTP Family Extraction` |

## Epic 3B: gRPC Applications

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `3B.1` | gRPC is opt-in | `Architectural Direction`, `Packaging Rules For New Code`, `gRPC Transport Family` | `Target Package Topology`, `gRPC Family Design`, `Runtime Composition Model` | `Milestone 3B: gRPC Family Extraction` |
| `3B.2` | Unary and streaming RPCs are modeled explicitly | `gRPC Transport Family` | `gRPC Family Design` | `Milestone 3B: gRPC Family Extraction` |
| `3B.3` | Interceptors and transport glue stay inside the gRPC family | `Packaging Rules For New Code`, `gRPC Transport Family` | `gRPC Family Design`, `Shared Capabilities` | `Milestone 3B: gRPC Family Extraction` |
| `3B.4` | Layered gRPC validation | `Validation Model`, `gRPC Transport Family` | `gRPC Family Design` | `Milestone 3B: gRPC Family Extraction` |
| `3B.5` | gRPC runtime integrates with shared health and lifecycle | `Architectural Direction`, `gRPC Transport Family` | `gRPC Family Design`, `Runtime Composition Model`, `CLI Design` | `Milestone 3B: gRPC Family Extraction` |
| `3B.6` | gRPC transport security stays transport-specific while policy stays reusable | `Enterprise Security Controls`, `gRPC Transport Family` | `gRPC Family Design`, `Security, Policy, Tenant, And Audit Design` | `Milestone 3B: gRPC Family Extraction` and `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |
| `3B.7` | Descriptor and CLI understand gRPC as a first-class family | `Architectural Direction`, `Descriptor And Tooling Alignment`, `gRPC Transport Family` | `CLI Design`, `gRPC Family Design` | `Milestone 3B: gRPC Family Extraction` |

## Epic 4: Eventing, Jobs, Scheduler, And Realtime

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `4.1` | Keep durable eventing separate from ephemeral pub/sub | `Architectural Direction`, `Persistence And Roles` | `Eventing Design` | `Milestone 6: Eventing, Jobs, Scheduler, And Coordination` |
| `4.2` | Layered event validation | `Validation Model` | `Eventing Design` | `Milestone 6: Eventing, Jobs, Scheduler, And Coordination` |
| `4.3` | Schema provider independent from broker | `Validation Model` | `Eventing Design` | `Milestone 6: Eventing, Jobs, Scheduler, And Coordination` |
| `4.4` | Jobs remain distinct from event bus messages | `Architectural Direction`, `Persistence And Roles` | `Eventing Design` | `Milestone 6: Eventing, Jobs, Scheduler, And Coordination` |
| `4.5` | Preserve scheduler multi-instance safety | `Architectural Direction` | `Eventing Design`, `Cache, Session, Coordination, And Other Roles` | `Milestone 6: Eventing, Jobs, Scheduler, And Coordination` |
| `4.6` | Move SSE and WebSocket under HTTP | `Packaging Rules For New Code` | `HTTP Family Design` | `Milestone 3: HTTP Family Extraction` |

## Epic 5: Persistence Families And Operational Roles

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `5.1` | Remove generic `pkg/store` umbrella | `Packaging Rules For New Code`, `Persistence And Roles` | `Persistence Design`, `Architecture Guardrails` | `Milestone 5: Persistence Families` |
| `5.2` | SQL portability belongs to relational family | `Persistence And Roles` | `Persistence Design` | `Milestone 5: Persistence Families` |
| `5.3` | Keep data families conceptually separate | `Persistence And Roles` | `Persistence Design` | `Milestone 5: Persistence Families` |
| `5.4` | Model Redis and Memcached by role | `Persistence And Roles`, `Packaging Rules For New Code` | `Cache, Session, Coordination, And Other Roles` | `Milestone 4: HTTP Security, Middleware, Cache, And Session` and `Milestone 5: Persistence Families` |
| `5.5` | Cache includes invalidation and freshness policies | `Persistence And Roles` | `Cache, Session, Coordination, And Other Roles` | `Milestone 4: HTTP Security, Middleware, Cache, And Session` |

## Epic 6: Health, Observability, And Debugging

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `6.1` | Health reflects only enabled features actually in use | `Architectural Direction` | `Runtime Composition Model`, `CLI Design` | `Milestone 1: Core Runtime And CLI` |
| `6.2` | Opinionated observability bootstrap without lock-in | `Architectural Direction` | `CLI Design`, `Shared Capabilities` | `Milestone 1: Core Runtime And CLI` |
| `6.3` | Framework introspection only in debug mode | `Architectural Direction` | `CLI Design`, `Runtime Composition Model` | `Milestone 1: Core Runtime And CLI` |

## Epic 7: Security, Policy, Tenant Isolation, And Cross-Cutting Capabilities

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `7.1` | Keep HTTP security inside HTTP family | `Packaging Rules For New Code` | `HTTP Family Design`, `Target Package Topology` | `Milestone 4: HTTP Security, Middleware, Cache, And Session` |
| `7.2` | Keep resilience public and reusable | `Architectural Direction` | `Shared Capabilities` | `Milestone 10: Shared Capabilities And Cleanup` |
| `7.3` | Support authorization providers beyond transport-specific code | `Enterprise Security Controls` | `Security, Policy, Tenant, And Audit Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |
| `7.4` | Make tenant identity explicit and reusable | `Enterprise Security Controls`, `Data Governance And Compliance` | `Security, Policy, Tenant, And Audit Design`, `Data Governance And Compliance Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |
| `7.5` | Model secret and certificate rotation as lifecycle concerns | `Configuration And Secrets`, `Enterprise Security Controls` | `Config And Schema Design`, `Security, Policy, Tenant, And Audit Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |
| `7.6` | Emit security events and tamper-aware audit records | `Enterprise Security Controls`, `Data Governance And Compliance` | `Security, Policy, Tenant, And Audit Design`, `Data Governance And Compliance Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |
| `7.7` | Reuse PII classification and masking across surfaces | `Enterprise Security Controls`, `Data Governance And Compliance` | `Security, Policy, Tenant, And Audit Design`, `Data Governance And Compliance Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |
| `7.8` | Enforce supply-chain release posture with explicit evidence and gates | `Enterprise Security Controls` | `Security, Policy, Tenant, And Audit Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` |

## Epic 8: Distributed Reliability And Consistency

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `8.1` | First-class idempotency and deduplication contracts | `Distributed Reliability And Consistency` | `Distributed Reliability Design` | `Milestone 7: Distributed Reliability And Consistency` |
| `8.2` | Standardize inbox and outbox patterns | `Distributed Reliability And Consistency` | `Distributed Reliability Design` | `Milestone 7: Distributed Reliability And Consistency` |
| `8.3` | Standardize retry taxonomy, budgets, and poison handling | `Distributed Reliability And Consistency` | `Distributed Reliability Design` | `Milestone 7: Distributed Reliability And Consistency` |
| `8.4` | Surface backpressure and consumer lag through framework signals | `Distributed Reliability And Consistency`, `Enterprise Operations` | `Distributed Reliability Design`, `Operations And Runtime Posture Design` | `Milestone 7: Distributed Reliability And Consistency` |
| `8.5` | Make saga and process-manager orchestration explicit | `Distributed Reliability And Consistency` | `Distributed Reliability Design` | `Milestone 7: Distributed Reliability And Consistency` |

## Epic 9: Enterprise Operations And Runtime Posture

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `9.1` | Define shared SLI, SLO, golden-signal, and alertability conventions | `Enterprise Operations` | `Operations And Runtime Posture Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `9.2` | Make startup, readiness, liveness, and degraded state explicit | `Enterprise Operations` | `Runtime Composition Model`, `Operations And Runtime Posture Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `9.3` | Support feature flags and config rollout safety | `Enterprise Operations` | `Shared Capabilities`, `Operations And Runtime Posture Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `9.4` | Expose failure-injection hooks for resilience testing | `Enterprise Operations` | `Operations And Runtime Posture Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `9.5` | Keep multi-region and disaster-recovery posture visible in the design | `Enterprise Operations`, `Distributed Reliability And Consistency` | `Distributed Reliability Design`, `Operations And Runtime Posture Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |

## Epic 10: Data Governance And Compliance

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `10.1` | Support auditability and retention-aware behavior | `Data Governance And Compliance`, `Enterprise Security Controls` | `Security, Policy, Tenant, And Audit Design`, `Data Governance And Compliance Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `10.2` | Provide hooks for erasure and compliance-driven deletion workflows | `Data Governance And Compliance` | `Data Governance And Compliance Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `10.3` | Make tenant-aware storage, cache, and session contracts explicit | `Data Governance And Compliance`, `Enterprise Security Controls` | `Security, Policy, Tenant, And Audit Design`, `Data Governance And Compliance Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` and `Milestone 9: Enterprise Operations, Governance, And Performance` |

## Epic 11: Performance And Capacity Engineering

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `11.1` | Add benchmark and load-test hooks to main families | `Performance And Capacity Engineering` | `Performance Engineering Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `11.2` | Attach latency budgets and regression detection to critical paths | `Performance And Capacity Engineering` | `Performance Engineering Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `11.3` | Expose capacity-planning hooks and saturation signals | `Performance And Capacity Engineering`, `Enterprise Operations` | `Performance Engineering Design`, `Operations And Runtime Posture Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |

## Epic 12: Non-Functional Verification

| Story | Summary | Requirements | Design | Primary milestone |
| --- | --- | --- | --- | --- |
| `12.1` | Add standard performance and load-test entry points | `Performance And Capacity Engineering`, `Non-Functional Verification` | `Performance Engineering Design`, `Verification Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `12.2` | Add soak-test coverage for long-running runtimes | `Non-Functional Verification` | `Verification Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `12.3` | Add resilience and failure-injection suites | `Enterprise Operations`, `Non-Functional Verification` | `Operations And Runtime Posture Design`, `Verification Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `12.4` | Add standard security test suites | `Enterprise Security Controls`, `Non-Functional Verification` | `Security, Policy, Tenant, And Audit Design`, `Verification Design` | `Milestone 8: Enterprise Security, Policy, Tenancy, And Audit` and `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `12.5` | Add compatibility and migration suites | `Non-Functional Verification`, `Configuration And Secrets` | `Verification Design`, `Config And Schema Design` | `Milestone 9: Enterprise Operations, Governance, And Performance` |
| `12.6` | Add concurrency race and ordering suites | `Non-Functional Verification`, `Distributed Reliability And Consistency` | `Verification Design`, `Distributed Reliability Design` | `Milestone 7: Distributed Reliability And Consistency` and `Milestone 9: Enterprise Operations, Governance, And Performance` |

## Change Management Rule

When a new implementation task is created:

- reference one or more story IDs from [refactoring-user-stories.md](./refactoring-user-stories.md)
- check the mapped requirement and design sections
- attach the task to the primary milestone from this matrix

If a task cannot be mapped to a story, it should be treated as one of:

- missing story coverage
- accidental scope creep
- internal technical subtask of an already-mapped story
