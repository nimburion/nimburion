# Repository Scripts

This directory contains only the repository-level scripts that are still part of the normal developer workflow.

## Scripts

### `test.sh`
Runs the local test workflow used by the `Makefile`.

```bash
# Fast unit tests
./scripts/test.sh

# Integration tests with Docker services
./scripts/test.sh integration
```

### `pre-commit`
Local git pre-commit hook. It is intentionally lightweight and does not mutate the worktree.

Checks:
- staged Go files exist
- `gofmt` is clean on staged Go files
- `golangci-lint` passes

### `install-hooks.sh`
Installs the local `pre-commit` hook into `.git/hooks/`.

```bash
./scripts/install-hooks.sh
```

## Policy

- Keep only scripts with a clear, current role in local development.
- One-off cleanup, codemod, or AI-generated maintenance scripts should not remain in this directory after use.
