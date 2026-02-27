#!/bin/bash
# Fix os.Setenv errors in pkg/cli/main.go

sed -i.bak 's/_ = os\.Setenv(/if err := os.Setenv(/g' pkg/cli/main.go

# Add error handling after each Setenv
perl -i -pe 's/if err := os\.Setenv\("([^"]+)", ([^)]+)\)$/if err := os.Setenv("$1", $2); err != nil {\n\t\t\t\treturn fmt.Errorf("failed to set $1: %w", err)\n\t\t\t}/g' pkg/cli/main.go

rm -f pkg/cli/main.go.bak

echo "Fixed os.Setenv in pkg/cli/main.go"
