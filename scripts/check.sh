#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${GOFRAME_CHECK_BIN:-/tmp/goframe-check-bin}"

cd "$ROOT_DIR"
mkdir -p "$BIN_DIR"
export GOCACHE="${GOCACHE:-/tmp/goframe-go-cache}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-/tmp/goframe-tinygo-cache}"
mkdir -p "$GOCACHE" "$XDG_CACHE_HOME"

echo "== Artifact gate =="
scripts/artifact-check.sh

echo "== Module path gate =="
scripts/module-path-check.sh

echo "== Go formatting =="
go fmt ./...

echo "== Go tests =="
go test ./...

echo "== Debug-tag tests =="
go test -tags=goframe_debug ./...

echo "== GOX golden tests =="
go test ./pkg/gox -run 'TestGolden|TestErrorGolden'

echo "== Race tests =="
go test -race ./pkg/... ./cmd/...

echo "== Go vet =="
go vet ./...

echo "== Install goxc =="
GOBIN="$BIN_DIR" go install ./cmd/goxc
GOXC="$BIN_DIR/goxc"

echo "== Toolchain doctor =="
"$GOXC" doctor

echo "== Package examples with TinyGo =="
"$GOXC" generate ./examples/counter
"$GOXC" package ./examples/counter --compiler=tinygo
"$GOXC" generate ./examples/components
"$GOXC" package ./examples/components --compiler=tinygo
"$GOXC" generate ./examples/todo
"$GOXC" package ./examples/todo --compiler=tinygo
"$GOXC" generate ./examples/dashboard
"$GOXC" package ./examples/dashboard --compiler=tinygo
"$GOXC" generate ./examples/context
"$GOXC" package ./examples/context --compiler=tinygo
"$GOXC" generate ./examples/virtualized
"$GOXC" package ./examples/virtualized --compiler=tinygo

echo "== Size budgets =="
scripts/size-budget.sh

echo "== Pure benchmarks =="
go test -bench=. -run='^$' ./pkg/goframe

echo "check: ok"
