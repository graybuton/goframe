#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

check_raw_budget() {
	local name="$1"
	local file="$2"
	local budget="$3"

	if [[ ! -f "$file" ]]; then
		printf '%-12s missing: %s\n' "$name" "${file#$ROOT_DIR/}"
		return 1
	fi

	local size
	size="$(wc -c < "$file")"
	printf '%-12s raw   %8d B / %8d B  %s\n' "$name" "$size" "$budget" "${file#$ROOT_DIR/}"
	if (( size > budget )); then
		printf '%-12s raw over budget by %d B\n' "$name" "$((size - budget))"
		return 1
	fi
}

ratio_percent() {
	local compressed="$1"
	local raw="$2"
	awk -v compressed="$compressed" -v raw="$raw" 'BEGIN { printf "%.2f", compressed * 100 / raw }'
}

check_compressed_budget() {
	local name="$1"
	local format="$2"
	local raw_file="$3"
	local compressed_file="$4"
	local budget="$5"
	local ratio_budget_permyriad="$6"

	local raw_size
	raw_size="$(wc -c < "$raw_file")"
	local size
	size="$(wc -c < "$compressed_file")"
	local ratio_permyriad=$((size * 10000 / raw_size))
	local ratio
	ratio="$(ratio_percent "$size" "$raw_size")"

	printf '%-12s %-5s %8d B / %8d B  ratio %s%%\n' "$name" "$format" "$size" "$budget" "$ratio"
	if (( size > budget )); then
		printf '%-12s %s over budget by %d B\n' "$name" "$format" "$((size - budget))"
		return 1
	fi
	if (( ratio_permyriad > ratio_budget_permyriad )); then
		printf '%-12s %s ratio over budget: %s%%\n' "$name" "$format" "$ratio"
		return 1
	fi
}

compress_gzip() {
	local source="$1"
	local destination="$2"
	if ! command -v gzip >/dev/null 2>&1; then
		printf 'gzip missing; cannot check gzip bundle budgets\n'
		return 1
	fi
	gzip -c -9 "$source" > "$destination"
}

compress_brotli() {
	local source="$1"
	local destination="$2"
	if ! command -v brotli >/dev/null 2>&1; then
		printf 'brotli missing; cannot check brotli bundle budgets\n'
		return 1
	fi
	brotli -f -q 11 -o "$destination" "$source"
}

compress_zstd() {
	local source="$1"
	local destination="$2"
	if ! command -v zstd >/dev/null 2>&1; then
		printf 'zstd missing; skipping optional zstd budget for %s\n' "${source#$ROOT_DIR/}"
		return 2
	fi
	zstd -q -19 -c "$source" > "$destination"
}

check_app() {
	local name="$1"
	local file="$2"
	local raw_budget="$3"
	local br_budget="$4"
	local gzip_budget="$5"
	local zstd_budget="$6"

	local status=0
	check_raw_budget "$name" "$file" "$raw_budget" || status=1
	if [[ ! -f "$file" ]]; then
		return "$status"
	fi

	local gzip_file="$TMP_DIR/$name.wasm.gz"
	local br_file="$TMP_DIR/$name.wasm.br"
	local zstd_file="$TMP_DIR/$name.wasm.zst"

	compress_gzip "$file" "$gzip_file" && check_compressed_budget "$name" gzip "$file" "$gzip_file" "$gzip_budget" 5200 || status=1
	compress_brotli "$file" "$br_file" && check_compressed_budget "$name" br "$file" "$br_file" "$br_budget" 3800 || status=1
	if compress_zstd "$file" "$zstd_file"; then
		check_compressed_budget "$name" zstd "$file" "$zstd_file" "$zstd_budget" 4600 || status=1
	fi
	return "$status"
}

bundle_path() {
	local app="$1"
	local package_dir="$ROOT_DIR/examples/$app/.goframe/package/standalone"
	local matches=()
	shopt -s nullglob
	matches=("$package_dir"/assets/bundle*.wasm "$package_dir"/main.wasm)
	shopt -u nullglob
	if (( ${#matches[@]} == 0 )); then
		echo "$package_dir/assets/bundle.wasm"
		return 0
	fi
	echo "${matches[0]}"
}

status=0
check_app "counter" "$(bundle_path counter)" 97280 40960 56320 49152 || status=1
check_app "components" "$(bundle_path components)" 107520 43008 56320 49152 || status=1
check_app "todo" "$(bundle_path todo)" 122880 40960 56320 49152 || status=1
check_app "dashboard" "$(bundle_path dashboard)" 168960 53248 71680 61440 || status=1
check_app "context" "$(bundle_path context)" 113664 36864 46080 40960 || status=1
check_app "virtualized" "$(bundle_path virtualized)" 122880 40960 49152 44032 || status=1

exit "$status"
