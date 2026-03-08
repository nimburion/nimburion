# Testing Guide

This document describes the testing workflow and the stable verification lanes used on this branch.

## Quick Start

```bash
make test-build TEST_PKG=./pkg/core/...
make test-fast-lane TEST_PKG=./pkg/core/...
make test-integration-lane TEST_PKG=./pkg/persistence/relational/postgres/...
make test-contract-lane TEST_PKG=./pkg/http/router/...
make test-nonfunctional-lane TEST_PKG=./...
make test
make test-fast
make test-integration
make test-parallel
make test-coverage
make test-review-sweep
```

Use:

- `make test-build TEST_PKG=./path/...` for build-only verification of a touched area
- `make test-fast-lane TEST_PKG=./path/...` for the canonical fast gate on a touched area
- `make test-integration-lane TEST_PKG=./path/...` for the canonical integration gate
- `make test-contract-lane TEST_PKG=./path/...` for the canonical contract gate
- `make test-nonfunctional-lane TEST_PKG=./path/...` for the canonical non-functional gate
- `make test` for the default test runner
- `make test-fast` for fast feedback
- `make test-integration` when external services are required
- `make test-parallel` for a full parallel run
- `make test-coverage` for coverage output
- `make test-review-sweep` for the documented pre-merge production-readiness sweep

The lane wrappers are the stable task-level entry points for `TASKS.md`. They currently shell out to `go test` with standard selection patterns and can be replaced later by more specialized wrappers without changing task language.

## Pre-Merge Review Sweep

For this worktree, the pre-merge production-readiness sweep is:

```bash
env GOCACHE=.cache/go-build go vet ./...
env GOCACHE=.cache/go-build go test ./pkg/config ./pkg/cli
env GOCACHE=.cache/go-build go test ./pkg/http/session ./pkg/http/server ./pkg/http/httpsignature
env GOCACHE=.cache/go-build go test ./pkg/jobs ./pkg/scheduler ./pkg/http/openapi
env GOCACHE=.cache/go-build go test ./pkg/reliability/idempotency ./internal/emailkit
env GOCACHE=.cache/go-build go test ./pkg/persistence/relational ./pkg/persistence/relational/mysql ./pkg/persistence/relational/migrate ./pkg/persistence/keyvalue/dynamodb
env GOCACHE=.cache/go-build go test ./pkg/cache ./pkg/session
env GOCACHE=.cache/go-build go test ./pkg/cache/redis -run 'TestAdapter_MapGetError_NotFoundMapsToCacheMiss|TestAdapter_WithOperationTimeout|TestAdapter_Integration'
env GOCACHE=.cache/go-build go test ./pkg/session/redis -run 'TestAdapter_MapGetError_NotFoundMapsToSessionErrNotFound|TestAdapter_WithOperationTimeout|TestAdapter_Integration'
env GOCACHE=.cache/go-build go test ./pkg/persistence/relational/postgres -run TestAdapter_Integration
env GOCACHE=.cache/go-build go test ./...
```

Notes:

- `GOCACHE=.cache/go-build` keeps verification artifacts local to the worktree.
- Redis and PostgreSQL integration tests now skip cleanly when Docker is unavailable.
- The final `go test ./...` is the confidence sweep after the targeted regression packages pass.

## Current Test Modes

### Build Gate

Build verification for a touched area should use:

```bash
make test-build TEST_PKG=./path/...
```

This maps to `go test ./path/... -run '^$'` and is the canonical no-test compile gate for refactor tasks.

### Fast Tests

Fast tests are intended for normal local feedback.

They should avoid external service dependencies where possible.

Command:

```bash
make test-fast
make test-fast-lane TEST_PKG=./path/...
```

### Integration Tests

Integration tests are for packages that require external services such as Redis or PostgreSQL.

Command:

```bash
make test-integration
make test-integration-lane TEST_PKG=./path/...
```

If you need manual service startup outside the test runner:

```bash
docker compose -f docker-compose.test.yml up -d
docker compose -f docker-compose.test.yml down
```

### Parallel Tests

Use this when you want a faster full-repo run and your environment can support it.

Command:

```bash
make test-parallel
```

### Coverage

Command:

```bash
make test-coverage
```

## Current Test Categories

The current codebase contains a mix of:

- unit tests
- integration tests
- property tests

Non-functional verification is still incomplete in the current codebase and should be treated as an explicit refactor target, not as optional follow-up work.

Property tests remain important in this repo, especially for interchangeable contracts such as router behavior and other pluggable components.

## Refactor Direction

During the refactor, testing should move away from package-group thinking based on the old layout and toward **contract-oriented suites**.

The default gate vocabulary for all implementation tasks is:

