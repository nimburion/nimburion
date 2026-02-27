#!/bin/bash
# Fix struct field alignment using fieldalignment tool

# Install fieldalignment if not present
if ! command -v fieldalignment &> /dev/null; then
    echo "Installing fieldalignment..."
    go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
fi

# Fix all packages
echo "Fixing field alignment..."
fieldalignment -fix ./pkg/...

echo "Field alignment fixed. Run tests to ensure no breakage."
