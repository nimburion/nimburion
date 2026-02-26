#!/bin/bash
# Verify test setup and run appropriate tests

set -e

echo "🔍 Checking test environment..."

# Check if Docker is available
if command -v docker &> /dev/null; then
    echo "✅ Docker found"
    DOCKER_AVAILABLE=true
else
    echo "⚠️  Docker not found - integration tests will be skipped"
    DOCKER_AVAILABLE=false
fi

# Check if services are running
if [ "$DOCKER_AVAILABLE" = true ]; then
    if docker compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo "✅ Test services are running"
        SERVICES_RUNNING=true
    else
        echo "ℹ️  Test services not running"
        SERVICES_RUNNING=false
    fi
fi

# Determine which tests to run
if [ "${1:-}" = "integration" ]; then
    if [ "$DOCKER_AVAILABLE" = false ]; then
        echo "❌ Cannot run integration tests without Docker"
        exit 1
    fi
    
    echo "🚀 Starting test services..."
    docker compose -f docker-compose.test.yml up -d
    sleep 3
    
    echo "🧪 Running integration tests..."
    REDIS_URL=redis://localhost:6379/0 \
    POSTGRES_URL=postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable \
    go test ./... -count=1 -v -race
    
    echo "🛑 Stopping test services..."
    docker compose -f docker-compose.test.yml down
else
    echo "🧪 Running fast tests (unit tests only)..."
    go test ./... -short -count=1 -v -race
fi

echo "✅ Tests completed successfully"
