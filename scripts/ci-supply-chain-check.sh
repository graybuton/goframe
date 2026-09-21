#!/usr/bin/env bash
set -euo pipefail

EXPECTED_TINYGO_VERSION="0.42.0"
EXPECTED_TINYGO_SHA256="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
TINYGO_RELEASE_REPOSITORY="tinygo-org/"'tinygo'
TINYGO_RELEASE_NAMESPACE="${TINYGO_RELEASE_REPOSITORY}/releases/download/"
CURL_COMMAND_NAME='cur''l'
WGET_COMMAND_NAME='wg''et'
EVAL_COMMAND_NAME='ev''al'
EXPECTED_TINYGO_WORKFLOWS=(
	".github/workflows/ci-browser-smoke.yml"
	".github/workflows/ci-core.yml"
	".github/workflows/ci-wasm-size.yml"
)
TINYGO_VERIFIED_DEB="/tmp/goframe-tinygo-${EXPECTED_TINYGO_VERSION}-verified.deb"
tinygo_shell_template='/usr/bin/env -i PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin /bin/bash --noprofile --norc -e -u -o pipefail {0}'
tinygo_cleanup_command="trap '/usr/bin/sudo /usr/bin/rm -f $TINYGO_VERIFIED_DEB' EXIT"
tinygo_download_command="/usr/bin/$CURL_COMMAND_NAME -fsSL -o /tmp/tinygo.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v${EXPECTED_TINYGO_VERSION}/tinygo_${EXPECTED_TINYGO_VERSION}_amd64.deb\""
tinygo_stage_command="/usr/bin/sudo /usr/bin/install -o root -g root -m 0644 /tmp/tinygo.deb $TINYGO_VERIFIED_DEB"
tinygo_verify_command="/usr/bin/env -i /usr/bin/sha256sum --check --strict - <<< '$EXPECTED_TINYGO_SHA256  $TINYGO_VERIFIED_DEB' || exit 1"
tinygo_install_command="/usr/bin/sudo /usr/bin/apt-get install -y $TINYGO_VERIFIED_DEB"
tinygo_verify_shell_template='/bin/bash --noprofile --norc -p -e -o pipefail {0}'
tinygo_version_command='/usr/local/bin/tinygo version'
yaml_block_scalar_re='^[|>]([1-9][+-]?|[+-][1-9]?)?$'

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
	local kind tail padded target scan_root metadata_count=0 metadata=""

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
			metadata="$target/action.yml"
		fi
		if [[ -f "$target/action.yaml" ]]; then
			metadata_count=$((metadata_count + 1))
			metadata="$target/action.yaml"
		fi
		if (( metadata_count != 1 )); then
			fail "${file#"$ROOT_DIR/"}:$line_number local Action must contain exactly one action.yml or action.yaml: $value"
			return
		fi
		validate_local_action_runtime "$file" "$line_number" "$metadata" "$value"
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

shell_downloader_token_re="(^|[^[:alnum:]_.-])(${CURL_COMMAND_NAME}|${WGET_COMMAND_NAME})([^[:alnum:]_./-]|$)"

shell_command_has_downloader_token() {
	local command="$1"
	# This is a bounded authored-shell policy, not a shell interpreter. Any
	# textual downloader identity is security-relevant regardless of wrappers.
	[[ "$command" =~ $shell_downloader_token_re ]]
}

shell_command_downloader_token_count() {
	local remainder="$1"
	local match count=0

	while [[ "$remainder" =~ $shell_downloader_token_re ]]; do
		match="${BASH_REMATCH[0]}"
		count=$((count + 1))
		remainder="${remainder#*"$match"}"
	done
	printf '%d' "$count"
}

shell_command_is_loopback_probe() {
	local command="$1"
	local direct_re='^(if[[:space:]]+)?(command[[:space:]]+)?(/usr/bin/)?'"$CURL_COMMAND_NAME"'([[:space:]]+-[A-Za-z]+)*[[:space:]]+"http://127[.]0[.]0[.]1:[^"]*"[[:space:]]*(>/dev/null[[:space:]]+2>&1)?[[:space:]]*(;[[:space:]]*then)?$'
	local substitution_re='^[A-Za-z_][A-Za-z0-9_]*="\$\([[:space:]]*(command[[:space:]]+)?(/usr/bin/)?'"$CURL_COMMAND_NAME"'([[:space:]]+-[A-Za-z]+)*[[:space:]]+"http://127[.]0[.]0[.]1:[^"]*"[[:space:]]*(\|\|[[:space:]]+true)?[[:space:]]*\)"$'
	if (( $(shell_command_downloader_token_count "$command") != 1 )); then
		return 1
	fi
	[[ "$command" =~ $direct_re || "$command" =~ $substitution_re ]]
}

shell_command_is_loopback_support_command() {
	local relative="$1"
	local command="$2"

	if [[ "$relative" == scripts/browser-smoke.sh ]] &&
		[[ "$command" == "require_command $CURL_COMMAND_NAME \"Install $CURL_COMMAND_NAME for smoke server readiness checks.\"" ]]; then
		return 0
	fi
	[[ "$relative" == .github/workflows/ci-browser-smoke.yml ]] &&
		[[ "$command" == "sudo apt-get install -y brotli zstd $CURL_COMMAND_NAME" ]]
}

inspect_shell_logical_command() {
	local relative="$1"
	local line_number="$2"
	local command="$3"
	local working_directory="${4:-}"
	local strict_helpers="${5:-false}"

	command="$(trim_whitespace "$command")"
	inspect_referenced_ci_helpers \
		"$relative" "$line_number" "$command" "$working_directory" \
		"$strict_helpers"
	if [[ -z "$command" ]] || ! shell_command_has_downloader_token "$command"; then
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
	if shell_command_is_loopback_support_command "$relative" "$command"; then
		return
	fi
	fail "$relative:$line_number downloader token is outside the accepted TinyGo fetch and literal-rooted loopback contract"
}

pending_shell_command=""
pending_shell_line=0

reset_shell_logical_command() {
	pending_shell_command=""
	pending_shell_line=0
}

