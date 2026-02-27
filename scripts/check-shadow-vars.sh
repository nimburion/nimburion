#!/bin/bash
# Fix common shadow variable patterns

echo "Fixing shadow variables..."

# Pattern 1: if err := something(); err != nil (when err already exists)
# This requires manual review as automated fix could break logic

# Pattern 2: Find remaining shadow issues
echo "Checking for remaining shadow variables..."
golangci-lint run --disable-all -E govet --out-format=line-number 2>&1 | grep "shadow:" > /tmp/shadow-issues.txt

if [ -s /tmp/shadow-issues.txt ]; then
    echo "Found shadow variable issues:"
    cat /tmp/shadow-issues.txt | head -20
    echo ""
    echo "Total issues: $(wc -l < /tmp/shadow-issues.txt)"
else
    echo "No shadow variable issues found!"
fi

rm -f /tmp/shadow-issues.txt
