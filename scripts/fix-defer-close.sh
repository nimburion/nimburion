#!/bin/bash
# Fix pattern: defer resp.Body.Close() -> defer func() { _ = resp.Body.Close() }()

find pkg -name "*.go" -type f -exec sed -i.bak \
  -e 's/defer resp\.Body\.Close()/defer func() { _ = resp.Body.Close() }()/g' \
  -e 's/defer \([a-zA-Z]*\)\.Body\.Close()/defer func() { _ = \1.Body.Close() }()/g' \
  {} \;

# Remove backup files
find pkg -name "*.bak" -type f -delete

echo "Fixed defer Body.Close() patterns"