strip_shell_comment() {
	local line="$1"
	local index character previous quote="" escaped=false

	for ((index = 0; index < ${#line}; index++)); do
		character="${line:index:1}"
		if [[ "$escaped" == true ]]; then
			escaped=false
			continue
		fi
		if [[ "$quote" == "'" ]]; then
			if [[ "$character" == "'" ]]; then
				quote=""
			fi
			continue
		fi
		if [[ "$quote" == '"' ]]; then
			if [[ "$character" == \\ ]]; then
				escaped=true
			elif [[ "$character" == '"' ]]; then
				quote=""
			fi
			continue
		fi
		if [[ "$character" == \\ ]]; then
			escaped=true
			continue
		fi
		if [[ "$character" == "'" || "$character" == '"' ]]; then
			quote="$character"
			continue
		fi
		if [[ "$character" != \# ]]; then
			continue
		fi
		if (( index == 0 )); then
			printf '%s' "${line:0:index}"
			return
		fi
		previous="${line:index-1:1}"
		case "$previous" in
			[[:space:]]|';'|'|'|'&')
				printf '%s' "${line:0:index}"
				return
				;;
		esac
	done
	printf '%s' "$line"
}

consume_shell_physical_line() {
	local relative="$1"
	local line_number="$2"
	local line="$3"
	local working_directory="${4:-}"
	local strict_helpers="${5:-false}"
	local command

	line="$(strip_shell_comment "$line")"
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
	inspect_shell_logical_command \
		"$relative" "$pending_shell_line" "$pending_shell_command" \
		"$working_directory" "$strict_helpers"
	reset_shell_logical_command
}

flush_shell_logical_command() {
	local relative="$1"
	local working_directory="${2:-}"
	local strict_helpers="${3:-false}"
	if [[ -n "$pending_shell_command" ]]; then
		inspect_shell_logical_command \
			"$relative" "$pending_shell_line" "$pending_shell_command" \
			"$working_directory" "$strict_helpers"
	fi
	reset_shell_logical_command
}

scan_shell_downloaders() {
	local file="$1"
	local relative="$2"
	local working_directory="${3:-}"
	local strict_helpers="${4:-false}"
	local line line_number=0

	reset_shell_logical_command
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		consume_shell_physical_line \
			"$relative" "$line_number" "$line" "$working_directory" \
			"$strict_helpers"
	done < "$file"
	flush_shell_logical_command \
		"$relative" "$working_directory" "$strict_helpers"
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
	[[ "$first_line" =~ ^\#!.*bash([[:space:]]|$) ||
		"$first_line" =~ ^\#!.*/sh([[:space:]]|$) ]]
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

shell_tokens=()

tokenize_shell_command() {
	local command="$1"
	local token="" quote="" character next escaped=false index

	shell_tokens=()
	for ((index = 0; index < ${#command}; index++)); do
		character="${command:index:1}"
		if [[ "$escaped" == true ]]; then
			token+="$character"
			escaped=false
			continue
		fi
		if [[ "$quote" == "'" ]]; then
			if [[ "$character" == "'" ]]; then
				quote=""
			else
				token+="$character"
			fi
			continue
		fi
		if [[ "$quote" == '"' ]]; then
			if [[ "$character" == \\ ]]; then
				escaped=true
			elif [[ "$character" == '"' ]]; then
				quote=""
			else
				token+="$character"
			fi
			continue
		fi
		case "$character" in
		\\)
			escaped=true
			;;
		"'"|'"')
			quote="$character"
			;;
		[[:space:]])
			if [[ -n "$token" ]]; then
				shell_tokens[${#shell_tokens[@]}]="$token"
				token=""
			fi
			;;
		';'|'|'|'&'|'('|')')
			if [[ -n "$token" ]]; then
				shell_tokens[${#shell_tokens[@]}]="$token"
				token=""
			fi
			next=""
			if (( index + 1 < ${#command} )); then
				next="${command:index+1:1}"
			fi
			if [[ ( "$character" == '|' || "$character" == '&' ) &&
				"$next" == "$character" ]] ||
				[[ "$character" == '|' && "$next" == '&' ]]; then
				shell_tokens[${#shell_tokens[@]}]="$character$next"
				index=$((index + 1))
			else
				shell_tokens[${#shell_tokens[@]}]="$character"
			fi
			;;
		*)
			token+="$character"
			;;
		esac
	done
	if [[ "$escaped" == true || -n "$quote" ]]; then
		return 1
	fi
	if [[ -n "$token" ]]; then
		shell_tokens[${#shell_tokens[@]}]="$token"
	fi
}

shell_token_is_operator() {
	case "$1" in
		';'|'|'|'|&'|'||'|'&'|'&&'|'('|')') return 0 ;;
	esac
	return 1
}

execution_target_is_dynamic() {
	case "$1" in
		*'$'*|*'`'*|*'${{'*) return 0 ;;
	esac
	return 1
}

resolve_execution_working_directory() {
	local source_relative="$1"
	local line_number="$2"
	local working_directory="$3"
	local strict="$4"
	local padded

	resolved_execution_directory=""
	if [[ -z "$working_directory" || "$working_directory" == . ]]; then
		return 0
	fi
	if execution_target_is_dynamic "$working_directory"; then
		if [[ "$strict" == true ]]; then
			fail "$source_relative:$line_number cannot statically resolve a repository helper under dynamic working-directory: $working_directory"
		fi
		return 1
	fi
	working_directory="${working_directory#./}"
	padded="/$working_directory/"
	if [[ "$working_directory" == /* || "$working_directory" == *\\* ||
		"$padded" == *'//'* || "$padded" == *'/./'* ||
		"$padded" == *'/../'* ]]; then
		fail "$source_relative:$line_number execution working-directory is not a canonical repository-relative path: $working_directory"
		return 1
	fi
	if path_has_symlink_component "$ROOT_DIR" "$working_directory"; then
		fail "$source_relative:$line_number execution working-directory traverses a symbolic link: $working_directory"
		return 1
	fi
	if [[ ! -d "$ROOT_DIR/$working_directory" ]]; then
		fail "$source_relative:$line_number execution working-directory does not name a repository directory: $working_directory"
		return 1
	fi
	resolved_execution_directory="$working_directory"
}

scan_execution_target() {
	local source_relative="$1"
	local line_number="$2"
	local working_directory="$3"
	local target="$4"
	local language="$5"
	local strict="$6"
	local mode="${7:-scan}"
	local relative padded file shell_source=false

	if [[ -z "$target" || "$target" == '{0}' ]]; then
		return
	fi
	if execution_target_is_dynamic "$target"; then
		if [[ "$strict" == true &&
			"$mode" != reject-indirect-executable ]]; then
			fail "$source_relative:$line_number cannot statically resolve repository execution target: $target"
		fi
		return
	fi
	if [[ "$target" == /* ]]; then
		if [[ "$mode" == scan || "$strict" == true ]]; then
			fail "$source_relative:$line_number repository execution target must be repository-relative: $target"
		fi
		return
	fi
	if ! resolve_execution_working_directory \
		"$source_relative" "$line_number" "$working_directory" "$strict"; then
		return
	fi
	target="${target#./}"
	if [[ -n "$resolved_execution_directory" ]]; then
		relative="$resolved_execution_directory/$target"
	else
		relative="$target"
	fi
	padded="/$relative/"
	if [[ -z "$relative" || "$relative" == *\\* ||
		"$padded" == *'//'* || "$padded" == *'/./'* ||
		"$padded" == *'/../'* ]]; then
		if [[ "$mode" == scan || "$strict" == true ]]; then
			fail "$source_relative:$line_number repository execution target is not canonical: $target"
		fi
		return
	fi
	if path_has_symlink_component "$ROOT_DIR" "$relative"; then
		fail "$source_relative:$line_number repository execution target traverses a symbolic link: $target"
		return
	fi
	file="$ROOT_DIR/$relative"
	if [[ ! -f "$file" ]]; then
		if [[ "$strict" == true &&
			"$mode" != reject-indirect-executable ]]; then
			fail "$source_relative:$line_number repository execution target does not name an existing regular file: $target"
		fi
		return
	fi
	if [[ "$mode" == reject-indirect-executable && ! -x "$file" ]]; then
		return
	fi
	if [[ "$mode" == reject-indirect ||
		"$mode" == reject-indirect-executable ]]; then
		if [[ "$strict" != true ]] &&
			repository_file_is_ci_executable_source "$relative" "$file"; then
			return
		fi
		fail "$source_relative:$line_number repository execution target appears behind unsupported execution indirection: $target"
		return
	fi

	scan_ci_executable_tinygo_provenance "$relative" "$file" true
	if [[ "$language" == shell ]]; then
		shell_source=true
	elif [[ "$language" == direct ]] &&
		repository_file_is_shell_source "$relative" "$file"; then
		shell_source=true
	fi
	if [[ "$shell_source" == true ]] &&
		! repository_file_is_shell_source "$relative" "$file"; then
		scan_shell_downloaders "$file" "$relative"
	fi
}

inspect_interpreter_target() {
	local source_relative="$1"
	local line_number="$2"
	local working_directory="$3"
	local strict="$4"
	local interpreter="$5"
	local start="$6"
	local end="$7"
	local mode="${8:-scan}"
	local stdin_from_pipe="${9:-false}"
	local index=$start token target="" language=non-shell option_flags

	if [[ "$interpreter" == bash || "$interpreter" == sh ]]; then
		language=shell
		while (( index < end )); do
			token="${shell_tokens[index]}"
			if [[ "$token" == \<* ]]; then
				fail "$source_relative:$line_number $interpreter redirected stdin execution is outside the bounded authored-CI contract"
				return
			fi
			if [[ "$token" == -- ]]; then
				index=$((index + 1))
				if (( index < end )); then
					target="${shell_tokens[index]}"
				fi
				break
			fi
			if [[ "$token" == - ]]; then
				fail "$source_relative:$line_number $interpreter stdin execution is outside the bounded authored-CI contract"
				return
			fi
			case "$token" in
				--command)
					fail "$source_relative:$line_number $interpreter command-string execution is outside the bounded authored-CI contract"
					return
					;;
				-o|-O)
					if (( index + 1 >= end )); then
						if [[ "$strict" == true ]]; then
							fail "$source_relative:$line_number $interpreter option is missing its argument: $token"
						fi
						return
					fi
					index=$((index + 2))
					continue
					;;
				-?*)
					if [[ "$token" != --* ]]; then
						option_flags="${token#-}"
						if [[ "$option_flags" == *c* ]]; then
							fail "$source_relative:$line_number $interpreter command-string execution is outside the bounded authored-CI contract: $token"
							return
						fi
						if [[ "$option_flags" == *s* ]]; then
							fail "$source_relative:$line_number $interpreter stdin execution is outside the bounded authored-CI contract: $token"
							return
						fi
					fi
					index=$((index + 1))
					continue
					;;
				*) target="$token"; break ;;
			esac
		done
	elif [[ "$interpreter" == python || "$interpreter" == python3 ]]; then
		while (( index < end )); do
			token="${shell_tokens[index]}"
			if [[ "$token" == \<* ]]; then
				fail "$source_relative:$line_number $interpreter redirected stdin execution is outside the bounded authored-CI contract"
				return
			fi
			if [[ "$token" == -- ]]; then
				index=$((index + 1))
				if (( index < end )); then
					target="${shell_tokens[index]}"
				fi
				break
			fi
			case "$token" in
				-c|-m)
					fail "$source_relative:$line_number $interpreter opaque code or module execution is outside the bounded authored-CI contract: $token"
					return
					;;
				-)
					fail "$source_relative:$line_number $interpreter stdin execution is outside the bounded authored-CI contract"
					return
					;;
				-W|-X)
					if (( index + 1 >= end )); then
						fail "$source_relative:$line_number $interpreter option is missing its argument: $token"
						return
					fi
					index=$((index + 2))
					continue
					;;
				-W?*|-X?*|-B|-E|-I|-O|-OO|-q|-s|-S|-u|-v|-x)
					index=$((index + 1))
					continue
					;;
				-V|--version|-h|--help)
					return
					;;
				-*)
					fail "$source_relative:$line_number unsupported $interpreter option prevents bounded execution-target resolution: $token"
					return
					;;
				*) target="$token"; break ;;
			esac
		done
	elif [[ "$interpreter" == node ]]; then
		while (( index < end )); do
			token="${shell_tokens[index]}"
			if [[ "$token" == \<* ]]; then
				fail "$source_relative:$line_number Node redirected stdin execution is outside the bounded authored-CI contract"
				return
			fi
			if [[ "$token" == -- ]]; then
				index=$((index + 1))
				if (( index < end )); then
					target="${shell_tokens[index]}"
				fi
				break
			fi
			case "$token" in
				-|--eval|--eval=*|--print|--print=*|--run|--run=*)
					fail "$source_relative:$line_number Node opaque code, script-graph, or stdin execution is outside the bounded authored-CI contract: $token"
					return
					;;
				-e|-p)
					fail "$source_relative:$line_number Node inline code execution is outside the bounded authored-CI contract: $token"
					return
					;;
				-r|--require|--import|--loader|--experimental-loader)
					index=$((index + 1))
					if (( index >= end )); then
						fail "$source_relative:$line_number Node preload option is missing its repository execution target"
						return
					fi
					scan_execution_target "$source_relative" "$line_number" \
						"$working_directory" "${shell_tokens[index]}" non-shell "$strict" "$mode"
					index=$((index + 1))
					continue
					;;
				--require=*|--import=*|--loader=*|--experimental-loader=*)
					scan_execution_target "$source_relative" "$line_number" \
						"$working_directory" "${token#*=}" non-shell "$strict" "$mode"
					index=$((index + 1))
					continue
					;;
				--experimental-websocket)
					index=$((index + 1))
					continue
					;;
				-v|--version|-h|--help)
					return
					;;
				-?*)
					if [[ "$token" != --* ]]; then
						option_flags="${token#-}"
						if [[ "$option_flags" == *e* || "$option_flags" == *p* ]]; then
							fail "$source_relative:$line_number Node inline code execution is outside the bounded authored-CI contract: $token"
							return
						fi
					fi
					fail "$source_relative:$line_number unsupported Node option prevents bounded execution-target resolution: $token"
					return
					;;
				*) target="$token"; break ;;
			esac
		done
	else
		while (( index < end )); do
			token="${shell_tokens[index]}"
			if [[ "$token" == \<* ]]; then
				fail "$source_relative:$line_number $interpreter redirected stdin execution is outside the bounded authored-CI contract"
				return
			fi
			case "$token" in
				-[Ff][Ii][Ll][Ee])
					index=$((index + 1))
					if (( index >= end )); then
						fail "$source_relative:$line_number $interpreter -File is missing its repository execution target"
						return
					fi
					target="${shell_tokens[index]}"
					if [[ "$target" == - ]]; then
						fail "$source_relative:$line_number $interpreter stdin execution is outside the bounded authored-CI contract"
						return
					fi
					break
					;;
				-*)
					fail "$source_relative:$line_number $interpreter command or encoded execution mode is outside the bounded authored-CI contract: $token"
					return
					;;
				*) target="$token"; break ;;
			esac
		done
	fi
	if [[ -n "$target" ]]; then
		scan_execution_target "$source_relative" "$line_number" \
			"$working_directory" "$target" "$language" "$strict" "$mode"
	elif [[ "$stdin_from_pipe" == true ]]; then
		fail "$source_relative:$line_number $interpreter cannot consume executable stdin from a pipeline in the bounded authored-CI contract"
	fi
}

supported_interpreter_name() {
	local candidate="$1"

	[[ "$candidate" == bash ]] ||
		[[ "$candidate" == sh ]] ||
		[[ "$candidate" == python ]] ||
		[[ "$candidate" == python3 ]] ||
		[[ "$candidate" == node ]] ||
		[[ "$candidate" == pwsh ]] ||
		[[ "$candidate" == powershell ]]
}

inspect_unsupported_execution_indirection() {
	local source_relative="$1"
	local line_number="$2"
	local working_directory="$3"
	local strict="$4"
	local start="$5"
	local end="$6"
	local stdin_from_pipe="${7:-false}"
	local index=$((start + 1)) token command_name

	# The command head is unsupported, so inspect later tokens only for the
	# repository execution shapes that the canonical grammar already owns.
	while (( index < end )); do
		token="${shell_tokens[index]}"
		command_name="${token##*/}"
		if supported_interpreter_name "$command_name"; then
			inspect_interpreter_target "$source_relative" "$line_number" \
				"$working_directory" "$strict" "$command_name" \
				"$((index + 1))" "$end" reject-indirect "$stdin_from_pipe"
			return
		fi
		case "$command_name" in
			source|.)
				index=$((index + 1))
				if (( index < end )); then
					scan_execution_target "$source_relative" "$line_number" \
						"$working_directory" "${shell_tokens[index]}" shell "$strict" \
						reject-indirect
				fi
				return
				;;
			$EVAL_COMMAND_NAME)
				fail "$source_relative:$line_number shell eval is outside the bounded authored-CI execution contract"
				return
				;;
		esac
		if [[ "$token" != *://* && "$token" != *[[:space:]]* ]] &&
			[[ "$token" == ./* ||
			( "$token" != /* && "$token" == */* ) ]]; then
			scan_execution_target "$source_relative" "$line_number" \
				"$working_directory" "$token" direct "$strict" \
				reject-indirect-executable
		fi
		index=$((index + 1))
	done
}

inspect_shell_command_segment() {
	local source_relative="$1"
	local line_number="$2"
	local working_directory="$3"
	local strict="$4"
	local start="$5"
	local end="$6"
	local stdin_from_pipe="${7:-false}"
	local index=$start token command_name

	while (( index < end )) &&
		[[ "${shell_tokens[index]}" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; do
		index=$((index + 1))
	done
	while (( index < end )); do
		token="${shell_tokens[index]}"
		command_name="${token##*/}"
		case "$command_name" in
			command)
				index=$((index + 1))
				while (( index < end )); do
					token="${shell_tokens[index]}"
					case "$token" in
						--) index=$((index + 1)); break ;;
						-p) index=$((index + 1)) ;;
						-v|-V) return ;;
						-*)
							if [[ "$strict" == true ]]; then
								fail "$source_relative:$line_number unsupported command wrapper option prevents bounded execution-target resolution: $token"
							fi
							return
							;;
						*) break ;;
					esac
				done
				;;
			exec)
				index=$((index + 1))
				while (( index < end )); do
					token="${shell_tokens[index]}"
					case "$token" in
						--) index=$((index + 1)); break ;;
						-c|-l) index=$((index + 1)) ;;
						-a)
							if (( index + 1 >= end )); then
								if [[ "$strict" == true ]]; then
									fail "$source_relative:$line_number exec -a is missing its argv0 argument"
								fi
								return
							fi
							index=$((index + 2))
							;;
						-*)
							if [[ "$strict" == true ]]; then
								fail "$source_relative:$line_number unsupported exec wrapper option prevents bounded execution-target resolution: $token"
							fi
							return
							;;
						*) break ;;
					esac
				done
				;;
			env)
				index=$((index + 1))
				while (( index < end )); do
					token="${shell_tokens[index]}"
					case "$token" in
						--) index=$((index + 1)); break ;;
						-i|--ignore-environment) index=$((index + 1)) ;;
						-u|--unset)
							if (( index + 1 >= end )); then
								if [[ "$strict" == true ]]; then
									fail "$source_relative:$line_number env unset option is missing its variable name"
								fi
								return
							fi
							index=$((index + 2))
							;;
						--unset=*) index=$((index + 1)) ;;
						-C|--chdir|-S|--split-string|--chdir=*|--split-string=*)
							if [[ "$strict" == true ]]; then
								fail "$source_relative:$line_number env option changes bounded execution-target semantics: $token"
							fi
							return
							;;
						-*)
							if [[ "$strict" == true ]]; then
								fail "$source_relative:$line_number unsupported env wrapper option prevents bounded execution-target resolution: $token"
							fi
							return
							;;
						*)
							if [[ "$token" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
								index=$((index + 1))
							else
								break
							fi
							;;
					esac
				done
				;;
			*) break ;;
		esac
	done
	if (( index >= end )); then
		return
	fi
	token="${shell_tokens[index]}"
	command_name="${token##*/}"
	if supported_interpreter_name "$command_name"; then
		inspect_interpreter_target "$source_relative" "$line_number" \
			"$working_directory" "$strict" "$command_name" \
			"$((index + 1))" "$end" scan "$stdin_from_pipe"
		return
	fi
	case "$command_name" in
		source|.)
			index=$((index + 1))
			if (( index < end )); then
				scan_execution_target "$source_relative" "$line_number" \
					"$working_directory" "${shell_tokens[index]}" shell "$strict"
			fi
			;;
		$EVAL_COMMAND_NAME)
			fail "$source_relative:$line_number shell eval is outside the bounded authored-CI execution contract"
			;;
		go)
			index=$((index + 1))
			if (( index < end )) && [[ "${shell_tokens[index]}" == run ]]; then
				index=$((index + 1))
				while (( index < end )); do
					token="${shell_tokens[index]}"
					if [[ "$token" == -* ]]; then
						index=$((index + 1))
						continue
					fi
					case "$token" in
						*.go|./*.go|*/*.go)
							scan_execution_target "$source_relative" "$line_number" \
								"$working_directory" "$token" non-shell "$strict"
							;;
					esac
					index=$((index + 1))
				done
			fi
			;;
		*)
			if [[ "$token" == ./* ||
				( "$token" != /* && "$token" == */* ) ]]; then
				scan_execution_target "$source_relative" "$line_number" \
					"$working_directory" "$token" direct "$strict"
			fi
			inspect_unsupported_execution_indirection "$source_relative" \
				"$line_number" "$working_directory" "$strict" "$index" "$end" \
				"$stdin_from_pipe"
			;;
	esac
}

