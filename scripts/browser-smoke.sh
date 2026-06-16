#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${GOFRAME_SMOKE_BIN:-/tmp/goframe-smoke-bin}"
GOXC="${GOXC:-}"
CHROME_BIN="${CHROME:-google-chrome}"

TODO_SMOKE_URL_BASE="${GOFRAME_TODO_SMOKE_URL:-http://127.0.0.1}"
DUPLICATE_SMOKE_URL_BASE="${GOFRAME_DUPLICATE_KEY_SMOKE_URL:-http://127.0.0.1}"

cd "$ROOT_DIR"
export GOCACHE="${GOCACHE:-/tmp/goframe-go-cache}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-/tmp/goframe-tinygo-cache}"
mkdir -p "$GOCACHE" "$XDG_CACHE_HOME" "$BIN_DIR"

require_command() {
	local command="$1"
	local help="$2"
	if ! command -v "$command" >/dev/null 2>&1; then
		echo "missing command: $command"
		echo "$help"
		exit 1
	fi
}

pick_free_port() {
	node -e 'const net = require("node:net");
const server = net.createServer();
server.on("error", (error) => {
  console.error(error.message);
  process.exit(1);
});
server.listen(0, "127.0.0.1", () => {
  const address = server.address();
  console.log(address.port);
  server.close();
});'
}

resolve_port() {
	local configured="$1"
	if [[ -n "$configured" ]]; then
		echo "$configured"
		return 0
	fi
	pick_free_port
}

wait_for_server() {
	local port="$1"
	local pid="$2"
	for _ in {1..120}; do
		if curl -fsS "http://127.0.0.1:$port/" >/dev/null 2>&1; then
			return 0
		fi
		if ! kill -0 "$pid" >/dev/null 2>&1; then
			return 1
		fi
		sleep 0.1
	done
	return 1
}

build_smoke_url() {
	local port="$1"
	echo "${TODO_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_duplicate_smoke_url() {
	local port="$1"
	echo "${DUPLICATE_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
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
	local url="$3"
	shift 3

	local log_file="$(mktemp)"
	if curl -fsS "http://127.0.0.1:$port/" >/dev/null 2>&1; then
		echo "HARNESS FAILURE: port $port already serves HTTP before smoke server start"
		rm -f "$log_file"
		exit 1
	fi

	"$GOXC" serve "$app" --port="$port" > "$log_file" 2>&1 &
	local server_pid="$!"
	trap 'stop_server "$server_pid"; rm -f "$log_file"' RETURN

	if ! wait_for_server "$port" "$server_pid"; then
		if ! kill -0 "$server_pid" >/dev/null 2>&1; then
			echo "HARNESS FAILURE: goxc serve failed to start for $app"
			cat "$log_file"
			exit 1
		fi
		stop_server "$server_pid"
		echo "HARNESS FAILURE: failed to start HTTP server on port $port"
		cat "$log_file"
		exit 1
	fi

	local wasm_headers
	wasm_headers="$(curl -fsSI "http://127.0.0.1:$port/main.wasm?smoke=$(date +%s%N)" || true)"
	if [[ "$wasm_headers" != *"application/wasm"* ]]; then
		echo "HARNESS FAILURE: server did not expose main.wasm as application/wasm on port $port"
		echo "$wasm_headers"
		cat "$log_file"
		exit 1
	fi

	set +e
	"$@" "$url"
	local status=$?
	set -e
	trap - RETURN
	stop_server "$server_pid"
	rm -f "$log_file"
	return "$status"
}

if [[ -z "$GOXC" ]]; then
	GOBIN="$BIN_DIR" go install ./cmd/goxc
	GOXC="$BIN_DIR/goxc"
fi

require_command "$GOXC" "Install it with: go install ./cmd/goxc"
require_command tinygo "Install TinyGo or skip browser smoke."
require_command node "Install Node.js 22+ or skip browser smoke."
require_command curl "Install curl for smoke server readiness checks."
require_command "$CHROME_BIN" "Set CHROME=/path/to/chrome if Chrome is installed under another name."

echo "== Todo debug browser smoke =="
"$GOXC" package ./examples/todo --compiler=tinygo
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
	-o ./examples/todo/dist/main.wasm ./examples/todo

TODO_PORT="$(resolve_port "${GOFRAME_TODO_SMOKE_PORT:-}")"
export GOFRAME_CHROME_DEBUG_PORT="${GOFRAME_CHROME_DEBUG_PORT:-$(pick_free_port)}"
TODO_URL="$(build_smoke_url "$TODO_PORT")"
run_with_server ./examples/todo "$TODO_PORT" "$TODO_URL" \
	node --experimental-websocket scripts/todo-browser-smoke.mjs

echo
echo "== Duplicate key debug browser smoke =="
"$GOXC" package ./scripts/fixtures/duplicate-keys --compiler=tinygo
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
	-o ./scripts/fixtures/duplicate-keys/dist/main.wasm ./scripts/fixtures/duplicate-keys

DUPLICATE_PORT="$(resolve_port "${GOFRAME_DUPLICATE_KEY_SMOKE_PORT:-}")"
export GOFRAME_DUPLICATE_KEY_CHROME_DEBUG_PORT="${GOFRAME_DUPLICATE_KEY_CHROME_DEBUG_PORT:-$(pick_free_port)}"
DUPLICATE_URL="$(build_duplicate_smoke_url "$DUPLICATE_PORT")"
run_with_server ./scripts/fixtures/duplicate-keys "$DUPLICATE_PORT" "$DUPLICATE_URL" \
	node --experimental-websocket scripts/duplicate-key-smoke.mjs

echo
echo "== Restore Todo production bundle =="
"$GOXC" package ./examples/todo --compiler=tinygo

echo "browser smoke: ok"
