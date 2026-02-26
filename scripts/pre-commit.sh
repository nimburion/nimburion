#!/bin/bash
# Install: ln -s ../../scripts/pre-commit.sh .git/hooks/pre-commit

set -e

echo "Running pre-commit checks..."

# Format check
if ! gofmt -l . | grep -q '^$'; then
    echo "❌ Code is not formatted. Run: gofmt -w ."
    exit 1
fi

# go.mod tidy check
go mod tidy
if ! git diff --exit-code go.mod go.sum; then
    echo "❌ go.mod is not tidy. Changes staged automatically."
    git add go.mod go.sum
fi

# Fast tests
echo "Running fast tests..."
go test -short ./... || {
    echo "❌ Tests failed"
    exit 1
}

echo "✅ Pre-commit checks passed"
