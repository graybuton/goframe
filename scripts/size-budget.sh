#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

check_budget() {
	local name="$1"
	local file="$2"
	local budget="$3"

	if [[ ! -f "$file" ]]; then
		printf '%-12s missing: %s\n' "$name" "${file#$ROOT_DIR/}"
		return 1
	fi

	local size
	size="$(wc -c < "$file")"
	printf '%-12s %8d B / %8d B  %s\n' "$name" "$size" "$budget" "${file#$ROOT_DIR/}"
	if (( size > budget )); then
		printf '%-12s over budget by %d B\n' "$name" "$((size - budget))"
		return 1
	fi
}

status=0
check_budget "counter" "$ROOT_DIR/examples/counter/dist/main.wasm" 97280 || status=1
check_budget "components" "$ROOT_DIR/examples/components/dist/main.wasm" 107520 || status=1
check_budget "todo" "$ROOT_DIR/examples/todo/dist/main.wasm" 122880 || status=1

exit "$status"
