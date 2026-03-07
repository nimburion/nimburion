# Nimburion

Production-ready Go framework for microservices with strong defaults for security, observability, and operations.

> Architecture status: this branch uses the refactored package layout built around `pkg/core`, `pkg/http`, `pkg/persistence`, `pkg/cache`, `pkg/session`, and related target-state families. Config schema ownership now lives in `pkg/config/schema`; for package boundaries and remaining guardrails, follow [docs/refactoring-requirements.md](./docs/refactoring-requirements.md).

## Value Proposition
- Build services faster with reusable platform modules.
- Keep architecture and operational contracts consistent.
- Ship production-ready services with less boilerplate.

## Core Capabilities
- Dual HTTP servers (`public` + `management`)
- AuthN/AuthZ (OAuth2/OIDC JWT + scopes)
- Pluggable adapters (store, cache, search, event bus, email)
- Observability (structured logs, metrics, tracing)
- Resilience (timeout, rate limit, circuit breaker, retry)
- OpenAPI generation and request validation

## Quick Start
```bash
go get github.com/nimburion/nimburion
```
Applications on this branch bootstrap through `pkg/http/server` + `pkg/config` for public/management servers with graceful shutdown. See [docs/refactoring-requirements.md](./docs/refactoring-requirements.md) before adding new framework code.

## Testing
```bash
# Task-scoped verification lanes during the refactor
make test-build TEST_PKG=./pkg/core/...
make test-fast-lane TEST_PKG=./pkg/core/...
make test-contract-lane TEST_PKG=./pkg/http/router/...

# Fast unit tests (no external dependencies)
make test-fast

# All tests with Docker services
make test-integration
```

Use the `*-lane` wrappers for task-scoped verification during the refactor and the aggregate targets for broad repo runs. See [Testing Guide](./docs/testing.md) for details.

## Configuration
- Priority: `ENV > secrets file > config file > defaults`
- Prefix: `APP_`
- Full reference: [Configuration guide](./docs/configuration.md), [Secrets](./docs/configuration-secrets.md)

## Current Package Map
- `pkg/http/server`, `pkg/http/router`: server lifecycle and HTTP routing
- `pkg/core/app`, `pkg/featureflag`: application lifecycle, rollout-safe flags, and runtime posture
- `pkg/config`, `pkg/config/schema`: config loading and schema helpers
- `pkg/http/*`, `pkg/auth`: transport security and request pipeline
- `pkg/persistence/*`: relational, document, key-value, search, and object families
- `pkg/cache/*`, `pkg/session/*`: shared operational roles and role-specific backend adapters
- `pkg/eventbus`, `pkg/jobs`, `pkg/realtime`: async messaging, jobs runtime, and realtime channels
- `pkg/observability`, `pkg/health`, `pkg/resilience`: runtime operations and shared health semantics
- `pkg/cli`, `pkg/persistence/relational/migrate`, `pkg/http/openapi`: tooling and HTTP API documentation

## Refactor Guardrails

Until the refactor lands, do not add new framework code under:

- `pkg/config/schema`

`pkg/controller` is already removed on this branch. Place HTTP response/input work in `pkg/http/response` and `pkg/http/input`, keep application errors in `pkg/core/errors`, place new transport code in `pkg/http/*` or `pkg/grpc/*`, keep `pkg/core` transport-agnostic, and prefer target-state families such as `pkg/persistence/*`, `pkg/cache/*`, `pkg/session/*`, and `pkg/config/schema` over legacy package roots.

Target direction for new framework code is tracked in [docs/refactoring-requirements.md](./docs/refactoring-requirements.md), the package design is in [docs/refactoring-design.md](./docs/refactoring-design.md), the execution order is tracked in [docs/refactoring-plan.md](./docs/refactoring-plan.md), the user interactions are tracked in [docs/refactoring-user-stories.md](./docs/refactoring-user-stories.md), and story traceability is tracked in [docs/refactoring-traceability.md](./docs/refactoring-traceability.md).

## Documentation
- Wiki: <https://github.com/nimburion/nimburion.github.com/wiki>
- Nimburion wiki section: <https://github.com/nimburion/nimburion.github.com/wiki/nimburion>
- Repo docs: `docs/` and package-level `README.md` files under `pkg/`
- Architecture transition and target requirements: [docs/refactoring-requirements.md](./docs/refactoring-requirements.md)
- Target architecture design: [docs/refactoring-design.md](./docs/refactoring-design.md)
- Operational specs for enterprise controls: [docs/refactoring-operational-specs.md](./docs/refactoring-operational-specs.md)
- Detailed execution plan: [docs/refactoring-plan.md](./docs/refactoring-plan.md)
- User stories and interactions: [docs/refactoring-user-stories.md](./docs/refactoring-user-stories.md)
- Traceability matrix: [docs/refactoring-traceability.md](./docs/refactoring-traceability.md)
- nimbctl alignment plan: [docs/nimbctl-alignment-plan.md](./docs/nimbctl-alignment-plan.md)
- nimbctl service descriptor design: [docs/nimbctl-service-descriptor-design.md](./docs/nimbctl-service-descriptor-design.md)
