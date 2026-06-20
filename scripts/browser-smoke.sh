#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${GOFRAME_SMOKE_BIN:-/tmp/goframe-smoke-bin}"
GOXC="${GOXC:-}"
CHROME_BIN="${CHROME:-google-chrome}"

TODO_SMOKE_URL_BASE="${GOFRAME_TODO_SMOKE_URL:-http://127.0.0.1}"
DUPLICATE_SMOKE_URL_BASE="${GOFRAME_DUPLICATE_KEY_SMOKE_URL:-http://127.0.0.1}"
RUNTIME_ERRORS_SMOKE_URL_BASE="${GOFRAME_RUNTIME_ERRORS_SMOKE_URL:-http://127.0.0.1}"
DASHBOARD_SMOKE_URL_BASE="${GOFRAME_DASHBOARD_SMOKE_URL:-http://127.0.0.1}"
CONTEXT_SMOKE_URL_BASE="${GOFRAME_CONTEXT_SMOKE_URL:-http://127.0.0.1}"
VIRTUALIZED_SMOKE_URL_BASE="${GOFRAME_VIRTUALIZED_SMOKE_URL:-http://127.0.0.1}"
MULTIPACKAGE_SMOKE_URL_BASE="${GOFRAME_MULTIPACKAGE_SMOKE_URL:-http://127.0.0.1}"
CMDAPP_SMOKE_URL_BASE="${GOFRAME_CMDAPP_SMOKE_URL:-http://127.0.0.1}"

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

manifest_wasm_path() {
	local app="$1"
	node -e 'const fs = require("node:fs");
const manifestPath = process.argv[1] + "/.goframe/package/standalone/asset-manifest.json";
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
if (!manifest.entrypoints || !manifest.entrypoints.wasm) {
  console.error(`missing entrypoints.wasm in ${manifestPath}`);
  process.exit(1);
}
console.log(manifest.entrypoints.wasm);' "$app"
}

build_smoke_url() {
	local port="$1"
	echo "${TODO_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_duplicate_smoke_url() {
	local port="$1"
	echo "${DUPLICATE_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_runtime_errors_smoke_url() {
	local port="$1"
	echo "${RUNTIME_ERRORS_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_dashboard_smoke_url() {
	local port="$1"
	echo "${DASHBOARD_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_context_smoke_url() {
	local port="$1"
	echo "${CONTEXT_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_virtualized_smoke_url() {
	local port="$1"
	echo "${VIRTUALIZED_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_multipackage_smoke_url() {
	local port="$1"
	echo "${MULTIPACKAGE_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
}

