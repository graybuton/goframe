#!/usr/bin/env bash
set -euo pipefail

EXPECTED_TINYGO_VERSION="0.42.0"
EXPECTED_TINYGO_SHA256="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
TINYGO_RELEASE_REPOSITORY="tinygo-org/"'tinygo'
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
tinygo_verify_shell_template='/bin/bash --noprofile --norc -p -e -o pipefail {0}'
tinygo_version_command='/usr/local/bin/tinygo version'

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
	inspect_referenced_ci_helpers "$relative" "$line_number" "$command"
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

repository_file_is_helper_source() {
	local relative="$1"
	local file="$2"
	local first_line

	case "$relative" in
		*.sh|*.bash|*.py|*.js|*.mjs|*.cjs|*.ps1|*.rb|*.go|*.ts|*.tsx)
			return 0
			;;
	esac
	if [[ -x "$file" ]]; then
		return 0
	fi
	IFS= read -r first_line < "$file" || true
	[[ "$first_line" == '#!'* ]]
}

repository_file_is_ci_executable_source() {
	local relative="$1"
	local file="$2"

	# This is a bounded authored-CI provenance inventory, not language parsing.
	# Common helper-language files and executable files are inspected regardless
	# of invocation; additional source types are included under CI helper roots.
	case "$relative" in
		*.sh|*.bash|*.py|*.js|*.mjs|*.cjs|*.ps1|*.rb) return 0 ;;
	esac
	case "$relative" in
		.github/scripts/*|.github/actions/*|scripts/*)
			repository_file_is_helper_source "$relative" "$file"
			return
			;;
	esac
	[[ -x "$file" ]]
}

repository_file_is_authored_ci_yaml() {
	local relative="$1"

	case "$relative" in
		.github/workflows/*.yml|.github/workflows/*.yaml|\
		.github/actions/*.yml|.github/actions/*.yaml|\
		.github/actions/*/*.yml|.github/actions/*/*.yaml)
			return 0
			;;
	esac
	return 1
}

scan_ci_executable_tinygo_provenance() {
	local relative="$1"
	local file="$2"
	local forced="${3:-false}"
	local grep_status line syntax

	# A literal repository identity is the bounded, language-agnostic marker.
	# Deliberately encoded or character-by-character construction is outside this
	# authored-CI policy; arbitrary source-language data flow is not interpreted.
	if [[ "$forced" != true ]] &&
		! repository_file_is_ci_executable_source "$relative" "$file" &&
		! repository_file_is_authored_ci_yaml "$relative"; then
		return
	fi
	if grep -Fiq -- "$TINYGO_RELEASE_REPOSITORY" "$file"; then
		if tinygo_release_url_owner_allowed "$relative"; then
			while IFS= read -r line || [[ -n "$line" ]]; do
				if ! grep -Fiq -- "$TINYGO_RELEASE_REPOSITORY" <<< "$line"; then
					continue
				fi
				syntax="$(trim_whitespace "$line")"
				if [[ "$syntax" != "$tinygo_download_command" ]]; then
					fail "TinyGo repository provenance in $relative is outside the accepted fetch command"
				fi
			done < "$file"
		else
			fail "TinyGo repository provenance is not allowed in executable CI source: $relative"
		fi
	else
		grep_status=$?
		if (( grep_status > 1 )); then
			fail "could not inspect executable CI source for TinyGo provenance: $relative"
		fi
	fi
}

inspect_referenced_ci_helpers() {
	local source_relative="$1"
	local line_number="$2"
	local command="$3"
	local token relative file
	local words=()

	read -r -a words <<< "$command"
	for token in "${words[@]}"; do
		token="${token#\"}"
		token="${token%\"}"
		token="${token#\'}"
		token="${token%\'}"
		token="${token#(}"
		token="${token%)}"
		token="${token%;}"
		token="${token%|}"
		token="${token%&}"
		case "$token" in
			http://*|https://*|/*|*'$'*|*'`'*) continue ;;
		esac
		relative="${token#./}"
		case "/$relative/" in
			*'/../'*|*'/./'*|*'//'*)
				file="$ROOT_DIR/$relative"
				case "$relative" in
					*.sh|*.bash|*.py|*.js|*.mjs|*.cjs|*.ps1|*.rb|*.go|*.ts|*.tsx)
						fail "$source_relative:$line_number referenced CI helper path is not canonical: $token"
						;;
					*)
						if [[ -f "$file" && -x "$file" ]]; then
							fail "$source_relative:$line_number referenced executable CI helper path is not canonical: $token"
						fi
						;;
				esac
				continue
				;;
		esac
		file="$ROOT_DIR/$relative"
		if [[ ! -f "$file" ]] ||
			! repository_file_is_helper_source "$relative" "$file"; then
			continue
		fi
		if repository_file_is_ci_executable_source "$relative" "$file"; then
			continue
		fi
		scan_ci_executable_tinygo_provenance "$relative" "$file" true
	done
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
	scan_ci_executable_tinygo_provenance "$relative" "$file"
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

validate_action_uses_ref() {
	local file="$1"
	local line_number="$2"
	local value="$3"
	local comment="$4"
	local comment_is_yaml="$5"

	if (( ${#value} >= 2 )); then
		if [[ "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
			value="${value:1:${#value}-2}"
		elif [[ "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
			value="${value:1:${#value}-2}"
		fi
	fi

	if [[ -z "$value" ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number has an empty uses reference"
		return
	fi
	if [[ "$value" == ./* ]]; then
		validate_local_uses_ref "$file" "$line_number" "$value"
		return
	fi

	remote_action_count=$((remote_action_count + 1))
	if [[ ! "$value" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[^@[:space:]]+)?@[0-9a-f]{40}$ ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number remote uses reference is not pinned to a lowercase 40-character commit SHA: $value"
	fi
	comment="$(trim_whitespace "$comment")"
	if [[ "$comment_is_yaml" != true || ! "$comment" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number remote uses reference must have a trailing # vMAJOR.MINOR.PATCH annotation: $value"
	fi
}

scan_action_refs() {
	local file="$1"
	local relative="${file#"$ROOT_DIR/"}"
	local line line_number=0 indent syntax key value sequence_prefix content
	local comment comment_is_yaml key_indent stack_count stack_index parent_kind
	local is_action_uses=false
	local action_step_active=false action_step_indent=-1 action_step_kind=""
	local scalar_indent=-1
	local mapping_re='^(-[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*):([[:space:]]+(.*))?$'
	local scalar_re='^[|>]([1-9][+-]?|[+-][1-9]?)?$'
	local inline_comment_re='^(.*[^[:space:]])[[:space:]]+#(.*)$'
	local full_comment_re='^[[:space:]]*#(.*)$'
	local path_keys=()
	local path_indents=()

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

		if [[ "$syntax" =~ $mapping_re ]]; then
			sequence_prefix="${BASH_REMATCH[1]}"
			key="${BASH_REMATCH[2]}"
			value="${BASH_REMATCH[4]}"
			key_indent=$((${#indent} + ${#sequence_prefix}))
		else
			sequence_prefix=""
			key_indent=${#indent}
			if [[ "$syntax" == '- '* ]]; then
				key_indent=$((key_indent + 2))
			fi
		fi

		while (( ${#path_indents[@]} > 0 )); do
			stack_index=$((${#path_indents[@]} - 1))
			if (( path_indents[stack_index] < key_indent )); then
				break
			fi
			unset 'path_indents[stack_index]' 'path_keys[stack_index]'
		done
		stack_count=${#path_keys[@]}
		parent_kind=""
		if [[ "$relative" == .github/workflows/* ]] &&
			(( stack_count == 3 )) &&
			[[ "${path_keys[0]}" == jobs && "${path_keys[2]}" == steps ]]; then
			parent_kind=workflow-step
		elif [[ "$relative" == .github/actions/* ]] &&
			(( stack_count == 2 )) &&
			[[ "${path_keys[0]}" == runs && "${path_keys[1]}" == steps ]]; then
			parent_kind=composite-step
		fi

		if [[ "$action_step_active" == true ]] &&
			(( key_indent < action_step_indent )); then
			action_step_active=false
			action_step_kind=""
		fi

		# Only canonical block mapping keys are authored for security-relevant
		# structure. Ordinary scalar/list data remains outside Action semantics.
		if [[ ! "$syntax" =~ $mapping_re ]]; then
			case "${syntax#- }" in
				*:*|\{*|\[*|\?*|\&*|\**|\!*)
					fail "$relative:$line_number unsupported workflow mapping syntax; use canonical block keys"
					;;
			esac
			continue
		fi

		if [[ -n "$sequence_prefix" && -n "$parent_kind" ]]; then
			action_step_active=true
			action_step_indent=$key_indent
			action_step_kind="$parent_kind"
		elif [[ -n "$sequence_prefix" && "$action_step_active" == true ]] &&
			(( key_indent <= action_step_indent )); then
			action_step_active=false
			action_step_kind=""
		fi

		case "$value" in
			\{*|\[*|\&*|\**|\!*)
				case "$key" in
					env|with|inputs|outputs) ;;
					*)
						fail "$relative:$line_number unsupported workflow mapping value; use canonical block mappings"
						;;
				esac
			continue
			;;
		esac

		is_action_uses=false
		if [[ "$key" == uses ]]; then
			if [[ "$relative" == .github/workflows/* ]] &&
				(( stack_count == 2 )) &&
				[[ "${path_keys[0]}" == jobs ]]; then
				is_action_uses=true
			elif [[ "$action_step_active" == true &&
				"$action_step_kind" == "$parent_kind" ]] &&
				(( key_indent == action_step_indent )); then
				is_action_uses=true
			fi
		fi
		if [[ "$is_action_uses" == true ]]; then
			validate_action_uses_ref "$file" "$line_number" "$value" \
				"$comment" "$comment_is_yaml"
		fi

		if [[ "$value" =~ $scalar_re ]]; then
			scalar_indent=$key_indent
			continue
		fi
		if [[ -z "$value" ]]; then
			path_indents[stack_count]=$key_indent
			path_keys[stack_count]="$key"
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

reset_tinygo_step() {
	tinygo_step_active=false
	tinygo_step_indent=-1
	tinygo_step_line=0
	tinygo_step_property_count=0
	tinygo_step_name_count=0
	tinygo_step_shell_count=0
	tinygo_step_run_count=0
	tinygo_step_install_name=false
	tinygo_step_verify_name=false
	tinygo_step_shell_value=""
	tinygo_step_run_value=""
	tinygo_step_run_lines=()
}

reset_tinygo_job() {
	tinygo_job_active=false
	tinygo_job_indent=-1
	tinygo_job_line=0
	tinygo_job_id=""
	tinygo_job_sequence_count=0
	tinygo_job_if_line=0
	tinygo_job_continue_on_error_line=0
	tinygo_job_needs_line=0
}

record_tinygo_job_property() {
	local key="$1"
	local line_number="$2"

	case "$key" in
		if)
			if (( tinygo_job_if_line == 0 )); then
				tinygo_job_if_line=$line_number
			fi
			;;
		continue-on-error)
			if (( tinygo_job_continue_on_error_line == 0 )); then
				tinygo_job_continue_on_error_line=$line_number
			fi
			;;
		needs)
			if (( tinygo_job_needs_line == 0 )); then
				tinygo_job_needs_line=$line_number
			fi
			;;
	esac
}

finalize_tinygo_job() {
	local relative="$1"

	if [[ "$tinygo_job_active" != true ]]; then
		return
	fi
	if [[ "$tinygo_pending_install" == true ]]; then
		fail "$relative:$tinygo_job_line protected TinyGo job $tinygo_job_id must keep Verify TinyGo immediately after Install TinyGo"
		tinygo_pending_install=false
	fi
	if (( tinygo_job_sequence_count > 0 )); then
		if (( tinygo_job_if_line > 0 )); then
			fail "$relative:$tinygo_job_if_line protected TinyGo job $tinygo_job_id must be unconditional and blocking: contains job-level if"
		fi
		if (( tinygo_job_continue_on_error_line > 0 )); then
			fail "$relative:$tinygo_job_continue_on_error_line protected TinyGo job $tinygo_job_id must be unconditional and blocking: contains job-level continue-on-error"
		fi
		if (( tinygo_job_needs_line > 0 )); then
			fail "$relative:$tinygo_job_needs_line protected TinyGo job $tinygo_job_id must be unconditional and blocking: contains job-level needs"
		fi
	fi
	reset_tinygo_job
}

record_tinygo_step_property() {
	local key="$1"
	local value="$2"

	tinygo_step_property_count=$((tinygo_step_property_count + 1))
	case "$key" in
		name)
			tinygo_step_name_count=$((tinygo_step_name_count + 1))
			if [[ "$value" == 'Install TinyGo' ]]; then
				tinygo_step_install_name=true
			elif [[ "$value" == 'Verify TinyGo' ]]; then
				tinygo_step_verify_name=true
			fi
			;;
		shell)
			tinygo_step_shell_count=$((tinygo_step_shell_count + 1))
			tinygo_step_shell_value="$value"
			;;
		run)
			tinygo_step_run_count=$((tinygo_step_run_count + 1))
			tinygo_step_run_value="$value"
			;;
	esac
}

tinygo_install_body_is_exact() {
	(( ${#tinygo_step_run_lines[@]} == 5 )) || return 1
	[[ "${tinygo_step_run_lines[0]}" == "$tinygo_cleanup_command" ]] || return 1
	[[ "${tinygo_step_run_lines[1]}" == "$tinygo_download_command" ]] || return 1
	[[ "${tinygo_step_run_lines[2]}" == "$tinygo_stage_command" ]] || return 1
	[[ "${tinygo_step_run_lines[3]}" == "$tinygo_verify_command" ]] || return 1
	[[ "${tinygo_step_run_lines[4]}" == "$tinygo_install_command" ]]
}

finalize_tinygo_step() {
	local relative="$1"
	local valid=true

	if [[ "$tinygo_step_active" != true ]]; then
		return
	fi
	if [[ "$tinygo_step_install_name" == true &&
		"$tinygo_step_verify_name" == true ]]; then
		fail "$relative:$tinygo_step_line TinyGo step has conflicting duplicate names"
		tinygo_pending_install=false
		reset_tinygo_step
		return
	fi

	if [[ "$tinygo_step_install_name" == true ]]; then
		if [[ "$tinygo_pending_install" == true ]]; then
			fail "$relative:$tinygo_step_line Verify TinyGo must immediately follow Install TinyGo"
		fi
		if (( tinygo_step_property_count != 3 ||
			tinygo_step_name_count != 1 ||
			tinygo_step_shell_count != 1 ||
			tinygo_step_run_count != 1 )); then
			valid=false
		fi
		if [[ "$tinygo_step_shell_value" != "$tinygo_shell_template" ||
			"$tinygo_step_run_value" != '|' ]] ||
			! tinygo_install_body_is_exact; then
			valid=false
		fi
		if [[ "$valid" != true ]]; then
			fail "$relative:$tinygo_step_line Install TinyGo must contain only the exact sanitized shell and protected run transaction"
			tinygo_pending_install=false
		else
			tinygo_pending_install=true
		fi
	elif [[ "$tinygo_step_verify_name" == true ]]; then
		if (( tinygo_step_property_count != 3 ||
			tinygo_step_name_count != 1 ||
			tinygo_step_shell_count != 1 ||
			tinygo_step_run_count != 1 )) ||
			[[ "$tinygo_step_shell_value" != "$tinygo_verify_shell_template" ||
				"$tinygo_step_run_value" != "$tinygo_version_command" ]]; then
			valid=false
		fi
		if [[ "$valid" != true ]]; then
			fail "$relative:$tinygo_step_line Verify TinyGo must contain only the exact privileged shell and package-owned version command"
			tinygo_pending_install=false
		elif [[ "$tinygo_pending_install" != true ]]; then
			fail "$relative:$tinygo_step_line Verify TinyGo must immediately follow Install TinyGo"
		elif [[ "$tinygo_job_active" != true ]]; then
			fail "$relative:$tinygo_step_line Verify TinyGo is outside a workflow job"
			tinygo_pending_install=false
		else
			tinygo_sequence_count=$((tinygo_sequence_count + 1))
			tinygo_job_sequence_count=$((tinygo_job_sequence_count + 1))
			tinygo_pending_install=false
		fi
	elif [[ "$tinygo_pending_install" == true ]]; then
		fail "$relative:$tinygo_step_line Verify TinyGo must immediately follow Install TinyGo"
		tinygo_pending_install=false
	fi
	reset_tinygo_step
}

scan_tinygo_install_steps() {
	local file="$1"
	local relative="${file#"$ROOT_DIR/"}"
	local line line_number=0 content syntax indent key value sequence_prefix
	local key_indent stack_count stack_index parent_is_steps=false new_job=false
	local scalar_indent=-1 scalar_is_step_run=false
	local mapping_re='^(-[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*):([[:space:]]+(.*))?$'
	local scalar_re='^[|>]([1-9][+-]?|[+-][1-9]?)?$'
	local inline_comment_re='^(.*[^[:space:]])[[:space:]]+#(.*)$'
	local full_comment_re='^[[:space:]]*#(.*)$'
	local path_keys=()
	local path_indents=()

	tinygo_sequence_count=0
	tinygo_pending_install=false
	reset_tinygo_step
	reset_tinygo_job
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		indent="${line%%[![:space:]]*}"
		syntax="$(trim_whitespace "$line")"
		if (( scalar_indent >= 0 )) &&
			[[ -z "$syntax" || ${#indent} -gt scalar_indent ]]; then
			if [[ "$scalar_is_step_run" == true &&
				-n "$syntax" && "$syntax" != \#* ]]; then
				tinygo_step_run_lines[${#tinygo_step_run_lines[@]}]="$syntax"
			fi
			continue
		fi
		scalar_indent=-1
		scalar_is_step_run=false

		content="$line"
		if [[ "$line" =~ $inline_comment_re ]]; then
			content="${BASH_REMATCH[1]}"
		elif [[ "$line" =~ $full_comment_re ]]; then
			content=""
		fi
		syntax="$(trim_whitespace "$content")"
		if [[ -z "$syntax" || ! "$syntax" =~ $mapping_re ]]; then
			continue
		fi
		sequence_prefix="${BASH_REMATCH[1]}"
		key="${BASH_REMATCH[2]}"
		value="${BASH_REMATCH[4]}"
		key_indent=$((${#indent} + ${#sequence_prefix}))

		while (( ${#path_indents[@]} > 0 )); do
			stack_index=$((${#path_indents[@]} - 1))
			if (( path_indents[stack_index] < key_indent )); then
				break
			fi
			unset 'path_indents[stack_index]' 'path_keys[stack_index]'
		done
		stack_count=${#path_keys[@]}
		parent_is_steps=false
		if (( stack_count == 3 )) &&
			[[ "${path_keys[0]}" == jobs && "${path_keys[2]}" == steps ]]; then
			parent_is_steps=true
		fi
		new_job=false
		if [[ -z "$sequence_prefix" ]] &&
			(( stack_count == 1 )) &&
			[[ "${path_keys[0]}" == jobs ]]; then
			new_job=true
		fi

		if [[ "$tinygo_step_active" == true ]] &&
			(( key_indent < tinygo_step_indent )); then
			finalize_tinygo_step "$relative"
		fi
		if [[ "$tinygo_job_active" == true ]] &&
			(( key_indent <= tinygo_job_indent )); then
			finalize_tinygo_job "$relative"
		fi
		if [[ "$new_job" == true ]]; then
			tinygo_job_active=true
			tinygo_job_indent=$key_indent
			tinygo_job_line=$line_number
			tinygo_job_id="$key"
		fi
		if [[ "$tinygo_job_active" == true ]] &&
			(( stack_count == 2 )) &&
			[[ "${path_keys[0]}" == jobs &&
				"${path_keys[1]}" == "$tinygo_job_id" ]] &&
			(( key_indent > tinygo_job_indent )); then
			record_tinygo_job_property "$key" "$line_number"
		fi
		if [[ -n "$sequence_prefix" && "$parent_is_steps" == true ]]; then
			finalize_tinygo_step "$relative"
			tinygo_step_active=true
			tinygo_step_indent=$key_indent
			tinygo_step_line=$line_number
		fi

		if [[ "$tinygo_step_active" == true ]] &&
			(( key_indent == tinygo_step_indent )); then
			record_tinygo_step_property "$key" "$value"
		fi
		if [[ "$value" =~ $scalar_re ]]; then
			scalar_indent=$key_indent
			if [[ "$tinygo_step_active" == true &&
				"$key" == run ]] &&
				(( key_indent == tinygo_step_indent )); then
				scalar_is_step_run=true
			fi
			continue
		fi
		if [[ -z "$value" ]]; then
			path_indents[stack_count]=$key_indent
			path_keys[stack_count]="$key"
		fi
	done < "$file"
	finalize_tinygo_step "$relative"
	finalize_tinygo_job "$relative"
	if [[ "$tinygo_pending_install" == true ]]; then
		fail "$relative Verify TinyGo must immediately follow Install TinyGo"
		tinygo_pending_install=false
	fi
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

	scan_tinygo_install_steps "$file"
	sequence_count="$tinygo_sequence_count"
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
