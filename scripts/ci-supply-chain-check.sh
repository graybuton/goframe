#!/usr/bin/env bash
set -euo pipefail

EXPECTED_TINYGO_VERSION="0.42.0"
EXPECTED_TINYGO_SHA256="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
TINYGO_RELEASE_REPOSITORY="tinygo-org/tinygo"
TINYGO_RELEASE_NAMESPACE="${TINYGO_RELEASE_REPOSITORY}/releases/download/"
EXPECTED_TINYGO_WORKFLOWS=(
	".github/workflows/ci-browser-smoke.yml"
	".github/workflows/ci-core.yml"
	".github/workflows/ci-wasm-size.yml"
)
TINYGO_VERIFIED_DEB="/tmp/goframe-tinygo-${EXPECTED_TINYGO_VERSION}-verified.deb"
tinygo_shell_template='/usr/bin/env -i PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin /bin/bash --noprofile --norc -e -u -o pipefail {0}'
tinygo_cleanup_command="trap '/usr/bin/sudo /usr/bin/rm -f $TINYGO_VERIFIED_DEB' EXIT"
tinygo_download_command="/usr/bin/curl -fsSL -o /tmp/tinygo.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v${EXPECTED_TINYGO_VERSION}/tinygo_${EXPECTED_TINYGO_VERSION}_amd64.deb\""
tinygo_stage_command="/usr/bin/sudo /usr/bin/install -o root -g root -m 0644 /tmp/tinygo.deb $TINYGO_VERIFIED_DEB"
tinygo_verify_command="/usr/bin/env -i /usr/bin/sha256sum --check --strict - <<< '$EXPECTED_TINYGO_SHA256  $TINYGO_VERIFIED_DEB' || exit 1"
tinygo_install_command="/usr/bin/sudo /usr/bin/apt-get install -y $TINYGO_VERIFIED_DEB"
tinygo_version_command='tinygo version'

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
loopback_download_count=0

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
	return 1
}

shell_downloader_re='(^|;|&&|\|\||\||\(|\{|\$\(|`)[[:space:]]*([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]+[[:space:]]+)*((if|elif|while|until|then|do|!)[[:space:]]+)*(command[[:space:]]+)?(/usr/bin/)?(curl|wget)([[:space:]]|$)'

shell_command_has_active_downloader() {
	local command="$1"
	# This is an authored-shell boundary, not a shell interpreter. Match command
	# positions used by repository CI and scripts, including substitutions.
	[[ "$command" =~ $shell_downloader_re ]]
}

shell_command_has_multiple_active_downloaders() {
	local remainder="$1"
	local first_match

	if [[ ! "$remainder" =~ $shell_downloader_re ]]; then
		return 1
	fi
	first_match="${BASH_REMATCH[0]}"
	remainder="${remainder#*"$first_match"}"
	[[ "$remainder" =~ $shell_downloader_re ]]
}

shell_command_is_loopback_probe() {
	local command="$1"
	local direct_re='^(if[[:space:]]+)?(command[[:space:]]+)?(/usr/bin/)?curl([[:space:]]+-[A-Za-z]+)*[[:space:]]+"http://127[.]0[.]0[.]1:[^"]*"[[:space:]]*(>/dev/null[[:space:]]+2>&1)?[[:space:]]*(;[[:space:]]*then)?$'
	local substitution_re='^[A-Za-z_][A-Za-z0-9_]*="\$\([[:space:]]*(command[[:space:]]+)?(/usr/bin/)?curl([[:space:]]+-[A-Za-z]+)*[[:space:]]+"http://127[.]0[.]0[.]1:[^"]*"[[:space:]]*(\|\|[[:space:]]+true)?[[:space:]]*\)"$'
	if shell_command_has_multiple_active_downloaders "$command"; then
		return 1
	fi
	[[ "$command" =~ $direct_re || "$command" =~ $substitution_re ]]
}

