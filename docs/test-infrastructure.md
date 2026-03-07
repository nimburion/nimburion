# Test Infrastructure

This document describes the test infrastructure used by the repo and the expected direction for remaining verification work.

## Current Infrastructure

The repo currently relies on:

- `docker-compose.test.yml` for local external services
- `scripts/test.sh` as the main test runner
- package-local tests, including unit, integration, and property tests

Relevant files:

- [docker-compose.test.yml](/Users/giuseppe/Workspace/github.com/nimburion/nimburion/docker-compose.test.yml)
- [scripts/test.sh](/Users/giuseppe/Workspace/github.com/nimburion/nimburion/scripts/test.sh)
- [Makefile](/Users/giuseppe/Workspace/github.com/nimburion/nimburion/Makefile)

## Current Goals

The current setup should provide:

- fast local feedback for normal development
- explicit integration coverage for external-service adapters
- repeatable local execution without depending on random developer machine state

## Operating Model

Use:

- `make test-build TEST_PKG=./path/...` for build-only verification of a touched area
- `make test-fast-lane TEST_PKG=./path/...` for task-level fast verification
- `make test-integration-lane TEST_PKG=./path/...` for task-level integration verification
- `make test-contract-lane TEST_PKG=./path/...` for family contract verification
- `make test-nonfunctional-lane TEST_PKG=./path/...` for non-functional verification
- `internal/testharness/nonfunctional` for standard category gating and naming
- `make test-fast` for quick feedback
- `make test-integration` for external-service coverage
- `make test-parallel` for full parallel runs

These wrappers are the current stable lane entry points for `TASKS.md`. They intentionally accept `TEST_PKG=./path/...` so later waves can keep the same task contract while evolving the underlying implementation.

When needed, local services can be started through:

```bash
docker compose -f docker-compose.test.yml up -d
docker compose -f docker-compose.test.yml down
```

## Refactor Direction

As the architecture moves away from the old package layout, test infrastructure should also stop being described in terms of old package groups such as:

- server
- store
- middleware

The preferred direction is:

- infrastructure by required dependency, not by old package bucket
- contract suites per family or role
- adapter-local integration tests
- explicit non-functional verification harnesses where the framework promises runtime qualities
- minimal shared orchestration only where it actually reduces duplication

## Priority Infrastructure Areas

The most important infrastructure support during the refactor is for:

- relational adapters
- Redis-backed roles
- gRPC runtimes and interceptor chains
- event bus adapters
- scheduler lock providers

The most important additional non-functional support is for:

- performance and benchmark harnesses for shared hot paths
- soak environments for long-running workers, schedulers, and retry-heavy consumers
- failure-injection harnesses for dependency loss, lock loss, and degraded-mode verification
- security-focused suites where the framework owns auth, masking, tenant, or secret-handling behavior
- compatibility and migration suites for descriptor, config, and startup-policy evolution
- race and ordering verification for concurrent runtimes

Current branch state:

- shared non-functional harness helpers live in `internal/testharness/nonfunctional`
- `NIMB_NONFUNCTIONAL` can restrict category execution without changing the lane command shape

These areas are the most likely to need external dependencies and contract verification.

## gRPC Testing Additions

The infrastructure plan should explicitly support gRPC verification for:

- contract suites for unary interceptor behavior
- contract suites for streaming interceptor behavior
- service registration and lifecycle integration
- validation-layer distinction tests for transport decode, contract/schema, and domain validation failures
- deadline and cancellation propagation suites
- status-mapping suites
- health and reflection enablement tests
- race and ordering suites for streaming handlers where ordering or exclusivity is promised
- security-focused suites for metadata credentials, mTLS or peer identity, and tenant or audit-context propagation
- benchmark, load, and soak coverage for unary and streaming critical paths

These suites do not imply that every local fast path must start a gRPC stack by default, but they do require stable entry points and repeatable environments when gRPC becomes a main public family.

## Guardrails

- Keep the default fast path lightweight.
- Do not make normal unit feedback depend on Docker by default.
- Add shared infrastructure only when more than one package family actually benefits from it.
- Prefer explicit setup over hidden global test magic.
- Keep long-running or expensive suites off the default fast path, but give them standard documented entry points and stable environments.
- Treat race, ordering, migration, and security verification as standard framework quality gates where the framework owns those contracts.
- Prefer family-scoped lanes over old package-bucket lanes such as `test-group-server` or `test-group-store` when defining new refactor tasks.