- build
- fast
- integration
- contract
- non-functional

Task descriptions should reference those names directly and pair them with one of the stable wrapper commands in this document.

Priority test areas:

- `pkg/core`
- `pkg/cli`
- `pkg/config`
- `pkg/http/router`
- `pkg/http`
- `pkg/grpc`
- `pkg/persistence/*`
- `pkg/eventbus`
- `pkg/jobs`
- `pkg/scheduler`
- `pkg/coordination`
- `pkg/cache`
- `pkg/session`

The preferred direction is:

- keep unit tests close to the package that owns the behavior
- keep integration tests close to the adapter or family that requires them
- add contract suites for replaceable implementations
- add standard non-functional suites for runtime qualities that the framework promises
- avoid central giant enumerations of implementations when family-local contract suites are clearer

## Contract Tests To Preserve Or Expand

Run the contract gate with:

```bash
make test-contract-lane TEST_PKG=./path/...
```

The refactor should preserve and expand contract-style tests for:

- router implementations
- gRPC unary and streaming interceptor chains
- gRPC service registration and lifecycle integration
- event bus implementations
- jobs backends
- scheduler lock providers
- cache and session backends
- persistence family implementations where a common subset is promised

## gRPC Testing Additions

Add contract suites for:

- unary interceptor chains
- streaming interceptor chains
- service registration and lifecycle integration

Add validation-focused suites for:

- transport decode failures
- contract or schema failures
- domain validation failures
- status-code and detail mapping

Add runtime behavior suites for:

- deadline propagation
- cancellation propagation
- health and reflection enablement
- streaming backpressure behavior where the framework promises it
- ordering and exclusivity behavior for streaming handlers where the framework promises it

Add security-focused suites for:

- metadata credential handling
- mTLS and peer-identity integration
- tenant-context propagation through interceptors
- audit-context propagation through interceptors

Add benchmark, load, and soak coverage for:

- unary runtime paths
- streaming runtime paths

## Non-Functional Verification Categories

Run the non-functional gate with:

```bash
make test-nonfunctional-lane TEST_PKG=./path/...
```

Shared harness support lives in `internal/testharness/nonfunctional`.

Use `NIMB_NONFUNCTIONAL` to narrow expensive categories when a package owns long-running suites, for example:

```bash
NIMB_NONFUNCTIONAL=resilience,ordering make test-nonfunctional-lane TEST_PKG=./pkg/core/app
```

During and after the refactor, the test model should explicitly include:

- performance tests
- soak tests
- resilience and failure-injection tests
- security tests
- compatibility and migration tests
- concurrency race and ordering suites

The standard category names are:

- `performance`
- `load`
- `soak`
- `resilience`
- `security`
- `compatibility`
- `race`
- `ordering`

### Performance Tests

Use these for:

- critical hot paths
- latency-sensitive framework contracts
- regressions in shared abstractions

Typical examples:

- router or middleware overhead
- event publish and consume paths
- jobs enqueue and execution paths
- persistence family hot paths where the framework owns shared abstractions

### Soak Tests

Use these for:

- long-running workers
- scheduler loops
- retry-heavy consumers
- cache or coordination behavior under time

The goal is to surface:

- memory growth
- retry drift
- queue or lag accumulation
- long-run degradation

### Resilience And Failure-Injection Tests

Use these when the framework owns:

- degraded-mode transitions
- retry and poison handling
- dependency-loss behavior
- coordination or lock-loss handling

These tests should exercise standard failure-injection hooks instead of requiring ad hoc code patching.

### Security Tests

Use these for framework-owned security behavior such as:

- authn or authz contracts
- masking and redaction
- tenant isolation
- secret handling
- security event or audit emission where the framework owns the contract

### Compatibility And Migration Tests

Use these for:

- descriptor evolution
- config evolution
- migration policy changes
- startup behavior coupled to migration state

### Concurrency Race And Ordering Suites

Use these for:

- router and middleware ordering
- cancellation semantics
- jobs and event handler concurrency
- scheduler exclusivity
- lock-provider contention
- any framework contract that promises ordering or exclusivity

## Environment Notes

External-service tests may rely on environment variables such as:

- `REDIS_URL`
- `POSTGRES_URL`
- `INTEGRATION_TESTS`

Keep environment-driven integration setup explicit and local to the tests that need it.

## CI Guidance

As a working rule:

- fast checks should run on normal iteration paths
- full integration coverage should run in a dedicated environment with the required services
- non-functional suites may run on separate entry points or dedicated environments, but they must still be part of the framework quality model and release readiness

This document intentionally stays short; the source of truth for available test commands is the [Makefile](/Users/giuseppe/Workspace/github.com/nimburion/nimburion/Makefile).
