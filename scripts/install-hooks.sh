#!/bin/sh

set -eu

cp scripts/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
echo "Installed scripts/pre-commit into .git/hooks/pre-commit"
