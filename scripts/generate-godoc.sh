#!/bin/bash
set -euo pipefail

# Script to analyze Go files and identify missing godoc comments

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG_DIR="$REPO_ROOT/pkg"

echo "Analyzing Go files for missing godoc comments..."
echo "================================================"

# Find all public symbols without godoc comments
find "$PKG_DIR" -name "*.go" -not -name "*_test.go" | while read -r file; do
    # Extract public functions/types without comments
    awk '
    /^type [A-Z]/ {
        if (prev !~ /^\/\//) {
            print FILENAME ":" NR ":" $0
        }
    }
    /^func [A-Z]/ {
        if (prev !~ /^\/\//) {
            print FILENAME ":" NR ":" $0
        }
    }
    /^func \([^)]+\) [A-Z]/ {
        if (prev !~ /^\/\//) {
            print FILENAME ":" NR ":" $0
        }
    }
    { prev = $0 }
    ' "$file"
done | head -50

echo ""
echo "Found public symbols without godoc comments"
