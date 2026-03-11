# Nimburion

Production-ready Go framework for microservices with strong defaults for security, observability, and operations.

> Architecture status: the repository uses the current package layout built around `pkg/core`, `pkg/http`, `pkg/grpc`, `pkg/persistence`, `pkg/cache`, `pkg/session`, and related families. For package boundaries and extension rules, follow [docs/architecture.md](./docs/architecture.md).

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
Applications bootstrap through `pkg/http/server` + `pkg/config` for public and management servers with graceful shutdown. See [docs/architecture.md](./docs/architecture.md) before adding new framework code.

## Testing
```bash
# Verification Lanes
make test-build TEST_PKG=./pkg/core/...
make test-fast-lane TEST_PKG=./pkg/core/...
make test-contract-lane TEST_PKG=./pkg/http/router/...

# Fast unit tests (no external dependencies)
make test-fast

# All tests with Docker services
make test-integration
```

Use the `*-lane` wrappers for focused verification and the aggregate targets for broad repo runs. See [Testing Guide](./docs/testing.md) for details.

## Configuration
- Priority: `ENV > secrets file > config file > defaults`
- Prefix: `APP_`
- Full reference: [Configuration guide](./docs/configuration.md)

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

## Architecture Guardrails

Do not add new framework code under removed or superseded legacy roots.

Place HTTP response/input work in `pkg/http/response` and `pkg/http/input`, keep application errors in `pkg/core/errors`, place new transport code in `pkg/http/*` or `pkg/grpc/*`, keep `pkg/core` transport-agnostic, and prefer target families such as `pkg/persistence/*`, `pkg/cache/*`, `pkg/session/*`, `pkg/coordination/*`, and `pkg/config/schema`.

## Documentation
- Wiki: <https://github.com/nimburion/nimburion.github.com/wiki>
- Nimburion wiki section: <https://github.com/nimburion/nimburion.github.com/wiki/nimburion>
- Repo docs: `docs/` and package-level `README.md` files under `pkg/`
- Architecture overview: [docs/architecture.md](./docs/architecture.md)
- Operational contracts: [docs/operational-specs.md](./docs/operational-specs.md)
- Configuration: [docs/configuration.md](./docs/configuration.md)
- Testing and lint workflow: [docs/testing.md](./docs/testing.md)
- Error model: [docs/error-model.md](./docs/error-model.md)
- Historical refactor material: [docs/archive/README.md](./docs/archive/README.md)
