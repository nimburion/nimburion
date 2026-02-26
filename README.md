# Nimburion

Production-ready Go framework for microservices with strong defaults for security, observability, and operations.

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
Use `pkg/server` + `pkg/config` to bootstrap public/management servers and run with graceful shutdown.

## Testing
```bash
# Fast unit tests (no external dependencies)
make test-fast

# All tests with Docker services
make test-integration
```

See [Testing Guide](./docs/testing.md) for details.

## Configuration
- Priority: `ENV > config file > defaults`
- Prefix: `APP_`
- Full reference: [Configuration guide](./docs/configuration.md), [Secrets](./docs/configuration-secrets.md)

## Package Map
- `pkg/server`, `pkg/server/router`: server lifecycle and routing
- `pkg/config`, `pkg/configschema`: config loading and schema helpers
- `pkg/middleware`, `pkg/auth`: transport security and request pipeline
- `pkg/store`, `pkg/repository`: data access and adapters
- `pkg/eventbus`, `pkg/jobs`, `pkg/realtime`: async messaging, jobs runtime, and realtime channels
- `pkg/observability`, `pkg/health`, `pkg/resilience`: runtime operations
- `pkg/cli`, `pkg/migrate`, `pkg/openapi` (via `pkg/server/openapi`): tooling

## Documentation
- Wiki: <https://github.com/nimburion/nimburion.github.com/wiki>
- Nimburion wiki section: <https://github.com/nimburion/nimburion.github.com/wiki/nimburion>
- Repo docs: `docs/` and package-level `README.md` files under `pkg/`
