# Testing Guide

## Quick Start

```bash
# Fast unit tests only (no external dependencies)
make test-fast

# All tests with Docker services
make test-integration

# Parallel execution
make test-parallel

# With coverage
make test-coverage
```

## Test Types

### Unit Tests
Pure unit tests with no external dependencies. Run by default.

```bash
go test ./... -short
```

### Integration Tests
Tests that require external services (Redis, Postgres, etc.). Skipped with `-short` flag.

```bash
# Start services
docker compose -f docker-compose.test.yml up -d

# Run tests
REDIS_URL=redis://localhost:6379/0 \
POSTGRES_URL=postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable \
go test ./...

# Stop services
docker compose -f docker-compose.test.yml down
```

### Property Tests
Property-based tests using gopter. Skipped with `-short` flag.

## CI/CD

- **PR workflow**: Runs fast tests only (`-short`)
- **Release workflow**: Runs all tests with GitHub Actions services

## Environment Variables

- `REDIS_URL`: Redis connection string (default: `redis://localhost:6379/0`)
- `POSTGRES_URL`: PostgreSQL connection string
- `INTEGRATION_TESTS=1`: Force integration tests to run
