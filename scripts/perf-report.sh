#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"
export GOCACHE="${GOCACHE:-/tmp/goframe-go-cache}"
mkdir -p "$GOCACHE"

echo "== GoFrame pure runtime benchmarks =="
go test -bench=. -benchmem ./pkg/goframe

echo
echo "== TinyGo size budgets =="
scripts/size-budget.sh
