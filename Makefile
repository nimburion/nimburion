.PHONY: help lint lint-fix lint-critical security tidy verify ci-local \
	test test-fast test-integration test-parallel test-coverage test-coverage-html \
	test-build test-fast-lane test-integration-lane test-contract-lane test-nonfunctional-lane \
	test-review-sweep

TEST_PKG ?= ./...
TEST_RUN ?=

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

lint: ## Run linter
	golangci-lint run

lint-fix: ## Run linter and apply automated fixes
	golangci-lint run --fix

lint-critical: ## Show only critical security issues
	golangci-lint run --disable-all -E gosec,errcheck --max-issues-per-linter=50

test: ## Run all tests
	./scripts/test.sh

test-fast: ## Run fast tests only (skip slow integration tests)
	./scripts/test.sh fast

test-integration: ## Run integration tests (requires Docker services)
	./scripts/test.sh integration

test-parallel: ## Run tests in parallel (uses all CPU cores)
	go test ./... -p $$(nproc 2>/dev/null || sysctl -n hw.ncpu) -count=1 -v

test-coverage: ## Run tests with coverage report
	@go test ./... -count=1 -covermode=atomic -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1

test-coverage-html: test-coverage ## Generate HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-build: ## Build-only verification lane; set TEST_PKG=./path/...
	go test $(TEST_PKG) -run '^$$'

test-fast-lane: ## Fast verification lane; set TEST_PKG=./path/...
	go test $(TEST_PKG) -short

test-integration-lane: ## Integration verification lane; set TEST_PKG=./path/... or TEST_RUN=Integration
	go test $(TEST_PKG) -run '$(if $(TEST_RUN),$(TEST_RUN),Integration)'

test-contract-lane: ## Contract verification lane; set TEST_PKG=./path/... or TEST_RUN=Contract
	go test $(TEST_PKG) -run '$(if $(TEST_RUN),$(TEST_RUN),Contract)'

test-nonfunctional-lane: ## Non-functional lane; set TEST_PKG=./path/... or TEST_RUN='Performance|...'
	go test $(TEST_PKG) -run '$(if $(TEST_RUN),$(TEST_RUN),Performance|Load|Soak|Resilience|Security|Compatibility|Race|Ordering)'

test-review-sweep: ## Run the documented pre-merge review sweep
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

security: ## Run security checks
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

tidy: ## Tidy go.mod
	go mod tidy

verify: tidy ## Verify no uncommitted changes
	@git diff --exit-code go.mod go.sum || (echo "go.mod or go.sum has uncommitted changes" && exit 1)

ci-local: lint security test-parallel ## Run CI checks locally