inspect_shell_logical_command() {
	local relative="$1"
	local line_number="$2"
	local command="$3"

	command="$(trim_whitespace "$command")"
	if [[ -z "$command" ]] || ! shell_command_has_active_downloader "$command"; then
		return
	fi
	if tinygo_release_url_owner_allowed "$relative" &&
		[[ "$command" == "$tinygo_download_command" ]]; then
		return
	fi
	if shell_command_is_loopback_probe "$command"; then
		loopback_download_count=$((loopback_download_count + 1))
		return
	fi
	fail "$relative:$line_number active curl/wget command is neither the accepted TinyGo fetch nor a literal-rooted loopback probe"
}

pending_shell_command=""
pending_shell_line=0

reset_shell_logical_command() {
	pending_shell_command=""
	pending_shell_line=0
}

consume_shell_physical_line() {
	local relative="$1"
	local line_number="$2"
	local line="$3"
	local command

	command="$(trim_whitespace "$line")"
	if [[ -z "$pending_shell_command" && ( -z "$command" || "$command" == \#* ) ]]; then
		return
	fi
	if [[ -z "$pending_shell_command" ]]; then
		pending_shell_line="$line_number"
	fi
	if [[ "$command" == *\\ ]]; then
		command="${command%\\}"
		pending_shell_command="${pending_shell_command}${pending_shell_command:+ }$command"
		return
	fi
	pending_shell_command="${pending_shell_command}${pending_shell_command:+ }$command"
	inspect_shell_logical_command "$relative" "$pending_shell_line" "$pending_shell_command"
	reset_shell_logical_command
}

flush_shell_logical_command() {
	local relative="$1"
	if [[ -n "$pending_shell_command" ]]; then
		inspect_shell_logical_command "$relative" "$pending_shell_line" "$pending_shell_command"
	fi
	reset_shell_logical_command
}

scan_shell_downloaders() {
	local file="$1"
	local relative="$2"
	local line line_number=0

	reset_shell_logical_command
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		consume_shell_physical_line "$relative" "$line_number" "$line"
	done < "$file"
	flush_shell_logical_command "$relative"
}

scan_workflow_downloaders() {
	local file="$1"
	local relative="${file#"$ROOT_DIR/"}"
	local line line_number=0 indent syntax key value sequence_prefix content
	local scalar_indent=-1 run_scalar=false
	local mapping_re='^(-[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*):([[:space:]]+(.*))?$'
	local scalar_re='^[|>]([1-9][+-]?|[+-][1-9]?)?$'

	reset_shell_logical_command
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		indent="${line%%[![:space:]]*}"
		syntax="$(trim_whitespace "$line")"
		if (( scalar_indent >= 0 )) &&
			[[ -z "$syntax" || ${#indent} -gt scalar_indent ]]; then
			if [[ "$run_scalar" == true ]]; then
				consume_shell_physical_line "$relative" "$line_number" "$line"
			fi
			continue
		fi
		if [[ "$run_scalar" == true ]]; then
			flush_shell_logical_command "$relative"
		fi
		scalar_indent=-1
		run_scalar=false
		content="$syntax"
		if [[ "$syntax" == \#* ]]; then
			continue
		fi
		if [[ ! "$content" =~ $mapping_re ]]; then
			continue
		fi
		sequence_prefix="${BASH_REMATCH[1]}"
		key="${BASH_REMATCH[2]}"
		value="${BASH_REMATCH[4]}"
		if [[ "$value" =~ $scalar_re ]]; then
			scalar_indent=$((${#indent} + ${#sequence_prefix}))
			if [[ "$key" == run ]]; then
				run_scalar=true
			fi
			continue
		fi
		if [[ "$key" != run || -z "$value" ]]; then
			continue
		fi
		if (( ${#value} >= 2 )); then
			if [[ "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
				value="${value:1:${#value}-2}"
			elif [[ "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
				value="${value:1:${#value}-2}"
			fi
		fi
		consume_shell_physical_line "$relative" "$line_number" "$value"
	done < "$file"
	if [[ "$run_scalar" == true ]]; then
		flush_shell_logical_command "$relative"
	fi
}

repository_file_is_shell_source() {
	local relative="$1"
	local file="$2"
	local first_line

	case "$relative" in
		*.sh|*.bash) return 0 ;;
	esac
	if [[ "${relative##*/}" == *.* ]]; then
		return 1
	fi
	IFS= read -r first_line < "$file" || true
	[[ "$first_line" =~ ^\#!.*(bash|/sh)([[:space:]]|$) ]]
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
	if repository_file_is_shell_source "$relative" "$file"; then
		scan_shell_downloaders "$file" "$relative"
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
	local scalar_re='^[|>]([1-9][+-]?|[+-][1-9]?)?$'
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
		scan_workflow_downloaders "$file"
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

count_tinygo_install_sequences() {
	local file="$1"
	local line command indent step_indent property_indent command_indent expected_command valid_indent
	local state=0 count=0
	while IFS= read -r line || [[ -n "$line" ]]; do
		command="$(trim_whitespace "$line")"
		indent="${line%%[![:space:]]*}"
		if (( state > 0 )); then
			if (( state == 8 )) && [[ -z "$command" ]]; then
				continue
			fi
			case "$state" in
				1) expected_command="shell: $tinygo_shell_template"; property_indent="$indent" ;;
				2) expected_command='run: |' ;;
				3) expected_command="$tinygo_cleanup_command"; command_indent="$indent" ;;
				4) expected_command="$tinygo_download_command" ;;
				5) expected_command="$tinygo_stage_command" ;;
				6) expected_command="$tinygo_verify_command" ;;
				7) expected_command="$tinygo_install_command" ;;
				8) expected_command='- name: Verify TinyGo' ;;
				9) expected_command="run: $tinygo_version_command" ;;
			esac
			if (( state == 1 )); then
				valid_indent=false
				if [[ "$indent" == "$step_indent"* ]] &&
					(( ${#indent} > ${#step_indent} )); then
					valid_indent=true
				fi
			elif (( state == 2 || state == 9 )); then
				valid_indent=false
				if [[ "$indent" == "$property_indent" ]]; then
					valid_indent=true
				fi
			elif (( state == 8 )); then
				valid_indent=false
				if [[ "$indent" == "$step_indent" ]]; then
					valid_indent=true
				fi
			else
				valid_indent=false
				if [[ "$indent" == "$command_indent" && "$indent" == "$property_indent"* ]] &&
					(( ${#indent} > ${#property_indent} )); then
					valid_indent=true
				fi
			fi
			if [[ "$command" == "$expected_command" && "$valid_indent" == true ]]; then
				state=$((state + 1))
				if (( state == 10 )); then
					count=$((count + 1))
					state=0
				fi
			else
				state=0
			fi
		fi
		if [[ "$command" == '- name: Install TinyGo' ]]; then
			step_indent="$indent"
			state=1
		fi
	done < "$file"

	printf '%d' "$count"
}

accepted_tinygo_install_sequences=0
for relative in "${EXPECTED_TINYGO_WORKFLOWS[@]}"; do
	file="$ROOT_DIR/$relative"
	if [[ ! -f "$file" ]]; then
		fail "required TinyGo workflow is missing: $relative"
		continue
	fi

	if [[ "$(count_active_occurrences "$tinygo_download_command" "$file")" != 1 ]]; then
		fail "$relative must contain exactly one accepted TinyGo download command"
	fi
	if [[ "$(count_active_occurrences "$tinygo_cleanup_command" "$file")" != 1 ]]; then
		fail "$relative must contain exactly one accepted TinyGo cleanup command"
	fi
	if [[ "$(count_active_occurrences "$tinygo_stage_command" "$file")" != 1 ]]; then
		fail "$relative must contain exactly one accepted TinyGo staging command"
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
		fail "$relative must contain exactly one sanitized TinyGo staging, verification, installation, and cleanup sequence immediately followed by an ordinary TinyGo version smoke"
	else
		accepted_tinygo_install_sequences=$((accepted_tinygo_install_sequences + 1))
	fi
done

if (( failures != 0 )); then
	exit 1
fi

printf 'ci supply-chain check: ok (%d remote Action refs, %d verified TinyGo downloads, %d protected install sequences, %d loopback probes)\n' \
	"$remote_action_count" "$direct_tinygo_downloads" \
	"$accepted_tinygo_install_sequences" "$loopback_download_count"
