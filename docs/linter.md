# Lint Workflow And Architectural Review

Use this document as a short reference for the current lint workflow.

It is not a changelog of past cleanup work and it is not a script catalog.

## Commands

```bash
make lint
make lint-fix
make lint-critical
```

Use:

- `make lint` for the normal lint pass
- `make lint-fix` for auto-fixable issues
- `make lint-critical` when you want to focus on the most important security or correctness findings

## Manual Review Areas

Auto-fix is not enough for these categories:

- exported API comments
- potential hardcoded secrets
- file path handling and unsafe file access
- error handling around I/O, shutdown, and cleanup
- structural refactors triggered by lint suggestions

## Refactor Note

During the architecture refactor, prefer fixing lint issues in packages that define the target public contracts first:

- `pkg/core`
- `pkg/cli`
- `pkg/config`
- `pkg/http`
- `pkg/persistence/*`
- `pkg/eventbus`, `pkg/jobs`, `pkg/scheduler`, `pkg/coordination`, `pkg/pubsub`

## Rules

- Do not add comments only to silence lint; follow [comment-templates.md](./comment-templates.md).
- Do not rely on old one-off cleanup scripts; they are no longer part of the repo workflow.
- Run tests after non-trivial lint-driven refactors, especially when changing control flow, struct layout, or concurrency-sensitive code.

## Architectural Guardrails

During the refactor, lint review should also enforce these package-boundary rules:

- do not add new framework code under `pkg/configschema`
- do not recreate removed legacy roots such as `pkg/store`
- route new persistence, transport, and config work to their target families instead of extending legacy roots
- do not place new gRPC runtime code under `pkg/http`
- gRPC runtime code must live in `pkg/grpc`
- `pkg/core` must not import transport families such as `pkg/http` or `pkg/grpc`
- `pkg/grpc` may depend on shared families such as `pkg/core`, `pkg/health`, `pkg/observability`, `pkg/policy`, `pkg/tenant`, and `pkg/audit`, but it must not redefine their contracts
- do not put provider-specific validation logic in `pkg/core`
- do not assume `serve` means HTTP-only once transport families can contribute their own runtime commands

When lint suggests moving code across packages, prefer landing it directly in the target-state family:

- `pkg/config/schema` instead of `pkg/configschema`
- `pkg/http/*`; use `pkg/http/response` and `pkg/http/input` for the responsibilities that previously lived in `pkg/controller`
- `pkg/grpc/*` for gRPC transport runtime, validation, auth, health, reflection, and metadata work
- `pkg/persistence/*`, `pkg/cache/*`, `pkg/session/*`, `pkg/coordination/*`, or `pkg/pubsub/*` instead of expanding `pkg/store`

## Review Cues For gRPC

Treat these as architecture-review checks even when lint does not automate them yet:

- gRPC remains a first-class family under `pkg/grpc`, not an HTTP variant
- Protobuf, Buf, and Protovalidate remain contract-validation providers, not the family definition
- unary and streaming semantics stay explicit instead of being collapsed into generic middleware or handler shapes
- reflection and gRPC health remain optional capabilities, not mandatory runtime assumptions
