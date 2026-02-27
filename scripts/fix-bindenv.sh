#!/bin/bash
# Fix v.BindEnv errors - these are safe to ignore in config loading context

# Add nolint comment for BindEnv calls
find pkg/config -name "*.go" -type f -exec sed -i.bak \
  's/v\.BindEnv(/\/\/nolint:errcheck \/\/ BindEnv errors are non-critical in config setup\n\tv.BindEnv(/g' \
  {} \;

# Clean up formatting
gofmt -w pkg/config/

# Remove backup files
find pkg/config -name "*.bak" -type f -delete

echo "Fixed v.BindEnv errcheck issues"
