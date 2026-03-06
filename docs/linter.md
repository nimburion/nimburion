# Lint Workflow

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