inspect_static_command_substitutions() {
	local source_relative="$1"
	local line_number="$2"
	local command="$3"
	local working_directory="$4"
	local strict="$5"
	local index=0 length=${#command} character next quote="" escaped=false
	local inner_start inner_end depth inner_quote inner_escaped

	# Substitutions are nested executable command segments hidden from the flat
	# token stream. Inspect their static bodies through the same bounded grammar.
	while (( index < length )); do
		character="${command:index:1}"
		if [[ "$escaped" == true ]]; then
			escaped=false
			index=$((index + 1))
			continue
		fi
		if [[ "$quote" == "'" ]]; then
			if [[ "$character" == "'" ]]; then
				quote=""
			fi
			index=$((index + 1))
			continue
		fi
		if [[ "$character" == \\ ]]; then
			escaped=true
			index=$((index + 1))
			continue
		fi
		if [[ "$character" == "'" && "$quote" != '"' ]]; then
			quote="'"
			index=$((index + 1))
			continue
		fi
		if [[ "$character" == '"' ]]; then
			if [[ "$quote" == '"' ]]; then
				quote=""
			else
				quote='"'
			fi
			index=$((index + 1))
			continue
		fi
		next=""
		if (( index + 1 < length )); then
			next="${command:index+1:1}"
		fi
		if [[ "$character" == '$' && "$next" == '(' ]]; then
			inner_start=$((index + 2))
			inner_end=$inner_start
			depth=1
			inner_quote=""
			inner_escaped=false
			while (( inner_end < length )); do
				character="${command:inner_end:1}"
				if [[ "$inner_escaped" == true ]]; then
					inner_escaped=false
					inner_end=$((inner_end + 1))
					continue
				fi
				if [[ "$inner_quote" == "'" ]]; then
					if [[ "$character" == "'" ]]; then
						inner_quote=""
					fi
					inner_end=$((inner_end + 1))
					continue
				fi
				if [[ "$character" == \\ ]]; then
					inner_escaped=true
					inner_end=$((inner_end + 1))
					continue
				fi
				if [[ "$character" == "'" && "$inner_quote" != '"' ]]; then
					inner_quote="'"
					inner_end=$((inner_end + 1))
					continue
				fi
				if [[ "$character" == '"' ]]; then
					if [[ "$inner_quote" == '"' ]]; then
						inner_quote=""
					else
						inner_quote='"'
					fi
					inner_end=$((inner_end + 1))
					continue
				fi
				if [[ "$inner_quote" != '"' && "$character" == '$' ]] &&
					(( inner_end + 1 < length )) &&
					[[ "${command:inner_end+1:1}" == '(' ]]; then
					depth=$((depth + 1))
					inner_end=$((inner_end + 2))
					continue
				fi
				if [[ -z "$inner_quote" && "$character" == ')' ]]; then
					depth=$((depth - 1))
					if (( depth == 0 )); then
						break
					fi
				fi
				inner_end=$((inner_end + 1))
			done
			if (( depth != 0 )); then
				if [[ "$strict" == true ]]; then
					fail "$source_relative:$line_number cannot safely inspect command substitution for repository helper resolution"
				fi
				return
			fi
			inspect_referenced_ci_helpers "$source_relative" "$line_number" \
				"${command:inner_start:inner_end-inner_start}" \
				"$working_directory" "$strict"
			index=$((inner_end + 1))
			continue
		fi
		if [[ "$character" == '`' ]]; then
			inner_start=$((index + 1))
			inner_end=$inner_start
			inner_escaped=false
			while (( inner_end < length )); do
				character="${command:inner_end:1}"
				if [[ "$inner_escaped" == true ]]; then
					inner_escaped=false
				elif [[ "$character" == \\ ]]; then
					inner_escaped=true
				elif [[ "$character" == '`' ]]; then
					break
				fi
				inner_end=$((inner_end + 1))
			done
			if (( inner_end < length )); then
				inspect_referenced_ci_helpers "$source_relative" "$line_number" \
					"${command:inner_start:inner_end-inner_start}" \
					"$working_directory" "$strict"
				index=$((inner_end + 1))
				continue
			fi
		fi
		index=$((index + 1))
	done
}

inspect_referenced_ci_helpers() {
	local source_relative="$1"
	local line_number="$2"
	local command="$3"
	local working_directory="${4:-}"
	local strict="${5:-false}"
	local start=0 index stdin_from_pipe=false

	inspect_static_command_substitutions "$source_relative" "$line_number" \
		"$command" "$working_directory" "$strict"
	if ! tokenize_shell_command "$command"; then
		if [[ "$strict" == true ]]; then
			fail "$source_relative:$line_number cannot safely tokenize executable command for repository helper resolution"
		fi
		return
	fi
	for ((index = 0; index <= ${#shell_tokens[@]}; index++)); do
		if (( index == ${#shell_tokens[@]} )) ||
			shell_token_is_operator "${shell_tokens[index]}"; then
			if (( start < index )); then
				inspect_shell_command_segment "$source_relative" "$line_number" \
					"$working_directory" "$strict" "$start" "$index" \
					"$stdin_from_pipe"
			fi
			stdin_from_pipe=false
			if (( index < ${#shell_tokens[@]} )) &&
				[[ "${shell_tokens[index]}" == '|' ||
				"${shell_tokens[index]}" == '|&' ]]; then
				stdin_from_pipe=true
			fi
			start=$((index + 1))
		fi
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

ci_files=()
ci_file_types=()
ci_file_default_shell_count=()
ci_file_default_shell_value=()
ci_file_default_shell_line=()
ci_file_default_directory_count=()
ci_file_default_directory_value=()
ci_file_default_directory_line=()
ci_file_action_using_count=()
ci_file_action_using_value=()
ci_file_action_using_line=()

ci_job_file=()
ci_job_id=()
ci_job_line=()
ci_job_if_line=()
ci_job_continue_line=()
ci_job_needs_line=()
ci_job_runs_on_count=()
ci_job_runs_on_value=()
ci_job_runs_on_line=()
ci_job_container_line=()
ci_job_uses_count=()
ci_job_uses_value=()
ci_job_uses_line=()
ci_job_uses_comment=()
ci_job_uses_comment_is_yaml=()
ci_job_default_shell_count=()
ci_job_default_shell_value=()
ci_job_default_shell_line=()
ci_job_default_directory_count=()
ci_job_default_directory_value=()
ci_job_default_directory_line=()

ci_step_file=()
ci_step_kind=()
ci_step_job=()
ci_step_line=()
ci_step_property_count=()
ci_step_name_count=()
ci_step_name_value=()
ci_step_install_name=()
ci_step_verify_name=()
ci_step_shell_count=()
ci_step_shell_value=()
ci_step_shell_line=()
ci_step_run_count=()
ci_step_run_value=()
ci_step_run_line=()
ci_step_directory_count=()
ci_step_directory_value=()
ci_step_directory_line=()
ci_step_uses_count=()
ci_step_uses_value=()
ci_step_uses_line=()
ci_step_uses_comment=()
ci_step_uses_comment_is_yaml=()
ci_run_step=()
ci_run_line=()
ci_run_text=()

register_ci_file() {
	local relative="$1"
	ci_current_file=${#ci_files[@]}
	ci_files[ci_current_file]="$relative"
	case "$relative" in
		.github/workflows/*) ci_file_types[ci_current_file]=workflow ;;
		*) ci_file_types[ci_current_file]=action ;;
	esac
	ci_file_default_shell_count[ci_current_file]=0
	ci_file_default_shell_value[ci_current_file]=""
	ci_file_default_shell_line[ci_current_file]=0
	ci_file_default_directory_count[ci_current_file]=0
	ci_file_default_directory_value[ci_current_file]=""
	ci_file_default_directory_line[ci_current_file]=0
	ci_file_action_using_count[ci_current_file]=0
	ci_file_action_using_value[ci_current_file]=""
	ci_file_action_using_line[ci_current_file]=0
}

register_ci_job() {
	local file_index="$1"
	local id="$2"
	local line_number="$3"
	ci_current_job=${#ci_job_id[@]}
	ci_job_file[ci_current_job]="$file_index"
	ci_job_id[ci_current_job]="$id"
	ci_job_line[ci_current_job]="$line_number"
	ci_job_if_line[ci_current_job]=0
	ci_job_continue_line[ci_current_job]=0
	ci_job_needs_line[ci_current_job]=0
	ci_job_runs_on_count[ci_current_job]=0
	ci_job_runs_on_value[ci_current_job]=""
	ci_job_runs_on_line[ci_current_job]=0
	ci_job_container_line[ci_current_job]=0
	ci_job_uses_count[ci_current_job]=0
	ci_job_uses_value[ci_current_job]=""
	ci_job_uses_line[ci_current_job]=0
	ci_job_uses_comment[ci_current_job]=""
	ci_job_uses_comment_is_yaml[ci_current_job]=false
	ci_job_default_shell_count[ci_current_job]=0
	ci_job_default_shell_value[ci_current_job]=""
	ci_job_default_shell_line[ci_current_job]=0
	ci_job_default_directory_count[ci_current_job]=0
	ci_job_default_directory_value[ci_current_job]=""
	ci_job_default_directory_line[ci_current_job]=0
}

find_ci_job() {
	local file_index="$1"
	local id="$2"
	local index
	ci_found_job=-1
	for ((index = 0; index < ${#ci_job_id[@]}; index++)); do
		if [[ "${ci_job_file[index]}" == "$file_index" &&
			"${ci_job_id[index]}" == "$id" ]]; then
			ci_found_job=$index
			return
		fi
	done
}

register_ci_step() {
	local file_index="$1"
	local kind="$2"
	local job_index="$3"
	local line_number="$4"
	ci_current_step=${#ci_step_file[@]}
	ci_step_file[ci_current_step]="$file_index"
	ci_step_kind[ci_current_step]="$kind"
	ci_step_job[ci_current_step]="$job_index"
	ci_step_line[ci_current_step]="$line_number"
	ci_step_property_count[ci_current_step]=0
	ci_step_name_count[ci_current_step]=0
	ci_step_name_value[ci_current_step]=""
	ci_step_install_name[ci_current_step]=false
	ci_step_verify_name[ci_current_step]=false
	ci_step_shell_count[ci_current_step]=0
	ci_step_shell_value[ci_current_step]=""
	ci_step_shell_line[ci_current_step]=0
	ci_step_run_count[ci_current_step]=0
	ci_step_run_value[ci_current_step]=""
	ci_step_run_line[ci_current_step]=0
	ci_step_directory_count[ci_current_step]=0
	ci_step_directory_value[ci_current_step]=""
	ci_step_directory_line[ci_current_step]=0
	ci_step_uses_count[ci_current_step]=0
	ci_step_uses_value[ci_current_step]=""
	ci_step_uses_line[ci_current_step]=0
	ci_step_uses_comment[ci_current_step]=""
	ci_step_uses_comment_is_yaml[ci_current_step]=false
}

record_ci_job_property() {
	local job_index="$1"
	local key="$2"
	local value="$3"
	local line_number="$4"
	local comment="$5"
	local comment_is_yaml="$6"

	case "$key" in
		if) ci_job_if_line[job_index]="$line_number" ;;
		continue-on-error) ci_job_continue_line[job_index]="$line_number" ;;
		needs) ci_job_needs_line[job_index]="$line_number" ;;
		runs-on)
			ci_job_runs_on_count[job_index]=$((ci_job_runs_on_count[job_index] + 1))
			if (( ci_job_runs_on_count[job_index] == 1 )); then
				ci_job_runs_on_value[job_index]="$value"
				ci_job_runs_on_line[job_index]="$line_number"
			fi
			;;
		container) ci_job_container_line[job_index]="$line_number" ;;
		uses)
			ci_job_uses_count[job_index]=$((ci_job_uses_count[job_index] + 1))
			if (( ci_job_uses_count[job_index] == 1 )); then
				ci_job_uses_value[job_index]="$value"
				ci_job_uses_line[job_index]="$line_number"
				ci_job_uses_comment[job_index]="$comment"
				ci_job_uses_comment_is_yaml[job_index]="$comment_is_yaml"
			fi
			;;
	esac
}

record_ci_step_property() {
	local step_index="$1"
	local key="$2"
	local value="$3"
	local line_number="$4"
	local comment="$5"
	local comment_is_yaml="$6"

	ci_step_property_count[step_index]=$((ci_step_property_count[step_index] + 1))
	case "$key" in
		name)
			ci_step_name_count[step_index]=$((ci_step_name_count[step_index] + 1))
			ci_step_name_value[step_index]="$value"
			if [[ "$value" == 'Install TinyGo' ]]; then
				ci_step_install_name[step_index]=true
			elif [[ "$value" == 'Verify TinyGo' ]]; then
				ci_step_verify_name[step_index]=true
			fi
			;;
		shell)
			ci_step_shell_count[step_index]=$((ci_step_shell_count[step_index] + 1))
			if (( ci_step_shell_count[step_index] == 1 )); then
				ci_step_shell_value[step_index]="$value"
				ci_step_shell_line[step_index]="$line_number"
			fi
			;;
		run)
			ci_step_run_count[step_index]=$((ci_step_run_count[step_index] + 1))
			if (( ci_step_run_count[step_index] == 1 )); then
				ci_step_run_value[step_index]="$value"
				ci_step_run_line[step_index]="$line_number"
			fi
			;;
		working-directory)
			ci_step_directory_count[step_index]=$((ci_step_directory_count[step_index] + 1))
			if (( ci_step_directory_count[step_index] == 1 )); then
				ci_step_directory_value[step_index]="$value"
				ci_step_directory_line[step_index]="$line_number"
			fi
			;;
		uses)
			ci_step_uses_count[step_index]=$((ci_step_uses_count[step_index] + 1))
			if (( ci_step_uses_count[step_index] == 1 )); then
				ci_step_uses_value[step_index]="$value"
				ci_step_uses_line[step_index]="$line_number"
				ci_step_uses_comment[step_index]="$comment"
				ci_step_uses_comment_is_yaml[step_index]="$comment_is_yaml"
			fi
			;;
	esac
}

parse_authored_ci_file() {
	local file="$1"
	local relative="${file#"$ROOT_DIR/"}"
	local line line_number=0 content syntax indent key value sequence_prefix
	local comment comment_is_yaml key_indent stack_count stack_index parent_kind
	local active_step=-1 active_step_indent=-1 job_index=-1
	local scalar_indent=-1 scalar_step=-1 scalar_key=""
	local mapping_re='^(-[[:space:]]+)?([A-Za-z_][A-Za-z0-9_-]*):([[:space:]]+(.*))?$'
	local inline_comment_re='^(.*[^[:space:]])[[:space:]]+#(.*)$'
	local full_comment_re='^[[:space:]]*#(.*)$'
	local path_keys=()
	local path_indents=()

	register_ci_file "$relative"
	local file_index=$ci_current_file
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_number=$((line_number + 1))
		indent="${line%%[![:space:]]*}"
		syntax="$(trim_whitespace "$line")"
		if (( scalar_indent >= 0 )) &&
			[[ -z "$syntax" || ${#indent} -gt scalar_indent ]]; then
			if (( scalar_step >= 0 )) && [[ "$scalar_key" == run ]]; then
				ci_run_step[${#ci_run_step[@]}]="$scalar_step"
				ci_run_line[${#ci_run_line[@]}]="$line_number"
				ci_run_text[${#ci_run_text[@]}]="$syntax"
			fi
			continue
		fi
		scalar_indent=-1
		scalar_step=-1
		scalar_key=""

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
		if [[ "${ci_file_types[file_index]}" == workflow ]] &&
			(( stack_count == 3 )) &&
			[[ "${path_keys[0]}" == jobs && "${path_keys[2]}" == steps ]]; then
			parent_kind=workflow-step
		elif [[ "${ci_file_types[file_index]}" == action ]] &&
			(( stack_count == 2 )) &&
			[[ "${path_keys[0]}" == runs && "${path_keys[1]}" == steps ]]; then
			parent_kind=composite-step
		fi
		if (( active_step >= 0 && key_indent < active_step_indent )); then
			active_step=-1
			active_step_indent=-1
		fi

		if [[ ! "$syntax" =~ $mapping_re ]]; then
			if [[ "$syntax" == - && -n "$parent_kind" ]]; then
				fail "$relative:$line_number unsupported standalone step sequence indicator; use canonical '- key:' step mappings"
				active_step=-1
				active_step_indent=-1
				continue
			fi
			case "${syntax#- }" in
				*:*|\{*|\[*|\?*|\&*|\**|\!*)
					fail "$relative:$line_number unsupported workflow mapping syntax; use canonical block keys"
					;;
			esac
			continue
		fi

		if [[ -n "$sequence_prefix" && -n "$parent_kind" ]]; then
			job_index=-1
			if [[ "$parent_kind" == workflow-step ]]; then
				find_ci_job "$file_index" "${path_keys[1]}"
				job_index=$ci_found_job
			fi
			register_ci_step "$file_index" "$parent_kind" "$job_index" "$line_number"
			active_step=$ci_current_step
			active_step_indent=$key_indent
		elif [[ -n "$sequence_prefix" && active_step -ge 0 ]] &&
			(( key_indent <= active_step_indent )); then
			active_step=-1
			active_step_indent=-1
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

		if [[ "${ci_file_types[file_index]}" == workflow &&
			-z "$sequence_prefix" ]] &&
			(( stack_count == 1 )) && [[ "${path_keys[0]}" == jobs ]] &&
			[[ -z "$value" ]]; then
			register_ci_job "$file_index" "$key" "$line_number"
		fi

		if [[ "${ci_file_types[file_index]}" == workflow ]] &&
			(( stack_count >= 2 )) && [[ "${path_keys[0]}" == jobs ]]; then
			find_ci_job "$file_index" "${path_keys[1]}"
			job_index=$ci_found_job
			if (( job_index >= 0 && stack_count == 2 )); then
				record_ci_job_property "$job_index" "$key" "$value" \
					"$line_number" "$comment" "$comment_is_yaml"
			fi
			if (( job_index >= 0 && stack_count == 4 )) &&
				[[ "${path_keys[2]}" == defaults && "${path_keys[3]}" == run ]]; then
				case "$key" in
					shell)
						ci_job_default_shell_count[job_index]=$((ci_job_default_shell_count[job_index] + 1))
						ci_job_default_shell_value[job_index]="$value"
						ci_job_default_shell_line[job_index]="$line_number"
						;;
					working-directory)
						ci_job_default_directory_count[job_index]=$((ci_job_default_directory_count[job_index] + 1))
						ci_job_default_directory_value[job_index]="$value"
						ci_job_default_directory_line[job_index]="$line_number"
						;;
				esac
			fi
		fi
		if [[ "${ci_file_types[file_index]}" == workflow ]] &&
			(( stack_count == 2 )) &&
			[[ "${path_keys[0]}" == defaults && "${path_keys[1]}" == run ]]; then
			case "$key" in
				shell)
					ci_file_default_shell_count[file_index]=$((ci_file_default_shell_count[file_index] + 1))
					ci_file_default_shell_value[file_index]="$value"
					ci_file_default_shell_line[file_index]="$line_number"
					;;
				working-directory)
					ci_file_default_directory_count[file_index]=$((ci_file_default_directory_count[file_index] + 1))
					ci_file_default_directory_value[file_index]="$value"
					ci_file_default_directory_line[file_index]="$line_number"
					;;
			esac
		fi
		if [[ "${ci_file_types[file_index]}" == action ]] &&
			(( stack_count == 1 )) && [[ "${path_keys[0]}" == runs ]] &&
			[[ "$key" == using ]]; then
			ci_file_action_using_count[file_index]=$((ci_file_action_using_count[file_index] + 1))
			ci_file_action_using_value[file_index]="$value"
			ci_file_action_using_line[file_index]="$line_number"
		fi
		if (( active_step >= 0 && key_indent == active_step_indent )); then
			record_ci_step_property "$active_step" "$key" "$value" \
				"$line_number" "$comment" "$comment_is_yaml"
		fi

		if [[ "$value" =~ $yaml_block_scalar_re ]]; then
			scalar_indent=$key_indent
			if (( active_step >= 0 && key_indent == active_step_indent )); then
				scalar_step=$active_step
				scalar_key="$key"
			fi
			continue
		fi
		if [[ -z "$value" ]]; then
			path_indents[stack_count]=$key_indent
			path_keys[stack_count]="$key"
		fi
	done < "$file"
}

unquote_yaml_scalar() {
	local value="$1"
	if (( ${#value} >= 2 )); then
		if [[ "${value:0:1}" == '"' && "${value: -1}" == '"' ]]; then
			value="${value:1:${#value}-2}"
		elif [[ "${value:0:1}" == "'" && "${value: -1}" == "'" ]]; then
			value="${value:1:${#value}-2}"
		fi
	fi
	printf '%s' "$value"
}

validate_local_action_runtime() {
	local source_file="$1"
	local line_number="$2"
	local metadata="$3"
	local value="$4"
	local relative="${metadata#"$ROOT_DIR/"}"
	local index found=-1 count runtime runtime_line

	for ((index = 0; index < ${#ci_files[@]}; index++)); do
		if [[ "${ci_files[index]}" == "$relative" ]]; then
			found=$index
			break
		fi
	done
	if (( found < 0 )); then
		fail "${source_file#"$ROOT_DIR/"}:$line_number could not structurally inspect local Action metadata: $value"
		return
	fi
	count=${ci_file_action_using_count[found]}
	runtime=${ci_file_action_using_value[found]}
	runtime_line=${ci_file_action_using_line[found]}
	if (( count != 1 )); then
		fail "$relative:$runtime_line local Action must contain exactly one canonical runs.using: composite"
	elif [[ "$runtime" != composite ]]; then
		fail "$relative:$runtime_line unsupported local Action runtime '$runtime'; A3 permits composite Actions only"
	fi
}

validate_action_uses_ref() {
	local file="$1"
	local line_number="$2"
	local value="$3"
	local comment="$4"
	local comment_is_yaml="$5"

	value="$(unquote_yaml_scalar "$value")"
	if [[ -z "$value" ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number has an empty uses reference"
		return
	fi
	if [[ "$value" == docker://* ]]; then
		fail "${file#"$ROOT_DIR/"}:$line_number direct Docker Actions are unsupported in the A3 authored-CI contract: $value"
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

validate_authored_action_refs() {
	local index file
	for ((index = 0; index < ${#ci_job_id[@]}; index++)); do
		if (( ci_job_uses_count[index] > 1 )); then
			file="$ROOT_DIR/${ci_files[${ci_job_file[index]}]}"
			fail "${ci_files[${ci_job_file[index]}]}:${ci_job_line[index]} reusable-workflow job contains duplicate uses properties"
		elif (( ci_job_uses_count[index] == 1 )); then
			file="$ROOT_DIR/${ci_files[${ci_job_file[index]}]}"
			validate_action_uses_ref "$file" "${ci_job_uses_line[index]}" \
				"${ci_job_uses_value[index]}" "${ci_job_uses_comment[index]}" \
				"${ci_job_uses_comment_is_yaml[index]}"
		fi
	done
	for ((index = 0; index < ${#ci_step_file[@]}; index++)); do
		if (( ci_step_uses_count[index] > 1 )); then
			fail "${ci_files[${ci_step_file[index]}]}:${ci_step_line[index]} Action step contains duplicate uses properties"
		elif (( ci_step_uses_count[index] == 1 )); then
			file="$ROOT_DIR/${ci_files[${ci_step_file[index]}]}"
			validate_action_uses_ref "$file" "${ci_step_uses_line[index]}" \
				"${ci_step_uses_value[index]}" "${ci_step_uses_comment[index]}" \
				"${ci_step_uses_comment_is_yaml[index]}"
		fi
	done
}

effective_step_directory() {
	local step_index="$1"
	local file_index=${ci_step_file[step_index]}
	local job_index=${ci_step_job[step_index]}

	effective_directory=""
	if (( ci_file_default_directory_count[file_index] > 1 )); then
		fail "${ci_files[file_index]}:${ci_file_default_directory_line[file_index]} workflow defaults.run contains duplicate working-directory properties"
	fi
	if (( ci_file_default_directory_count[file_index] == 1 )); then
		effective_directory=${ci_file_default_directory_value[file_index]}
	fi
	if (( job_index >= 0 )); then
		if (( ci_job_default_directory_count[job_index] > 1 )); then
			fail "${ci_files[file_index]}:${ci_job_default_directory_line[job_index]} job defaults.run contains duplicate working-directory properties"
		fi
		if (( ci_job_default_directory_count[job_index] == 1 )); then
			effective_directory=${ci_job_default_directory_value[job_index]}
		fi
	fi
	if (( ci_step_directory_count[step_index] > 1 )); then
		fail "${ci_files[file_index]}:${ci_step_directory_line[step_index]} step contains duplicate working-directory properties"
	fi
	if (( ci_step_directory_count[step_index] == 1 )); then
		effective_directory=${ci_step_directory_value[step_index]}
	fi
	effective_directory="$(unquote_yaml_scalar "$effective_directory")"
}

scan_ci_step_run() {
	local step_index="$1"
	local relative="${ci_files[${ci_step_file[step_index]}]}"
	local directory="$2"
	local value index

	if (( ci_step_run_count[step_index] != 1 )); then
		return
	fi
	value=${ci_step_run_value[step_index]}
	reset_shell_logical_command
	if [[ "$value" =~ $yaml_block_scalar_re ]]; then
		for ((index = 0; index < ${#ci_run_step[@]}; index++)); do
			if [[ "${ci_run_step[index]}" != "$step_index" ]]; then
				continue
			fi
			consume_shell_physical_line "$relative" "${ci_run_line[index]}" \
				"${ci_run_text[index]}" "$directory" true
		done
		flush_shell_logical_command "$relative" "$directory" true
	else
		value="$(unquote_yaml_scalar "$value")"
		consume_shell_physical_line "$relative" "${ci_step_run_line[step_index]}" \
			"$value" "$directory" true
		flush_shell_logical_command "$relative" "$directory" true
	fi
}

apply_authored_execution_policy() {
	local index file_index job_index relative directory shell

	for ((file_index = 0; file_index < ${#ci_files[@]}; file_index++)); do
		if (( ci_file_default_shell_count[file_index] > 1 )); then
			fail "${ci_files[file_index]}:${ci_file_default_shell_line[file_index]} workflow defaults.run contains duplicate shell properties"
		elif (( ci_file_default_shell_count[file_index] == 1 )); then
			shell=${ci_file_default_shell_value[file_index]}
			directory=""
			if (( ci_file_default_directory_count[file_index] == 1 )); then
				directory=${ci_file_default_directory_value[file_index]}
			fi
			if [[ "$shell" =~ $yaml_block_scalar_re ]]; then
				fail "${ci_files[file_index]}:${ci_file_default_shell_line[file_index]} executable shell must use a canonical inline scalar; block/folded shell scalars are outside the bounded authored-CI execution contract"
			elif [[ "$shell" == *'${{'* ]]; then
				fail "${ci_files[file_index]}:${ci_file_default_shell_line[file_index]} dynamic workflow defaults.run.shell is outside the bounded execution contract"
			else
				inspect_shell_logical_command "${ci_files[file_index]}" \
					"${ci_file_default_shell_line[file_index]}" "$shell" "$directory" true
			fi
		fi
	done
	for ((job_index = 0; job_index < ${#ci_job_id[@]}; job_index++)); do
		file_index=${ci_job_file[job_index]}
		if (( ci_job_default_shell_count[job_index] > 1 )); then
			fail "${ci_files[file_index]}:${ci_job_default_shell_line[job_index]} job defaults.run contains duplicate shell properties"
		elif (( ci_job_default_shell_count[job_index] == 1 )); then
			shell=${ci_job_default_shell_value[job_index]}
			directory=""
			if (( ci_file_default_directory_count[file_index] == 1 )); then
				directory=${ci_file_default_directory_value[file_index]}
			fi
			if (( ci_job_default_directory_count[job_index] == 1 )); then
				directory=${ci_job_default_directory_value[job_index]}
			fi
			if [[ "$shell" =~ $yaml_block_scalar_re ]]; then
				fail "${ci_files[file_index]}:${ci_job_default_shell_line[job_index]} executable shell must use a canonical inline scalar; block/folded shell scalars are outside the bounded authored-CI execution contract"
			elif [[ "$shell" == *'${{'* ]]; then
				fail "${ci_files[file_index]}:${ci_job_default_shell_line[job_index]} dynamic job defaults.run.shell is outside the bounded execution contract"
			else
				inspect_shell_logical_command "${ci_files[file_index]}" \
					"${ci_job_default_shell_line[job_index]}" "$shell" "$directory" true
			fi
		fi
	done
	for ((index = 0; index < ${#ci_step_file[@]}; index++)); do
		file_index=${ci_step_file[index]}
		relative=${ci_files[file_index]}
		effective_step_directory "$index"
		directory=$effective_directory
		if (( ci_step_shell_count[index] > 1 )); then
			fail "$relative:${ci_step_shell_line[index]} step contains duplicate shell properties"
		elif (( ci_step_shell_count[index] == 1 )); then
			shell=${ci_step_shell_value[index]}
			if [[ "$shell" =~ $yaml_block_scalar_re ]]; then
				fail "$relative:${ci_step_shell_line[index]} executable shell must use a canonical inline scalar; block/folded shell scalars are outside the bounded authored-CI execution contract"
			elif [[ "$shell" == *'${{'* ]]; then
				fail "$relative:${ci_step_shell_line[index]} dynamic step shell is outside the bounded execution contract"
			else
				inspect_shell_logical_command "$relative" \
					"${ci_step_shell_line[index]}" "$shell" "$directory" true
			fi
		fi
		if (( ci_step_run_count[index] > 1 )); then
			fail "$relative:${ci_step_run_line[index]} step contains duplicate run properties"
		else
			scan_ci_step_run "$index" "$directory"
		fi
	done
}

tinygo_step_body_is_exact() {
	local step_index="$1"
	local index syntax body=()
	for ((index = 0; index < ${#ci_run_step[@]}; index++)); do
		if [[ "${ci_run_step[index]}" != "$step_index" ]]; then
			continue
		fi
		syntax="$(trim_whitespace "${ci_run_text[index]}")"
		if [[ -n "$syntax" && "$syntax" != \#* ]]; then
			body[${#body[@]}]="$syntax"
		fi
	done
	(( ${#body[@]} == 5 )) || return 1
	[[ "${body[0]}" == "$tinygo_cleanup_command" ]] || return 1
	[[ "${body[1]}" == "$tinygo_download_command" ]] || return 1
	[[ "${body[2]}" == "$tinygo_stage_command" ]] || return 1
	[[ "${body[3]}" == "$tinygo_verify_command" ]] || return 1
	[[ "${body[4]}" == "$tinygo_install_command" ]]
}

validate_tinygo_job_controls() {
	local job_index="$1"
	local relative="${ci_files[${ci_job_file[job_index]}]}"
	local id="${ci_job_id[job_index]}"
	if (( ci_job_if_line[job_index] > 0 )); then
		fail "$relative:${ci_job_if_line[job_index]} protected TinyGo job $id must be unconditional and blocking: contains job-level if"
	fi
	if (( ci_job_continue_line[job_index] > 0 )); then
		fail "$relative:${ci_job_continue_line[job_index]} protected TinyGo job $id must be unconditional and blocking: contains job-level continue-on-error"
	fi
	if (( ci_job_needs_line[job_index] > 0 )); then
		fail "$relative:${ci_job_needs_line[job_index]} protected TinyGo job $id must be unconditional and blocking: contains job-level needs"
	fi
	if (( ci_job_runs_on_count[job_index] != 1 )); then
		fail "$relative:${ci_job_line[job_index]} protected TinyGo job $id must contain exactly one canonical runs-on: ubuntu-latest"
	elif [[ "${ci_job_runs_on_value[job_index]}" != ubuntu-latest ]]; then
		fail "$relative:${ci_job_runs_on_line[job_index]} protected TinyGo job $id must use canonical runs-on: ubuntu-latest"
	fi
	if (( ci_job_container_line[job_index] > 0 )); then
		fail "$relative:${ci_job_container_line[job_index]} protected TinyGo job $id must run directly on the trusted hosted runner without a job container"
	fi
}

evaluate_tinygo_workflow() {
	local file_index="$1"
	local relative="${ci_files[file_index]}"
	local job_index step_index pending=false job_sequences valid
	tinygo_sequence_count=0
	for ((job_index = 0; job_index < ${#ci_job_id[@]}; job_index++)); do
		if [[ "${ci_job_file[job_index]}" != "$file_index" ]]; then
			continue
		fi
		pending=false
		job_sequences=0
		for ((step_index = 0; step_index < ${#ci_step_file[@]}; step_index++)); do
			if [[ "${ci_step_job[step_index]}" != "$job_index" ]]; then
				continue
			fi
			if [[ "${ci_step_install_name[step_index]}" == true &&
				"${ci_step_verify_name[step_index]}" == true ]]; then
				fail "$relative:${ci_step_line[step_index]} TinyGo step has conflicting duplicate names"
				pending=false
				continue
			fi
			if [[ "${ci_step_install_name[step_index]}" == true ]]; then
				if [[ "$pending" == true ]]; then
					fail "$relative:${ci_step_line[step_index]} Verify TinyGo must immediately follow Install TinyGo"
				fi
				valid=true
				if (( ci_step_property_count[step_index] != 3 ||
					ci_step_name_count[step_index] != 1 ||
					ci_step_shell_count[step_index] != 1 ||
					ci_step_run_count[step_index] != 1 )) ||
					[[ "${ci_step_shell_value[step_index]}" != "$tinygo_shell_template" ||
						"${ci_step_run_value[step_index]}" != '|' ]] ||
					! tinygo_step_body_is_exact "$step_index"; then
					valid=false
				fi
				if [[ "$valid" != true ]]; then
					fail "$relative:${ci_step_line[step_index]} Install TinyGo must contain only the exact sanitized shell and protected run transaction"
					pending=false
				else
					pending=true
				fi
			elif [[ "${ci_step_verify_name[step_index]}" == true ]]; then
				valid=true
				if (( ci_step_property_count[step_index] != 3 ||
					ci_step_name_count[step_index] != 1 ||
					ci_step_shell_count[step_index] != 1 ||
					ci_step_run_count[step_index] != 1 )) ||
					[[ "${ci_step_shell_value[step_index]}" != "$tinygo_verify_shell_template" ||
						"${ci_step_run_value[step_index]}" != "$tinygo_version_command" ]]; then
					valid=false
				fi
				if [[ "$valid" != true ]]; then
					fail "$relative:${ci_step_line[step_index]} Verify TinyGo must contain only the exact privileged shell and package-owned version command"
					pending=false
				elif [[ "$pending" != true ]]; then
					fail "$relative:${ci_step_line[step_index]} Verify TinyGo must immediately follow Install TinyGo"
				else
					job_sequences=$((job_sequences + 1))
					tinygo_sequence_count=$((tinygo_sequence_count + 1))
					pending=false
				fi
			elif [[ "$pending" == true ]]; then
				fail "$relative:${ci_step_line[step_index]} Verify TinyGo must immediately follow Install TinyGo"
				pending=false
			fi
		done
		if [[ "$pending" == true ]]; then
			fail "$relative:${ci_job_line[job_index]} protected TinyGo job ${ci_job_id[job_index]} must keep Verify TinyGo immediately after Install TinyGo"
		fi
		if (( job_sequences > 0 )); then
			validate_tinygo_job_controls "$job_index"
		fi
	done
}

authored_ci_files=()
while IFS= read -r -d '' file; do
	authored_ci_files[${#authored_ci_files[@]}]="$file"
	parse_authored_ci_file "$file"
done < <(find "${scan_roots[@]}" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0)

validate_authored_action_refs
apply_authored_execution_policy

direct_tinygo_downloads=0
for file in "${authored_ci_files[@]}"; do
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
done
if (( direct_tinygo_downloads != ${#EXPECTED_TINYGO_WORKFLOWS[@]} )); then
	fail "expected ${#EXPECTED_TINYGO_WORKFLOWS[@]} direct TinyGo release downloads, found $direct_tinygo_downloads"
fi

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
	file_index=-1
	for ((index = 0; index < ${#ci_files[@]}; index++)); do
		if [[ "${ci_files[index]}" == "$relative" ]]; then
			file_index=$index
			break
		fi
	done
	if (( file_index < 0 )); then
		fail "$relative was not structurally inspected"
		continue
	fi
	evaluate_tinygo_workflow "$file_index"
	if (( tinygo_sequence_count != 1 )); then
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