build_cmdapp_smoke_url() {
	local port="$1"
	echo "${CMDAPP_SMOKE_URL_BASE}:${port}/?smoke=$(date +%s%N)"
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
	local wasm_path
	wasm_path="$(manifest_wasm_path "$app")"
	wasm_headers="$(curl -fsSI "http://127.0.0.1:$port/$wasm_path?smoke=$(date +%s%N)" || true)"
	if [[ "$wasm_headers" != *"application/wasm"* ]]; then
		echo "HARNESS FAILURE: server did not expose $wasm_path as application/wasm on port $port"
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
(
	cd ./examples/todo/.goframe/work/dev/examples/todo
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/examples/todo/.goframe/package/standalone/assets/bundle.wasm" .
)

TODO_PORT="$(resolve_port "${GOFRAME_TODO_SMOKE_PORT:-}")"
export GOFRAME_CHROME_DEBUG_PORT="${GOFRAME_CHROME_DEBUG_PORT:-$(pick_free_port)}"
TODO_URL="$(build_smoke_url "$TODO_PORT")"
run_with_server ./examples/todo "$TODO_PORT" "$TODO_URL" \
	node --experimental-websocket scripts/todo-browser-smoke.mjs

echo
echo "== Duplicate key debug browser smoke =="
"$GOXC" package ./scripts/fixtures/duplicate-keys --compiler=tinygo
(
	cd ./scripts/fixtures/duplicate-keys/.goframe/work/dev/scripts/fixtures/duplicate-keys
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/scripts/fixtures/duplicate-keys/.goframe/package/standalone/assets/bundle.wasm" .
)

DUPLICATE_PORT="$(resolve_port "${GOFRAME_DUPLICATE_KEY_SMOKE_PORT:-}")"
export GOFRAME_DUPLICATE_KEY_CHROME_DEBUG_PORT="${GOFRAME_DUPLICATE_KEY_CHROME_DEBUG_PORT:-$(pick_free_port)}"
DUPLICATE_URL="$(build_duplicate_smoke_url "$DUPLICATE_PORT")"
run_with_server ./scripts/fixtures/duplicate-keys "$DUPLICATE_PORT" "$DUPLICATE_URL" \
	node --experimental-websocket scripts/duplicate-key-smoke.mjs

echo
echo "== Runtime errors debug browser smoke =="
"$GOXC" package ./scripts/fixtures/runtime-errors --compiler=go

RUNTIME_ERRORS_PORT="$(resolve_port "${GOFRAME_RUNTIME_ERRORS_SMOKE_PORT:-}")"
export GOFRAME_RUNTIME_ERRORS_CHROME_DEBUG_PORT="${GOFRAME_RUNTIME_ERRORS_CHROME_DEBUG_PORT:-$(pick_free_port)}"
RUNTIME_ERRORS_URL="$(build_runtime_errors_smoke_url "$RUNTIME_ERRORS_PORT")"
run_with_server ./scripts/fixtures/runtime-errors "$RUNTIME_ERRORS_PORT" "$RUNTIME_ERRORS_URL" \
	node --experimental-websocket scripts/runtime-errors-browser-smoke.mjs

echo
echo "== Dashboard debug browser smoke =="
"$GOXC" package ./examples/dashboard --compiler=tinygo
(
	cd ./examples/dashboard/.goframe/work/dev/examples/dashboard
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/examples/dashboard/.goframe/package/standalone/assets/bundle.wasm" .
)

DASHBOARD_PORT="$(resolve_port "${GOFRAME_DASHBOARD_SMOKE_PORT:-}")"
export GOFRAME_DASHBOARD_CHROME_DEBUG_PORT="${GOFRAME_DASHBOARD_CHROME_DEBUG_PORT:-$(pick_free_port)}"
DASHBOARD_URL="$(build_dashboard_smoke_url "$DASHBOARD_PORT")"
run_with_server ./examples/dashboard "$DASHBOARD_PORT" "$DASHBOARD_URL" \
	node --experimental-websocket scripts/dashboard-browser-smoke.mjs

echo
echo "== Context debug browser smoke =="
"$GOXC" package ./examples/context --compiler=tinygo
(
	cd ./examples/context/.goframe/work/dev/examples/context
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/examples/context/.goframe/package/standalone/assets/bundle.wasm" .
)

CONTEXT_PORT="$(resolve_port "${GOFRAME_CONTEXT_SMOKE_PORT:-}")"
export GOFRAME_CONTEXT_CHROME_DEBUG_PORT="${GOFRAME_CONTEXT_CHROME_DEBUG_PORT:-$(pick_free_port)}"
CONTEXT_URL="$(build_context_smoke_url "$CONTEXT_PORT")"
run_with_server ./examples/context "$CONTEXT_PORT" "$CONTEXT_URL" \
	node --experimental-websocket scripts/context-browser-smoke.mjs

echo
echo "== Virtualized debug browser smoke =="
"$GOXC" package ./examples/virtualized --compiler=tinygo
(
	cd ./examples/virtualized/.goframe/work/dev/examples/virtualized
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/examples/virtualized/.goframe/package/standalone/assets/bundle.wasm" .
)

VIRTUALIZED_PORT="$(resolve_port "${GOFRAME_VIRTUALIZED_SMOKE_PORT:-}")"
export GOFRAME_VIRTUALIZED_CHROME_DEBUG_PORT="${GOFRAME_VIRTUALIZED_CHROME_DEBUG_PORT:-$(pick_free_port)}"
VIRTUALIZED_URL="$(build_virtualized_smoke_url "$VIRTUALIZED_PORT")"
run_with_server ./examples/virtualized "$VIRTUALIZED_PORT" "$VIRTUALIZED_URL" \
	node --experimental-websocket scripts/virtualized-browser-smoke.mjs

echo
echo "== Multipackage debug browser smoke =="
"$GOXC" package ./examples/multipackage --compiler=tinygo
(
	cd ./examples/multipackage/.goframe/work/dev/examples/multipackage
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/examples/multipackage/.goframe/package/standalone/assets/bundle.wasm" .
)

MULTIPACKAGE_PORT="$(resolve_port "${GOFRAME_MULTIPACKAGE_SMOKE_PORT:-}")"
export GOFRAME_MULTIPACKAGE_CHROME_DEBUG_PORT="${GOFRAME_MULTIPACKAGE_CHROME_DEBUG_PORT:-$(pick_free_port)}"
MULTIPACKAGE_URL="$(build_multipackage_smoke_url "$MULTIPACKAGE_PORT")"
run_with_server ./examples/multipackage "$MULTIPACKAGE_PORT" "$MULTIPACKAGE_URL" \
	node --experimental-websocket scripts/multipackage-browser-smoke.mjs

echo
echo "== Cmdapp debug browser smoke =="
"$GOXC" package ./examples/cmdapp --compiler=tinygo
(
	cd ./examples/cmdapp/.goframe/work/dev/examples/cmdapp/cmd/app
	tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
		-o "$ROOT_DIR/examples/cmdapp/.goframe/package/standalone/assets/bundle.wasm" .
)

CMDAPP_PORT="$(resolve_port "${GOFRAME_CMDAPP_SMOKE_PORT:-}")"
export GOFRAME_CMDAPP_CHROME_DEBUG_PORT="${GOFRAME_CMDAPP_CHROME_DEBUG_PORT:-$(pick_free_port)}"
CMDAPP_URL="$(build_cmdapp_smoke_url "$CMDAPP_PORT")"
run_with_server ./examples/cmdapp "$CMDAPP_PORT" "$CMDAPP_URL" \
	node --experimental-websocket scripts/cmdapp-browser-smoke.mjs

echo
echo "== Restore Todo production bundle =="
"$GOXC" package ./examples/todo --compiler=tinygo
"$GOXC" package ./examples/dashboard --compiler=tinygo
"$GOXC" package ./examples/context --compiler=tinygo
"$GOXC" package ./examples/virtualized --compiler=tinygo
"$GOXC" package ./examples/multipackage --compiler=tinygo
"$GOXC" package ./examples/cmdapp --compiler=tinygo

echo "browser smoke: ok"
