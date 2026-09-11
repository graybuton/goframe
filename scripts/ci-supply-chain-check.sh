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

count_active_occurrences() {
	local needle="$1"
	local file="$2"
	local line remainder count=0
	while IFS= read -r line || [[ -n "$line" ]]; do
		if [[ "$line" =~ ^[[:space:]]*# ]]; then
			continue
		fi
		# A # inside a shell string is data, not necessarily a comment.
		remainder="$line"
		while [[ "$remainder" == *"$needle"* ]]; do
			count=$((count + 1))
			remainder="${remainder#*"$needle"}"
		done
	done < "$file"
	printf '%d' "$count"
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

scan_action_refs() {
	local file="$1"
	local line line_number=0 indent syntax key value sequence_prefix
	local scalar_indent=-1
	local mapping_re='^(-[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*):([[:space:]]+(.*))?$'
	local scalar_re='^[|>][+-]?$'
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		syntax="$(trim_whitespace "${line%%#*}")"
		if [[ -z "$syntax" ]]; then
			continue
		fi
		indent="${line%%[![:space:]]*}"
		if (( scalar_indent >= 0 && ${#indent} > scalar_indent )); then
			continue
		fi
		scalar_indent=-1

		# Only canonical block mapping keys are authored here. Reject YAML
		# indirection and flow forms instead of letting them hide a uses key.
		if [[ ! "$syntax" =~ $mapping_re ]]; then
			case "${syntax#- }" in
				*:*|\{*|\[*|\?*|\&*|\**|\!*)
					fail "${file#"$ROOT_DIR/"}:$line_number unsupported workflow mapping syntax; use canonical block keys"
					;;
			esac
			continue
		fi
		sequence_prefix="${BASH_REMATCH[1]}"
		key="${BASH_REMATCH[2]}"
		value="${BASH_REMATCH[4]}"
		case "$value" in
			\{*|\[*|\&*|\**|\!*)
				fail "${file#"$ROOT_DIR/"}:$line_number unsupported workflow mapping value; use canonical block mappings"
				continue
				;;
		esac
		if [[ "$key" != uses ]]; then
			if [[ "$value" =~ $scalar_re ]]; then
				scalar_indent=$((${#indent} + ${#sequence_prefix}))
			fi
			continue
		fi

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
}

direct_tinygo_downloads=0
if (( ${#scan_roots[@]} > 0 )); then
	# Traversal order affects diagnostics only; keep filenames NUL-delimited.
	while IFS= read -r -d '' file; do
		scan_action_refs "$file"
		download_count="$(count_active_occurrences 'tinygo-org/tinygo/releases/download/' "$file")"
		direct_tinygo_downloads=$((direct_tinygo_downloads + download_count))
		if (( download_count > 0 )); then
			relative="${file#"$ROOT_DIR/"}"
			expected=false
			for candidate in "${EXPECTED_TINYGO_WORKFLOWS[@]}"; do
				if [[ "$relative" == "$candidate" ]]; then
					expected=true
				fi
			done
			if [[ "$expected" == false ]]; then
				fail "unexpected direct TinyGo download in $relative"
			elif (( download_count != 1 )); then
				fail "$relative must contain exactly one direct TinyGo release download, found $download_count"
			fi
		fi
	done < <(find "${scan_roots[@]}" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0)
fi

if (( direct_tinygo_downloads != ${#EXPECTED_TINYGO_WORKFLOWS[@]} )); then
	fail "expected ${#EXPECTED_TINYGO_WORKFLOWS[@]} direct TinyGo release downloads, found $direct_tinygo_downloads"
fi

tinygo_download_command='curl -fsSL -o /tmp/tinygo.deb "https://github.com/tinygo-org/tinygo/releases/download/v${TINYGO_VERSION}/tinygo_${TINYGO_VERSION}_amd64.deb"'
tinygo_verify_command="printf '%s  %s\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || exit 1"
tinygo_install_command='sudo apt-get install -y /tmp/tinygo.deb'
tinygo_version_command='tinygo version'

count_tinygo_install_sequences() {
	local file="$1"
	local line command indent run_indent command_indent expected_command
	local state=0 count=0
	while IFS= read -r line || [[ -n "$line" ]]; do
		command="$(trim_whitespace "$line")"
		indent="${line%%[![:space:]]*}"
		if (( state > 0 )); then
			case "$state" in
				1) expected_command="$tinygo_download_command"; command_indent="$indent" ;;
				2) expected_command="$tinygo_verify_command" ;;
				3) expected_command="$tinygo_install_command" ;;
				4) expected_command="$tinygo_version_command" ;;
			esac
			if [[ "$command" == "$expected_command" && "$indent" == "$command_indent" &&
				"$indent" == "$run_indent"* ]] && (( ${#indent} > ${#run_indent} )); then
				state=$((state + 1))
				if (( state == 5 )); then
					count=$((count + 1))
					state=0
				fi
			else
				state=0
			fi
		fi
		if [[ "$command" == 'run: |' || "$command" == '- run: |' ]]; then
			run_indent="$indent"
			state=1
		fi
	done < "$file"

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

	if [[ "$(count_active_occurrences "$tinygo_download_command" "$file")" != 1 ]]; then
		fail "$relative must contain exactly one accepted TinyGo download command"
	fi
	if [[ "$(count_active_occurrences "$tinygo_verify_command" "$file")" != 1 ]]; then
		fail "$relative must contain exactly one accepted TinyGo verification command"
	fi
	if [[ "$(count_active_occurrences "$tinygo_install_command" "$file")" != 1 ]]; then
		fail "$relative must contain exactly one accepted TinyGo installation command"
	fi
	if [[ "$(count_active_occurrences "$tinygo_version_command" "$file")" != 1 ]]; then
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
	"$remote_action_count" "$direct_tinygo_downloads"
