#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOXC="${GOXC:-goxc}"
CHROME_BIN="${CHROME:-google-chrome}"
TODO_PORT="${GOFRAME_TODO_SMOKE_PORT:-18080}"
DUPLICATE_PORT="${GOFRAME_DUPLICATE_KEY_SMOKE_PORT:-18081}"

cd "$ROOT_DIR"
export GOCACHE="${GOCACHE:-/tmp/goframe-go-cache}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-/tmp/goframe-tinygo-cache}"
mkdir -p "$GOCACHE" "$XDG_CACHE_HOME"

require_command() {
	local command="$1"
	local help="$2"
	if ! command -v "$command" >/dev/null 2>&1; then
		echo "missing command: $command"
		echo "$help"
		exit 1
	fi
}

stop_server() {
	local pid="${1:-}"
	if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
		kill "$pid" >/dev/null 2>&1 || true
		wait "$pid" >/dev/null 2>&1 || true
	fi
}

run_with_server() {
	local app="$1"
	local port="$2"
	shift 2

	"$GOXC" serve "$app" --port="$port" &
	local server_pid="$!"
	trap 'stop_server "$server_pid"' RETURN
	sleep 0.2
	"$@"
	stop_server "$server_pid"
	trap - RETURN
}

require_command "$GOXC" "Install it with: go install ./cmd/goxc"
require_command tinygo "Install TinyGo or skip browser smoke."
require_command node "Install Node.js 22+ or skip browser smoke."
require_command "$CHROME_BIN" "Set CHROME=/path/to/chrome if Chrome is installed under another name."

echo "== Todo debug browser smoke =="
"$GOXC" package ./examples/todo --compiler=tinygo
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
	-o ./examples/todo/dist/main.wasm ./examples/todo
run_with_server ./examples/todo "$TODO_PORT" \
	node --experimental-websocket scripts/todo-browser-smoke.mjs "http://127.0.0.1:$TODO_PORT/"

echo
echo "== Duplicate key debug browser smoke =="
"$GOXC" package ./scripts/fixtures/duplicate-keys --compiler=tinygo
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
	-o ./scripts/fixtures/duplicate-keys/dist/main.wasm ./scripts/fixtures/duplicate-keys
run_with_server ./scripts/fixtures/duplicate-keys "$DUPLICATE_PORT" \
	node --experimental-websocket scripts/duplicate-key-smoke.mjs "http://127.0.0.1:$DUPLICATE_PORT/"

echo
echo "== Restore Todo production bundle =="
"$GOXC" package ./examples/todo --compiler=tinygo

echo "browser smoke: ok"
