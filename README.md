# Nimburion

Production-ready Go framework for microservices with strong defaults for security, observability, and operations.

> Architecture status: the repository is in an active refactor from the current package layout toward a more modular `core + features/modules` design. The packages under `pkg/server`, `pkg/store`, `pkg/configschema`, `pkg/migrate`, and parts of `pkg/controller` document the current implementation, not the long-term extension points for new code. For new work, follow [docs/refactoring-requirements.md](./docs/refactoring-requirements.md).

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
The current implementation can be bootstrapped with `pkg/server` + `pkg/config` to run public/management servers with graceful shutdown. That bootstrap path is being refactored; see [docs/refactoring-requirements.md](./docs/refactoring-requirements.md) before adding new framework code.

## Testing
```bash
# Fast unit tests (no external dependencies)
make test-fast

# All tests with Docker services
make test-integration
```

See [Testing Guide](./docs/testing.md) for details.

## Configuration
- Priority in the current implementation: `ENV > secrets file > config file > defaults`
- Prefix: `APP_`
- Full reference for the current implementation: [Configuration guide](./docs/configuration.md), [Secrets](./docs/configuration-secrets.md)

## Current Package Map
- `pkg/server`, `pkg/server/router`: server lifecycle and routing
- `pkg/config`, `pkg/configschema`: config loading and schema helpers
- `pkg/middleware`, `pkg/auth`: transport security and request pipeline
- `pkg/store`, `pkg/repository`: data access and adapters
- `pkg/eventbus`, `pkg/jobs`, `pkg/realtime`: async messaging, jobs runtime, and realtime channels
- `pkg/observability`, `pkg/health`, `pkg/resilience`: runtime operations
- `pkg/cli`, `pkg/migrate`, `pkg/openapi` (via `pkg/server/openapi`): tooling

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
