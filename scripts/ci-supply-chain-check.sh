#!/usr/bin/env bash
set -euo pipefail

EXPECTED_TINYGO_VERSION="0.42.0"
EXPECTED_TINYGO_SHA256="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
TINYGO_RELEASE_NAMESPACE="tinygo-org/tinygo/releases/download/"
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

path_has_symlink_component() {
	local current="$1"
	local remainder="$2"
	local component

	while [[ -n "$remainder" ]]; do
		component="${remainder%%/*}"
		current="$current/$component"
		if [[ -L "$current" ]]; then
			return 0
		fi
		if [[ "$remainder" == */* ]]; then
			remainder="${remainder#*/}"
		else
			remainder=""
		fi
	done
	return 1
}

validate_local_uses_ref() {
	local file="$1"
	local line_number="$2"
	local value="$3"
	local kind tail padded target scan_root metadata_count=0

	case "$value" in
		./.github/actions/*)
			kind=action
			scan_root="$ROOT_DIR/.github/actions"
			tail="${value#./.github/actions/}"
			;;
		./.github/workflows/*)
			kind=workflow
			scan_root="$ROOT_DIR/.github/workflows"
			tail="${value#./.github/workflows/}"
			;;
		*)
			fail "${file#"$ROOT_DIR/"}:$line_number local uses reference is outside the scanned .github/actions and .github/workflows roots: $value"
			return
			;;
	esac

	padded="/$tail/"
	if [[ -z "$tail" || "$tail" == *\\* || "$padded" == *'//'* ||
		"$padded" == *'/./'* || "$padded" == *'/../'* ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number local uses reference is not a canonical in-root path: $value"
		return
	fi
	if [[ -L "$scan_root" ]] || path_has_symlink_component "$scan_root" "$tail"; then
		fail "${file#"$ROOT_DIR/"}:$line_number local uses reference traverses a symbolic link outside the file scan: $value"
		return
	fi

	target="$ROOT_DIR/${value#./}"
	if [[ "$kind" == action ]]; then
		if [[ ! -d "$target" ]]; then
			fail "${file#"$ROOT_DIR/"}:$line_number local Action directory does not exist: $value"
			return
		fi
		if [[ -L "$target/action.yml" || -L "$target/action.yaml" ]]; then
			fail "${file#"$ROOT_DIR/"}:$line_number local Action metadata must be a regular file inside the scanned tree: $value"
			return
		fi
		if [[ -f "$target/action.yml" ]]; then
			metadata_count=$((metadata_count + 1))
		fi
		if [[ -f "$target/action.yaml" ]]; then
			metadata_count=$((metadata_count + 1))
		fi
		if (( metadata_count != 1 )); then
			fail "${file#"$ROOT_DIR/"}:$line_number local Action must contain exactly one action.yml or action.yaml: $value"
		fi
		return
	fi

	if [[ ! "$tail" =~ \.ya?ml$ || ! -f "$target" ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number local reusable workflow must name an existing .yml or .yaml file: $value"
	fi
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

tinygo_release_url_owner_allowed() {
	local path="$1"
	local candidate

	for candidate in "${EXPECTED_TINYGO_WORKFLOWS[@]}"; do
		if [[ "$path" == "$candidate" ]]; then
			return 0
		fi
	done
	case "$path" in
		scripts/ci-supply-chain-check.sh | .github/scripts/ci-supply-chain-check.tests.sh)
			return 0
			;;
	esac
	return 1
}

enumerate_repository_files() {
	if [[ "$repository_uses_git_inventory" == true ]]; then
		git -C "$ROOT_DIR" ls-files -z --cached --others --exclude-standard || return
	else
		find "$ROOT_DIR" -path "$ROOT_DIR/.git" -prune -o -type f -print0 || return
	fi
	# An empty record proves that the producer completed successfully.
	printf '\0'
}

repository_uses_git_inventory=false
if command -v git >/dev/null 2>&1 &&
	[[ "$(git -C "$ROOT_DIR" rev-parse --is-inside-work-tree 2>/dev/null || true)" == true ]]; then
	repository_uses_git_inventory=true
fi

repository_inventory_complete=false
while IFS= read -r -d '' repository_path; do
	if [[ -z "$repository_path" ]]; then
		repository_inventory_complete=true
		continue
	fi
	if [[ "$repository_path" == "$ROOT_DIR/"* ]]; then
		relative="${repository_path#"$ROOT_DIR/"}"
		file="$repository_path"
	else
		relative="$repository_path"
		file="$ROOT_DIR/$repository_path"
	fi
	if [[ ! -f "$file" ]]; then
		continue
	fi
	if grep -Fq -- "$TINYGO_RELEASE_NAMESPACE" "$file"; then
		if ! tinygo_release_url_owner_allowed "$relative"; then
			fail "TinyGo release-download namespace is not owned by $relative"
		fi
	else
		grep_status=$?
		if (( grep_status > 1 )); then
			fail "could not inspect repository file for TinyGo release downloads: $relative"
		fi
	fi
done < <(enumerate_repository_files)
if [[ "$repository_inventory_complete" != true ]]; then
	fail "could not enumerate repository-owned files for TinyGo release downloads"
fi

scan_roots=()
for candidate in "$ROOT_DIR/.github/workflows" "$ROOT_DIR/.github/actions"; do
	if [[ -d "$candidate" ]]; then
		if [[ -L "$candidate" ]]; then
			fail "authored GitHub workflow and Action scan root must not be a symbolic link: ${candidate#"$ROOT_DIR/"}"
		else
			scan_roots+=("$candidate")
		fi
	fi
done
if (( ${#scan_roots[@]} == 0 )); then
	fail "no authored GitHub workflow or Action directories found under $ROOT_DIR/.github"
fi

scan_action_refs() {
	local file="$1"
	local line line_number=0 indent syntax key value sequence_prefix content
	local comment comment_is_yaml
	local scalar_indent=-1
	local mapping_re='^(-[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*):([[:space:]]+(.*))?$'
	local scalar_re='^[|>][+-]?$'
	local inline_comment_re='^(.*[^[:space:]])[[:space:]]+#(.*)$'
	local full_comment_re='^[[:space:]]*#(.*)$'
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		content="$line"
		comment=""
		comment_is_yaml=false
		if [[ "$line" =~ $inline_comment_re ]]; then
			content="${BASH_REMATCH[1]}"
			comment="${BASH_REMATCH[2]}"
			comment_is_yaml=true
		elif [[ "$line" =~ $full_comment_re ]]; then
			content=""
			comment="${BASH_REMATCH[1]}"
			comment_is_yaml=true
		fi
		syntax="$(trim_whitespace "$content")"
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
			validate_local_uses_ref "$file" "$line_number" "$value"
			continue
		fi

		remote_action_count=$((remote_action_count + 1))
		if [[ ! "$value" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[^@[:space:]]+)?@[0-9a-f]{40}$ ]]; then
			fail "${file#"$ROOT_DIR/"}:$line_number remote uses reference is not pinned to a lowercase 40-character commit SHA: $value"
		fi
		comment="$(trim_whitespace "$comment")"
		if [[ "$comment_is_yaml" != true || ! "$comment" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
			fail "${file#"$ROOT_DIR/"}:$line_number remote uses reference must have a trailing # vMAJOR.MINOR.PATCH annotation: $value"
		fi
	done < "$file"
}

direct_tinygo_downloads=0
if (( ${#scan_roots[@]} > 0 )); then
	# Traversal order affects diagnostics only; keep filenames NUL-delimited.
	while IFS= read -r -d '' file; do
		scan_action_refs "$file"
		download_count="$(count_active_occurrences "$TINYGO_RELEASE_NAMESPACE" "$file")"
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

tinygo_download_command="curl -fsSL -o /tmp/tinygo.deb \"https://github.com/tinygo-org/tinygo/releases/download/v${EXPECTED_TINYGO_VERSION}/tinygo_${EXPECTED_TINYGO_VERSION}_amd64.deb\""
tinygo_verify_command="printf '%s  %s\\n' '$EXPECTED_TINYGO_SHA256' /tmp/tinygo.deb | sha256sum --check --strict - || exit 1"
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
