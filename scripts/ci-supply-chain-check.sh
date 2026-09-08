#!/usr/bin/env bash
set -euo pipefail

EXPECTED_TINYGO_VERSION="0.42.0"
EXPECTED_TINYGO_SHA256="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
EXPECTED_TINYGO_WORKFLOWS=(
	".github/workflows/ci-browser-smoke.yml"
	".github/workflows/ci-core.yml"
	".github/workflows/ci-wasm-size.yml"
)

usage() {
	printf 'usage: %s [repository-root]\n' "${0##*/}" >&2
}

if (( $# > 1 )); then
	usage
	exit 2
fi

if (( $# == 1 )); then
	ROOT_DIR="$(cd "$1" && pwd)"
else
	ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

failures=0
remote_action_count=0

fail() {
	printf 'ci supply-chain check: %s\n' "$*" >&2
	failures=1
}

trim_whitespace() {
	local value="$1"
	value="${value#"${value%%[![:space:]]*}"}"
	value="${value%"${value##*[![:space:]]}"}"
	printf '%s' "$value"
}

active_fixed_lines() {
	local needle="$1"
	local file="$2"
	grep -nF "$needle" "$file" 2>/dev/null |
		grep -Ev '^[0-9]+:[[:space:]]*#' || true
}

scan_roots=()
for candidate in "$ROOT_DIR/.github/workflows" "$ROOT_DIR/.github/actions"; do
	if [[ -d "$candidate" ]]; then
		scan_roots+=("$candidate")
	fi
done
if (( ${#scan_roots[@]} == 0 )); then
	fail "no authored GitHub workflow or Action directories found under $ROOT_DIR/.github"
fi

workflow_files=()
if (( ${#scan_roots[@]} > 0 )); then
	mapfile -d '' workflow_files < <(
		find "${scan_roots[@]}" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0 |
			sort -z
	)
fi

for file in "${workflow_files[@]}"; do
	line_number=0
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		if [[ "$line" =~ ^[[:space:]]*# ]]; then
			continue
		fi
		if [[ ! "$line" =~ ^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*(.*)$ ]]; then
			continue
		fi

		value="${BASH_REMATCH[2]}"
		value="${value%%#*}"
		value="$(trim_whitespace "$value")"
		if (( ${#value} >= 2 )); then
			if [[ "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
				value="${value:1:${#value}-2}"
			elif [[ "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
				value="${value:1:${#value}-2}"
			fi
		fi

		if [[ -z "$value" ]]; then
			fail "${file#"$ROOT_DIR/"}:$line_number has an empty uses reference"
			continue
		fi
		if [[ "$value" == ./* ]]; then
			continue
		fi

		remote_action_count=$((remote_action_count + 1))
		if [[ ! "$value" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[^@[:space:]]+)?@[0-9a-f]{40}$ ]]; then
			fail "${file#"$ROOT_DIR/"}:$line_number remote uses reference is not pinned to a lowercase 40-character commit SHA: $value"
		fi
	done < "$file"
done

declare -A expected_tinygo_workflows=()
for relative in "${EXPECTED_TINYGO_WORKFLOWS[@]}"; do
	expected_tinygo_workflows["$relative"]=1
done

direct_tinygo_workflows=()
for file in "${workflow_files[@]}"; do
	if [[ -n "$(active_fixed_lines 'tinygo-org/tinygo/releases/download/' "$file")" ]]; then
		direct_tinygo_workflows+=("${file#"$ROOT_DIR/"}")
	fi
done

if (( ${#direct_tinygo_workflows[@]} != ${#EXPECTED_TINYGO_WORKFLOWS[@]} )); then
	fail "expected ${#EXPECTED_TINYGO_WORKFLOWS[@]} direct TinyGo workflow downloads, found ${#direct_tinygo_workflows[@]}"
fi

for relative in "${direct_tinygo_workflows[@]}"; do
	if [[ -z "${expected_tinygo_workflows[$relative]+present}" ]]; then
		fail "unexpected direct TinyGo download in $relative"
	fi
done

tinygo_download_command='curl -fsSL -o /tmp/tinygo.deb "https://github.com/tinygo-org/tinygo/releases/download/v${TINYGO_VERSION}/tinygo_${TINYGO_VERSION}_amd64.deb"'
tinygo_verify_command="printf '%s  %s\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict -"
tinygo_install_command='sudo apt-get install -y /tmp/tinygo.deb'
tinygo_version_command='tinygo version'

count_tinygo_install_sequences() {
	local file="$1"
	local lines=()
	local index run_header run_indent command_indent
	local download_line verify_line install_line version_line
	local count=0

	mapfile -t lines < "$file"
	for ((index = 0; index + 4 < ${#lines[@]}; index++)); do
		run_header="$(trim_whitespace "${lines[index]}")"
		if [[ "$run_header" != "run: |" && "$run_header" != "- run: |" ]]; then
			continue
		fi

		run_indent="${lines[index]%%[![:space:]]*}"
		download_line="${lines[index + 1]}"
		verify_line="${lines[index + 2]}"
		install_line="${lines[index + 3]}"
		version_line="${lines[index + 4]}"
		command_indent="${download_line%%[![:space:]]*}"

		if (( ${#command_indent} <= ${#run_indent} )) || [[ "$command_indent" != "$run_indent"* ]]; then
			continue
		fi
		if [[ "${verify_line%%[![:space:]]*}" != "$command_indent" ||
			"${install_line%%[![:space:]]*}" != "$command_indent" ||
			"${version_line%%[![:space:]]*}" != "$command_indent" ]]; then
			continue
		fi
		if [[ "$(trim_whitespace "$download_line")" == "$tinygo_download_command" &&
			"$(trim_whitespace "$verify_line")" == "$tinygo_verify_command" &&
			"$(trim_whitespace "$install_line")" == "$tinygo_install_command" &&
			"$(trim_whitespace "$version_line")" == "$tinygo_version_command" ]]; then
			count=$((count + 1))
		fi
	done

	printf '%d' "$count"
}

for relative in "${EXPECTED_TINYGO_WORKFLOWS[@]}"; do
	file="$ROOT_DIR/$relative"
	if [[ ! -f "$file" ]]; then
		fail "required TinyGo workflow is missing: $relative"
		continue
	fi

	version_count="$(grep -Ec "^[[:space:]]*TINYGO_VERSION:[[:space:]]*${EXPECTED_TINYGO_VERSION//./\\.}([[:space:]]*(#.*)?)?$" "$file" || true)"
	if [[ "$version_count" != "1" ]]; then
		fail "$relative must declare TINYGO_VERSION exactly once as $EXPECTED_TINYGO_VERSION"
	fi

	sha_declaration_count="$(grep -Ec '^[[:space:]]*TINYGO_SHA256:' "$file" || true)"
	sha_match_count="$(grep -Ec "^[[:space:]]*TINYGO_SHA256:[[:space:]]*$EXPECTED_TINYGO_SHA256([[:space:]]*(#.*)?)?$" "$file" || true)"
	if [[ "$sha_declaration_count" != "1" || "$sha_match_count" != "1" ]]; then
		fail "$relative must declare exactly one accepted TinyGo SHA-256: $EXPECTED_TINYGO_SHA256"
	fi

	mapfile -t download_lines < <(active_fixed_lines "$tinygo_download_command" "$file")
	mapfile -t verify_lines < <(active_fixed_lines "$tinygo_verify_command" "$file")
	mapfile -t install_lines < <(active_fixed_lines "$tinygo_install_command" "$file")
	mapfile -t version_lines < <(active_fixed_lines "$tinygo_version_command" "$file")

	if (( ${#download_lines[@]} != 1 )); then
		fail "$relative must contain exactly one accepted TinyGo download command"
	fi
	if (( ${#verify_lines[@]} != 1 )); then
		fail "$relative must contain exactly one accepted TinyGo verification command"
	fi
	if (( ${#install_lines[@]} != 1 )); then
		fail "$relative must contain exactly one accepted TinyGo installation command"
	fi
	if (( ${#version_lines[@]} != 1 )); then
		fail "$relative must contain exactly one accepted TinyGo version-report command"
	fi

	sequence_count="$(count_tinygo_install_sequences "$file")"
	if [[ "$sequence_count" != "1" ]]; then
		fail "$relative must contain exactly one contiguous TinyGo download, verification, installation, and version-report sequence"
	fi
done

if (( failures != 0 )); then
	exit 1
fi

printf 'ci supply-chain check: ok (%d remote Action refs, %d verified TinyGo downloads)\n' \
	"$remote_action_count" "${#direct_tinygo_workflows[@]}"
