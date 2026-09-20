#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHECKER="$ROOT_DIR/scripts/ci-supply-chain-check.sh"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

EXPECTED_SHA="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
TINYGO_RELEASE_REPOSITORY="tinygo-org/"'tinygo'
TINYGO_RELEASE_REPOSITORY_CASE_VARIANT="TinyGo-Org/"'TinyGo'
TINYGO_RELEASE_NAMESPACE="${TINYGO_RELEASE_REPOSITORY}/releases/download/"
TINYGO_BINARY="/usr/local/bin/tinygo"
TINYGO_VERSION_COMMAND="$TINYGO_BINARY version"
VERIFIED_DEB="/tmp/goframe-tinygo-0.42.0-verified.deb"
CLEAN_PATH='/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin'
CLEAN_SHELL="/usr/bin/env -i PATH=$CLEAN_PATH /bin/bash --noprofile --norc -e -u -o pipefail {0}"
VERIFY_SHELL='/bin/bash --noprofile --norc -p -e -o pipefail {0}'
CLEANUP_COMMAND="trap '/usr/bin/sudo /usr/bin/rm -f $VERIFIED_DEB' EXIT"
STAGE_COMMAND="/usr/bin/sudo /usr/bin/install -o root -g root -m 0644 /tmp/tinygo.deb $VERIFIED_DEB"
CURL_COMMAND_NAME='cur''l'
WGET_COMMAND_NAME='wg''et'
FULL_ACTION_SHA="3d3c42e5aac5ba805825da76410c181273ba90b1"
FULL_ACTION_REF="actions/checkout@$FULL_ACTION_SHA # v7.0.1"
FULL_REUSABLE_REF="owner/repo/.github/workflows/build.yml@$FULL_ACTION_SHA # v1.2.3"
tests_run=0

write_tinygo_workflow() {
	local path="$1"
	local action_ref="$2"
	local checksum="$3"
	local verification_order="$4"
	local include_checksum="$5"
	local trust_root_mode="${6:-literal}"
	local verifier_mode="${7:-trusted}"
	local boundary_mode="${8:-protected}"
	local smoke_mode="${9:-separate}"
	local download_command verify_command verify_digest verify_input verify_target
	local shell_template="$CLEAN_SHELL"
	local cleanup_command="$CLEANUP_COMMAND"
	local stage_command="$STAGE_COMMAND"
	local install_target="$VERIFIED_DEB"

	mkdir -p "$(dirname "$path")"
	case "$trust_root_mode" in
		literal|variable-digest)
			download_command="/usr/bin/$CURL_COMMAND_NAME -fsSL -o /tmp/tinygo.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v0.42.0/tinygo_0.42.0_amd64.deb\""
			;;
		variable-url|variable-both)
			download_command="/usr/bin/$CURL_COMMAND_NAME -fsSL -o /tmp/tinygo.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v\${TINYGO_VERSION}/tinygo_\${TINYGO_VERSION}_amd64.deb\""
			;;
		*)
			printf 'unsupported trust-root mode: %s\n' "$trust_root_mode" >&2
			exit 1
			;;
	esac
	case "$boundary_mode" in
		protected) ;;
		legacy)
			shell_template=""
			cleanup_command=""
			stage_command=""
			install_target=/tmp/tinygo.deb
			;;
		no-staging)
			stage_command=""
			install_target=/tmp/tinygo.deb
			;;
		checksum-original) ;;
		install-original)
			install_target=/tmp/tinygo.deb
			;;
		missing-cleanup)
			cleanup_command=""
			;;
		mutable-shell)
			shell_template='bash {0}'
			;;
		*)
			printf 'unsupported boundary mode: %s\n' "$boundary_mode" >&2
			exit 1
			;;
	esac
	verify_target="$VERIFIED_DEB"
	case "$boundary_mode" in
		legacy|no-staging|checksum-original) verify_target=/tmp/tinygo.deb ;;
	esac
	case "$trust_root_mode" in
		literal|variable-url)
			verify_digest="'$checksum'"
			verify_input="'$checksum  $verify_target'"
			;;
		variable-digest|variable-both)
			verify_digest='"$TINYGO_SHA256"'
			verify_input="\"\$TINYGO_SHA256  $verify_target\""
			;;
	esac
	case "$verifier_mode" in
		unqualified)
			verify_command="printf '%s  %s\\n' $verify_digest $verify_target | sha256sum --check --strict -"
			;;
		trusted)
			verify_command="/usr/bin/env -i /usr/bin/sha256sum --check --strict - <<< $verify_input"
			;;
		*)
			printf 'unsupported verifier mode: %s\n' "$verifier_mode" >&2
			exit 1
			;;
	esac
	{
		printf 'name: fixture\n'
		if [[ "$trust_root_mode" != "literal" ]]; then
			printf 'env:\n'
			printf '  TINYGO_VERSION: 0.42.0\n'
			printf '  TINYGO_SHA256: %s\n' "$checksum"
		fi
		printf 'jobs:\n'
		printf '  fixture:\n'
		printf '    runs-on: ubuntu-latest\n'
		printf '    steps:\n'
		if [[ -n "$action_ref" ]]; then
			printf '      - uses: %s\n' "$action_ref"
		fi
		printf '      # uses: actions/commented-out@v1\n'
		if [[ "$smoke_mode" == "before-install" ]]; then
			printf '      - name: Verify TinyGo\n'
			printf '        shell: %s\n' "$VERIFY_SHELL"
			printf '        run: %s\n' "$TINYGO_VERSION_COMMAND"
		fi
		if [[ -n "$shell_template" ]]; then
			printf '      - name: Install TinyGo\n'
			printf '        shell: %s\n' "$shell_template"
			printf '        run: |\n'
		else
			printf '      - run: |\n'
		fi
		if [[ -n "$cleanup_command" ]]; then
			printf '          %s\n' "$cleanup_command"
		fi
		printf '          %s\n' "$download_command"
		if [[ -n "$stage_command" ]]; then
			printf '          %s\n' "$stage_command"
		fi
		if [[ "$verification_order" == "after-install" ]]; then
			printf '          /usr/bin/sudo /usr/bin/apt-get install -y %s\n' "$install_target"
		fi
		if [[ "$include_checksum" == "yes" ]]; then
			if [[ "$verification_order" == "commented" ]]; then
				printf '          # %s || exit 1\n' "$verify_command"
			elif [[ "$verification_order" == "ignored" ]]; then
				printf '          %s || true\n' "$verify_command"
			elif [[ "$verification_order" == "warned" ]]; then
				printf '          %s || echo warning\n' "$verify_command"
			elif [[ "$verification_order" == "skipped" ]]; then
				printf '          false && %s || exit 1\n' "$verify_command"
			elif [[ "$verification_order" == "implicit" ]]; then
				printf '          %s\n' "$verify_command"
			else
				printf '          %s || exit 1\n' "$verify_command"
			fi
		fi
		if [[ "$verification_order" == "replaced" ]]; then
			printf '          /usr/bin/cp /tmp/replacement.deb %s\n' "$install_target"
		fi
		if [[ "$verification_order" != "after-install" ]]; then
			printf '          /usr/bin/sudo /usr/bin/apt-get install -y %s\n' "$install_target"
		fi
		case "$smoke_mode" in
			inside)
				printf '          %s\n' "$TINYGO_VERSION_COMMAND"
				;;
			separate)
				printf '      - name: Verify TinyGo\n'
				printf '        shell: %s\n' "$VERIFY_SHELL"
				printf '        run: %s\n' "$TINYGO_VERSION_COMMAND"
				;;
			ambient)
				printf '      - name: Verify TinyGo\n'
				printf '        shell: %s\n' "$VERIFY_SHELL"
				printf '        run: tinygo version\n'
				;;
			missing|before-install) ;;
			malformed)
				printf '      - name: Verify TinyGo\n'
				printf '        shell: %s\n' "$VERIFY_SHELL"
				printf '        run: echo wrong command\n'
				;;
			custom-shell)
				printf '      - name: Verify TinyGo\n'
				printf '        shell: %s\n' "$CLEAN_SHELL"
				printf '        run: %s\n' "$TINYGO_VERSION_COMMAND"
				;;
			*)
				printf 'unsupported TinyGo smoke mode: %s\n' "$smoke_mode" >&2
				exit 1
				;;
		esac
	} > "$path"
}

make_fixture() {
	local name="$1"
	local action_ref="$2"
	local checksum="${3:-$EXPECTED_SHA}"
	local verification_order="${4:-before-install}"
	local include_checksum="${5:-yes}"
	local trust_root_mode="${6:-literal}"
	local verifier_mode="${7:-trusted}"
	local boundary_mode="${8:-protected}"
	local smoke_mode="${9:-separate}"
	local fixture="$TMP_ROOT/$name"

	write_tinygo_workflow "$fixture/.github/workflows/ci-core.yml" \
		"$action_ref" "$checksum" "$verification_order" "$include_checksum" "$trust_root_mode" "$verifier_mode" "$boundary_mode" "$smoke_mode"
	write_tinygo_workflow "$fixture/.github/workflows/ci-browser-smoke.yml" \
		"" "$checksum" "before-install" "$include_checksum" "$trust_root_mode" "$verifier_mode" "$boundary_mode" "$smoke_mode"
	write_tinygo_workflow "$fixture/.github/workflows/ci-wasm-size.yml" \
		"" "$checksum" "before-install" "$include_checksum" "$trust_root_mode" "$verifier_mode" "$boundary_mode" "$smoke_mode"
	printf '%s' "$fixture"
}

write_local_action() {
	local fixture="$1"
	local path="$2"
	local metadata="$3"
	local nested_ref="${4:-}"
	local target="$fixture/${path#./}"

	mkdir -p "$target"
	{
		printf 'name: local fixture\n'
		printf 'runs:\n'
		printf '  using: composite\n'
		printf '  steps:\n'
		if [[ -n "$nested_ref" ]]; then
			printf '    - uses: %s\n' "$nested_ref"
		else
			printf '    - shell: bash\n'
			printf '      run: echo local\n'
		fi
	} > "$target/$metadata"
}

write_local_workflow() {
	local fixture="$1"
	local path="$2"
	local nested_ref="${3:-}"
	local target="$fixture/${path#./}"

	mkdir -p "$(dirname "$target")"
	{
		printf 'name: reusable fixture\n'
		printf 'on:\n'
		printf '  workflow_call:\n'
		printf 'jobs:\n'
		printf '  fixture:\n'
		printf '    runs-on: ubuntu-latest\n'
		printf '    steps:\n'
		if [[ -n "$nested_ref" ]]; then
			printf '      - uses: %s\n' "$nested_ref"
		else
			printf '      - run: echo local\n'
		fi
	} > "$target"
}

add_runtime_override() {
	local file="$1"
	awk '
		/      - name: Install TinyGo/ && !inserted {
			print "      - run: |"
			print "          echo '\''TINYGO_VERSION=attacker-value'\'' >> \"$GITHUB_ENV\""
			print "          echo '\''TINYGO_SHA256=attacker-digest'\'' >> \"$GITHUB_ENV\""
			inserted = 1
		}
		{ print }
	' "$file" > "$TMP_ROOT/runtime-override.yml"
	mv "$TMP_ROOT/runtime-override.yml" "$file"
}

add_path_poisoning() {
	local file="$1"
	awk '
		/      - name: Install TinyGo/ && !inserted {
			print "      - run: |"
			print "          mkdir -p \"$RUNNER_TEMP/fake-bin\""
			print "          cp /bin/true \"$RUNNER_TEMP/fake-bin/sha256sum\""
			print "          echo \"$RUNNER_TEMP/fake-bin\" >> \"$GITHUB_PATH\""
			inserted = 1
		}
		{ print }
	' "$file" > "$TMP_ROOT/path-poison.yml"
	mv "$TMP_ROOT/path-poison.yml" "$file"
}

add_tinygo_path_poisoning() {
	local file="$1"

	awk '
		/      - name: Install TinyGo/ && !inserted {
			print "      - run: |"
			print "          mkdir -p \"$RUNNER_TEMP/fake-tinygo-bin\""
			print "          cp /bin/true \"$RUNNER_TEMP/fake-tinygo-bin/tinygo\""
			print "          echo \"$RUNNER_TEMP/fake-tinygo-bin\" >> \"$GITHUB_PATH\""
			inserted = 1
		}
		{ print }
	' "$file" > "$TMP_ROOT/tinygo-path-poison.yml"
	mv "$TMP_ROOT/tinygo-path-poison.yml" "$file"
}

add_bash_env_shadowing() {
	local file="$1"
	awk '
		/      - name: Install TinyGo/ && !inserted {
			print "      - run: |"
			print "          printf '\''%s\\n'\'' '\''sha256sum() { return 0; }'\'' '\''printf() { return 0; }'\'' > \"$RUNNER_TEMP/bash-env\""
			print "          echo \"BASH_ENV=$RUNNER_TEMP/bash-env\" >> \"$GITHUB_ENV\""
			inserted = 1
		}
		{ print }
	' "$file" > "$TMP_ROOT/bash-env-shadow.yml"
	mv "$TMP_ROOT/bash-env-shadow.yml" "$file"
}

add_bash_env_sudo_shadowing() {
	local file="$1"
	awk '
		/      - name: Install TinyGo/ && !inserted {
			print "      - run: |"
			print "          printf '\''%s\\n'\'' '\''sudo() { cp /tmp/replacement.deb /tmp/tinygo.deb; /usr/bin/sudo \"$@\"; }'\'' > \"$RUNNER_TEMP/bash-env\""
			print "          echo \"BASH_ENV=$RUNNER_TEMP/bash-env\" >> \"$GITHUB_ENV\""
			inserted = 1
		}
		{ print }
	' "$file" > "$TMP_ROOT/bash-env-sudo-shadow.yml"
	mv "$TMP_ROOT/bash-env-sudo-shadow.yml" "$file"
}

write_block_scalar_workflow() {
	local path="$1"
	local scalar="$2"

	mkdir -p "$(dirname "$path")"
	{
		printf 'jobs:\n'
		printf '  scalar:\n'
		printf '    steps:\n'
		printf '      - run: %s\n' "$scalar"
		printf '          echo "https://example.invalid"\n'
		printf '          echo "uses: actions/example@v1"\n'
	} > "$path"
}

write_repository_file() {
	local fixture="$1"
	local path="$2"
	local target="$fixture/$path"
	shift 2

	mkdir -p "$(dirname "$target")"
	printf '%s\n' "$@" > "$target"
}

write_shell_source() {
	local fixture="$1"
	local path="$2"
	shift 2
	local target="$fixture/$path"

	mkdir -p "$(dirname "$target")"
	{
		printf '#!/usr/bin/env bash\n'
		printf '%s\n' "$@"
	} > "$target"
}

add_workflow_script_invocation() {
	local file="$1"
	local command="$2"

	printf '\n  delegated-script:\n    steps:\n      - run: %s\n' "$command" >> "$file"
}

add_verify_property() {
	local file="$1"
	local position="$2"
	local property="$3"
	local child="${4:-}"

	awk -v position="$position" -v property="$property" -v child="$child" '
		function emit_property() {
			print "        " property
			if (child != "") {
				print "          " child
			}
		}
		$0 == "      - name: Verify TinyGo" { in_verify = 1 }
		in_verify && position == "before" && $0 ~ /^        run:/ {
			emit_property()
		}
		{ print }
		in_verify && position == "after" && $0 ~ /^        run:/ {
			emit_property()
			in_verify = 0
		}
	' "$file" > "$TMP_ROOT/verify-property.yml"
	mv "$TMP_ROOT/verify-property.yml" "$file"
}

add_install_property() {
	local file="$1"
	local position="$2"
	local property="$3"
	local child="${4:-}"

	awk -v position="$position" -v property="$property" -v child="$child" '
		function emit_property() {
			print "        " property
			if (child != "") {
				print "          " child
			}
		}
		$0 == "      - name: Install TinyGo" { in_install = 1 }
		in_install && position == "before" && $0 == "        run: |" {
			emit_property()
		}
		in_install && $0 == "      - name: Verify TinyGo" {
			if (position == "after") {
				emit_property()
			}
			in_install = 0
		}
		{ print }
	' "$file" > "$TMP_ROOT/install-property.yml"
	mv "$TMP_ROOT/install-property.yml" "$file"
}

replace_verify_shell() {
	local file="$1"
	local shell="$2"

	awk -v shell="$shell" '
		$0 == "      - name: Verify TinyGo" { in_verify = 1 }
		in_verify && $0 ~ /^        shell:/ {
			print "        shell: " shell
			in_verify = 0
			next
		}
		{ print }
	' "$file" > "$TMP_ROOT/verify-shell.yml"
	mv "$TMP_ROOT/verify-shell.yml" "$file"
}

remove_verify_shell() {
	local file="$1"

	awk '
		$0 == "      - name: Verify TinyGo" { in_verify = 1 }
		in_verify && $0 ~ /^        shell:/ {
			in_verify = 0
			next
		}
		{ print }
	' "$file" > "$TMP_ROOT/verify-shell.yml"
	mv "$TMP_ROOT/verify-shell.yml" "$file"
}

add_job_property() {
	local file="$1"
	local position="$2"
	local property="$3"
	local child="${4:-}"

	if [[ "$position" == before ]]; then
		awk -v property="$property" -v child="$child" '
			{ print }
			$0 == "  fixture:" {
				print "    " property
				if (child != "") {
					print "      " child
				}
			}
		' "$file" > "$TMP_ROOT/job-property.yml"
		mv "$TMP_ROOT/job-property.yml" "$file"
		return
	fi

	printf '    %s\n' "$property" >> "$file"
	if [[ -n "$child" ]]; then
		printf '      %s\n' "$child" >> "$file"
	fi
}

replace_fixture_runs_on() {
	local file="$1"
	local value="$2"
	local child="${3:-}"

	awk -v value="$value" -v child="$child" '
		$0 == "  fixture:" { in_fixture = 1 }
		in_fixture && !replaced && $0 ~ /^    runs-on:/ {
			if (value == "") {
				print "    runs-on:"
			} else {
				print "    runs-on: " value
			}
			if (child != "") {
				print "      " child
			}
			replaced = 1
			next
		}
		{ print }
	' "$file" > "$TMP_ROOT/runs-on.yml"
	mv "$TMP_ROOT/runs-on.yml" "$file"
}

remove_fixture_runs_on() {
	local file="$1"

	awk '
		$0 == "  fixture:" { in_fixture = 1 }
		in_fixture && !removed && $0 ~ /^    runs-on:/ {
			removed = 1
			next
		}
		{ print }
	' "$file" > "$TMP_ROOT/runs-on.yml"
	mv "$TMP_ROOT/runs-on.yml" "$file"
}

add_unrelated_runner_jobs() {
	local file="$1"

	printf '%s\n' \
		'  unrelated-container:' \
		'    runs-on: self-hosted' \
		'    container: attacker/unrelated:latest' \
		'    steps:' \
		'      - run: echo unrelated' \
		>> "$file"
}

add_workflow_run_default() {
	local file="$1"

	awk '
		$0 == "jobs:" {
			print "defaults:"
			print "  run:"
			print "    shell: sh"
		}
		{ print }
	' "$file" > "$TMP_ROOT/workflow-default.yml"
	mv "$TMP_ROOT/workflow-default.yml" "$file"
}

add_job_run_default() {
	local file="$1"

	awk '
		{ print }
		$0 == "  fixture:" {
			print "    defaults:"
			print "      run:"
			print "        shell: sh"
		}
	' "$file" > "$TMP_ROOT/job-default.yml"
	mv "$TMP_ROOT/job-default.yml" "$file"
}

add_job_strategy_matrix() {
	local file="$1"

	awk '
		{ print }
		$0 == "  fixture:" {
			print "    strategy:"
			print "      fail-fast: false"
			print "      matrix:"
			print "        go:"
			print "          - 1.26.6"
		}
	' "$file" > "$TMP_ROOT/job-strategy.yml"
	mv "$TMP_ROOT/job-strategy.yml" "$file"
}

add_unrelated_control_jobs() {
	local file="$1"

	printf '%s\n' \
		'  prerequisite:' \
		'    runs-on: ubuntu-latest' \
		'    steps:' \
		'      - run: echo prerequisite' \
		'  unrelated:' \
		'    if: true' \
		'    continue-on-error: false' \
		'    needs: prerequisite' \
		'    runs-on: ubuntu-latest' \
		'    steps:' \
		'      - run: echo unrelated' \
		>> "$file"
}

split_verify_into_new_job() {
	local file="$1"

	awk '
		$0 == "      - name: Verify TinyGo" {
			print "  verify-tinygo:"
			print "    runs-on: ubuntu-latest"
			print "    steps:"
		}
		{ print }
	' "$file" > "$TMP_ROOT/cross-job-verify.yml"
	mv "$TMP_ROOT/cross-job-verify.yml" "$file"
}

reorder_verify_properties() {
	local file="$1"

	awk '
		$0 == "      - name: Verify TinyGo" {
			name = "        name: Verify TinyGo"
			if ((getline shell_line) <= 0 || (getline run_line) <= 0) {
				exit 1
			}
			sub(/^        run:/, "      - run:", run_line)
			print run_line
			print name
			print shell_line
			next
		}
		{ print }
	' "$file" > "$TMP_ROOT/reordered-verify.yml"
	mv "$TMP_ROOT/reordered-verify.yml" "$file"
}

move_install_shell_after_run() {
	local file="$1"

	awk '
		$0 == "      - name: Install TinyGo" { in_install = 1 }
		in_install && $0 ~ /^        shell:/ {
			shell_line = $0
			next
		}
		in_install && $0 == "      - name: Verify TinyGo" {
			print shell_line
			in_install = 0
		}
		{ print }
	' "$file" > "$TMP_ROOT/reordered-install.yml"
	mv "$TMP_ROOT/reordered-install.yml" "$file"
}

expect_pass() {
	local name="$1"
	local fixture="$2"
	tests_run=$((tests_run + 1))
	if ! output="$("$BASH" "$CHECKER" "$fixture" 2>&1)"; then
		printf 'not ok %d - %s\n%s\n' "$tests_run" "$name" "$output" >&2
		exit 1
	fi
	printf 'ok %d - %s\n' "$tests_run" "$name"
}

expect_fail() {
	local name="$1"
	local fixture="$2"
	tests_run=$((tests_run + 1))
	if output="$("$BASH" "$CHECKER" "$fixture" 2>&1)"; then
		printf 'not ok %d - %s unexpectedly passed\n%s\n' "$tests_run" "$name" "$output" >&2
		exit 1
	fi
	printf 'ok %d - %s\n' "$tests_run" "$name"
}

test_clean_shell_boundary() {
	local marker_dir="$TMP_ROOT/clean-shell-markers"
	local fake_bin="$TMP_ROOT/clean-shell-fake-bin"
	local bash_env="$TMP_ROOT/clean-shell-bash-env"
	local child_script="$TMP_ROOT/clean-shell-child.sh"
	local driver_script="$TMP_ROOT/clean-shell-driver.sh"
	local failing_script="$TMP_ROOT/clean-shell-failing.sh"
	local command_name

	mkdir -p "$marker_dir" "$fake_bin"
	printf '%s\n' \
		'mark_override() { : > "$P1_MARKER_DIR/$1"; }' \
		'sudo() { mark_override sudo; return 99; }' \
		"$CURL_COMMAND_NAME() { mark_override $CURL_COMMAND_NAME; return 99; }" \
		'sha256sum() { mark_override sha256sum; return 99; }' \
		'printf() { mark_override printf; return 99; }' \
		'function /usr/bin/env() { mark_override env; return 99; }' \
		> "$bash_env"
	for command_name in sudo "$CURL_COMMAND_NAME" sha256sum printf; do
		printf '%s\n' '#!/bin/sh' ": > \"$marker_dir/path-$command_name\"" 'exit 99' \
			> "$fake_bin/$command_name"
		chmod +x "$fake_bin/$command_name"
	done
	printf '%s\n' \
		'[[ -z "${BASH_ENV+x}" ]]' \
		"[[ \"\$PATH\" == '$CLEAN_PATH' ]]" \
		"for function_name in sudo $CURL_COMMAND_NAME sha256sum printf /usr/bin/env; do" \
		'  ! declare -F "$function_name" >/dev/null' \
		'done' \
		> "$child_script"
	printf '%s\n' \
		"command /usr/bin/env -i PATH=$CLEAN_PATH /bin/bash --noprofile --norc -e -u -o pipefail \"$child_script\"" \
		> "$driver_script"

	tests_run=$((tests_run + 1))
	if ! BASH_ENV="$bash_env" P1_MARKER_DIR="$marker_dir" PATH="$fake_bin" \
		/bin/bash --noprofile --norc "$driver_script"; then
		printf 'not ok %d - sanitized custom-shell boundary\n' "$tests_run" >&2
		exit 1
	fi
	if find "$marker_dir" -type f -print -quit | grep -q .; then
		printf 'not ok %d - parent BASH_ENV or PATH override entered clean shell\n' "$tests_run" >&2
		exit 1
	fi
	printf 'ok %d - sanitized custom-shell boundary excludes parent BASH_ENV and PATH\n' "$tests_run"

	printf 'false\n' > "$failing_script"
	printf '%s\n' \
		"command /usr/bin/env -i PATH=$CLEAN_PATH /bin/bash --noprofile --norc -e -u -o pipefail \"$failing_script\"" \
		> "$driver_script"
	tests_run=$((tests_run + 1))
	if BASH_ENV="$bash_env" P1_MARKER_DIR="$marker_dir" PATH="$fake_bin" \
		/bin/bash --noprofile --norc "$driver_script"; then
		printf 'not ok %d - sanitized custom-shell failure unexpectedly passed\n' "$tests_run" >&2
		exit 1
	fi
	printf 'ok %d - sanitized custom-shell failure propagates\n' "$tests_run"
}

test_ambient_sudo_interposition() {
	local artifact="$TMP_ROOT/ambient-tinygo.deb"
	local replacement="$TMP_ROOT/ambient-replacement.deb"
	local consumed="$TMP_ROOT/ambient-consumed.deb"
	local marker="$TMP_ROOT/ambient-sudo-ran"
	local bash_env="$TMP_ROOT/ambient-bash-env"
	local script="$TMP_ROOT/ambient-interposition.sh"
	local cp_bin digest verify_command

	cp_bin="$(command -v cp || true)"
	if [[ "$cp_bin" != /* || ! -x "$cp_bin" ]]; then
		printf 'not ok - ambient sudo interposition oracle requires an absolute executable cp\n' >&2
		exit 1
	fi

	printf 'verified artifact\n' > "$artifact"
	printf 'replacement artifact\n' > "$replacement"
	if [[ -x /usr/bin/sha256sum ]]; then
		digest="$(/usr/bin/sha256sum "$artifact" | awk '{print $1}')"
		verify_command="/usr/bin/env -i /usr/bin/sha256sum --check --strict - <<< '$digest  $artifact' || exit 1"
	elif [[ -x /usr/bin/shasum ]]; then
		digest="$(/usr/bin/shasum -a 256 "$artifact" | awk '{print $1}')"
		verify_command="/usr/bin/printf '%s  %s\\n' '$digest' '$artifact' | /usr/bin/shasum -a 256 --check - || exit 1"
	else
		printf 'skip - ambient sudo interposition oracle requires sha256sum or shasum\n'
		return
	fi
	printf '%s\n' \
		'sudo() {' \
		'  : > "$P1_MARKER"' \
		"  \"$cp_bin\" \"\$P1_REPLACEMENT\" \"\$P1_ARTIFACT\"" \
		'  "$@"' \
		'}' \
		> "$bash_env"
	printf '%s\n' "$verify_command" \
		"sudo \"$cp_bin\" \"\$P1_ARTIFACT\" \"\$P1_CONSUMED\"" \
		> "$script"

	tests_run=$((tests_run + 1))
	if ! BASH_ENV="$bash_env" P1_MARKER="$marker" P1_REPLACEMENT="$replacement" \
		P1_ARTIFACT="$artifact" P1_CONSUMED="$consumed" \
		/bin/bash --noprofile --norc "$script"; then
		printf 'not ok %d - ambient sudo interposition oracle execution\n' "$tests_run" >&2
		exit 1
	fi
	if [[ ! -f "$marker" ]] || ! /usr/bin/cmp -s "$replacement" "$consumed"; then
		printf 'not ok %d - ambient sudo did not replace verified bytes\n' "$tests_run" >&2
		exit 1
	fi
	printf 'ok %d - ambient sudo can replace bytes after verification\n' "$tests_run"
}

test_verify_shell_boundary() {
	local oracle_dir="$TMP_ROOT/verify-shell-oracle"
	local target="$oracle_dir/tinygo"
	local marker="$oracle_dir/interposed"
	local bash_env="$oracle_dir/bash-env"
	local target_script="$oracle_dir/run-target.sh"
	local environment_script="$oracle_dir/check-environment.sh"
	local output

	mkdir -p "$oracle_dir"
	printf '%s\n' '#!/bin/sh' 'printf "real tinygo\\n"' > "$target"
	chmod +x "$target"
	printf 'function %s {\n' "$target" > "$bash_env"
	printf '%s\n' \
		'  printf "fake tinygo\n"' \
		'  : > "$VERIFY_MARKER"' \
		'  return 0' \
		'}' \
		>> "$bash_env"
	printf '%s\n' '"$VERIFY_TARGET" version' > "$target_script"
	printf '%s\n' \
		'[[ "$VERIFY_ENV_SENTINEL" == "setup-go-visible" ]]' \
		'printf "%s\n" "$VERIFY_ENV_SENTINEL"' \
		> "$environment_script"

	rm -f "$marker"
	tests_run=$((tests_run + 1))
	if ! output="$(BASH_ENV="$bash_env" VERIFY_MARKER="$marker" \
		VERIFY_TARGET="$target" /bin/bash --noprofile --norc -e -o pipefail \
		"$target_script")"; then
		printf 'not ok %d - ordinary Bash BASH_ENV interposition oracle execution\n' \
			"$tests_run" >&2
		exit 1
	fi
	if [[ ! -f "$marker" || "$output" != 'fake tinygo' ]]; then
		printf 'not ok %d - ordinary Bash did not load slash-named BASH_ENV function\n' \
			"$tests_run" >&2
		exit 1
	fi
	printf 'ok %d - ordinary Bash loads slash-named BASH_ENV function\n' "$tests_run"

	rm -f "$marker"
	tests_run=$((tests_run + 1))
	if ! output="$(BASH_ENV="$bash_env" VERIFY_MARKER="$marker" \
		VERIFY_TARGET="$target" /bin/bash --noprofile --norc -p -e -o pipefail \
		"$target_script")"; then
		printf 'not ok %d - privileged Verify shell oracle execution\n' \
			"$tests_run" >&2
		exit 1
	fi
	if [[ -e "$marker" || "$output" != 'real tinygo' ]]; then
		printf 'not ok %d - privileged Verify shell allowed BASH_ENV interposition\n' \
			"$tests_run" >&2
		exit 1
	fi
	printf 'ok %d - privileged Verify shell blocks BASH_ENV interposition\n' \
		"$tests_run"

	tests_run=$((tests_run + 1))
	if ! output="$(BASH_ENV="$bash_env" VERIFY_ENV_SENTINEL=setup-go-visible \
		/bin/bash --noprofile --norc -p -e -o pipefail "$environment_script")"; then
		printf 'not ok %d - privileged Verify shell environment oracle execution\n' \
			"$tests_run" >&2
		exit 1
	fi
	if [[ "$output" != setup-go-visible ]]; then
		printf 'not ok %d - privileged Verify shell lost normal environment\n' \
			"$tests_run" >&2
		exit 1
	fi
	printf 'ok %d - privileged Verify shell preserves normal environment\n' \
		"$tests_run"
}

expect_pass "protected TinyGo job without conditional controls" \
	"$(make_fixture protected-job-controls "$FULL_ACTION_REF")"

fixture="$(make_fixture execution-data-contexts "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/execution-data.yml \
	'name: execution data' \
	'env:' \
	"  run: $CURL_COMMAND_NAME" \
	"  shell: $CURL_COMMAND_NAME" \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	"      - uses: actions/checkout@$FULL_ACTION_SHA # v7.0.1" \
	'        with:' \
	"          run: $CURL_COMMAND_NAME" \
	"          shell: $CURL_COMMAND_NAME"
expect_pass "run and shell keys in env and with are ordinary data" "$fixture"

fixture="$(make_fixture action-execution-data-contexts "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/actions/data/action.yml \
	'name: data' \
	'inputs:' \
	'  run:' \
	"    default: $CURL_COMMAND_NAME" \
	'outputs:' \
	'  shell:' \
	"    value: $CURL_COMMAND_NAME" \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    - shell: bash' \
	'      run: echo ok'
expect_pass "run and shell keys in Action inputs and outputs are ordinary data" "$fixture"

fixture="$(make_fixture workflow-step-run-downloader "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/execution.yml \
	'name: execution' \
	'jobs:' \
	'  run:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	"      - run: $CURL_COMMAND_NAME https://example.invalid/file"
expect_fail "workflow step run receives downloader policy" "$fixture"

fixture="$(make_fixture workflow-step-shell-downloader "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/execution.yml \
	'name: execution' \
	'jobs:' \
	'  shell:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	"      - shell: bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}" \
	'        run: echo harmless'
expect_fail "workflow step shell receives downloader policy" "$fixture"

fixture="$(make_fixture workflow-default-shell-downloader "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/execution.yml \
	'name: execution' \
	'jobs:' \
	'  shell:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - run: echo harmless' \
	'defaults:' \
	'  run:' \
	"    shell: bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}"
expect_fail "workflow defaults run shell receives downloader policy" "$fixture"

fixture="$(make_fixture job-default-shell-downloader "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/execution.yml \
	'name: execution' \
	'jobs:' \
	'  shell:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - run: echo harmless' \
	'    defaults:' \
	'      run:' \
	"        shell: bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}"
expect_fail "job defaults run shell receives downloader policy" "$fixture"

fixture="$(make_fixture composite-run-downloader './.github/actions/composite')"
write_repository_file "$fixture" .github/actions/composite/action.yml \
	'name: composite' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    - shell: bash' \
	"      run: $CURL_COMMAND_NAME https://example.invalid/file"
expect_fail "composite step run receives downloader policy" "$fixture"

fixture="$(make_fixture composite-shell-downloader './.github/actions/composite')"
write_repository_file "$fixture" .github/actions/composite/action.yml \
	'name: composite' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	"    - shell: bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}" \
	'      run: echo harmless'
expect_fail "composite step shell receives downloader policy" "$fixture"

for scalar in '|' '>' '|-' '>+' '|2-'; do
	fixture="$(make_fixture "workflow-step-block-shell-$tests_run" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" .github/workflows/executable-shell.yml \
		'name: executable shell' \
		'jobs:' \
		'  shell:' \
		'    runs-on: ubuntu-latest' \
		'    steps:' \
		"      - shell: $scalar" \
		"          bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}" \
		'        run: echo harmless'
	expect_fail "workflow step executable shell block scalar $scalar" "$fixture"
done

fixture="$(make_fixture workflow-default-block-shell "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/executable-shell.yml \
	'name: executable shell' \
	'defaults:' \
	'  run:' \
	'    shell: |' \
	"      bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}" \
	'jobs:' \
	'  shell:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - run: echo harmless'
expect_fail "workflow defaults executable shell block scalar" "$fixture"

fixture="$(make_fixture job-default-block-shell "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/executable-shell.yml \
	'name: executable shell' \
	'jobs:' \
	'  shell:' \
	'    runs-on: ubuntu-latest' \
	'    defaults:' \
	'      run:' \
	'        shell: >' \
	"          bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}" \
	'    steps:' \
	'      - run: echo harmless'
expect_fail "job defaults executable shell folded scalar" "$fixture"

fixture="$(make_fixture composite-block-shell './.github/actions/composite')"
write_repository_file "$fixture" .github/actions/composite/action.yml \
	'name: composite' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    - shell: |-' \
	"        bash -c '$CURL_COMMAND_NAME https://example.invalid/file; bash \"\$1\"' -- {0}" \
	'      run: echo harmless'
expect_fail "composite executable shell block scalar" "$fixture"

fixture="$(make_fixture data-block-shell "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/data-shell.yml \
	'name: data shell' \
	'env:' \
	'  shell: |' \
	'    harmless environment data' \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	"      - uses: actions/checkout@$FULL_ACTION_SHA # v7.0.1" \
	'        with:' \
	'          shell: >' \
	'            harmless input data'
expect_pass "block scalar shell keys in env and with remain ordinary data" "$fixture"

fixture="$(make_fixture inline-pwsh-shell "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/inline-shell.yml \
	'name: inline shell' \
	'jobs:' \
	'  shell:' \
	'    runs-on: windows-latest' \
	'    steps:' \
	'      - shell: pwsh' \
	"        run: Write-Output 'ok'"
expect_pass "canonical inline executable shell remains accepted" "$fixture"

fixture="$(make_fixture local-docker-action './.github/actions/docker')"
write_repository_file "$fixture" .github/actions/docker/action.yml \
	'name: docker' \
	'runs:' \
	'  using: docker' \
	'  image: Dockerfile'
write_repository_file "$fixture" .github/actions/docker/Dockerfile \
	'FROM ubuntu:latest' \
	"RUN base=https://github.com/$TINYGO_RELEASE_REPOSITORY; \\" \
	"    $CURL_COMMAND_NAME \"\$base/releases/download/v0.42.0/file\""
expect_fail "local Docker Action runtime is unsupported" "$fixture"

fixture="$(make_fixture local-docker-image-action './.github/actions/docker-image')"
write_repository_file "$fixture" .github/actions/docker-image/action.yml \
	'name: docker image' \
	'runs:' \
	'  using: docker' \
	'  image: docker://alpine:latest'
expect_fail "local Docker image Action runtime is unsupported" "$fixture"

for runtime in node20 node24; do
	fixture="$(make_fixture "local-$runtime-action" './.github/actions/javascript')"
	write_repository_file "$fixture" .github/actions/javascript/action.yml \
		'name: javascript' \
		'runs:' \
		"  using: $runtime" \
		'  main: dist/action'
	write_repository_file "$fixture" .github/actions/javascript/dist/action \
		"const base = 'https://github.com/$TINYGO_RELEASE_REPOSITORY';" \
		"spawn('$CURL_COMMAND_NAME', [base + '/releases/download/v0.42.0/file']);"
	expect_fail "local $runtime Action runtime is unsupported" "$fixture"
done

for mode in unknown expression quoted missing duplicate; do
	fixture="$(make_fixture "local-action-runtime-$mode" './.github/actions/runtime')"
	case "$mode" in
		unknown)
			lines=('name: runtime' 'runs:' '  using: custom-runtime')
			;;
		expression)
			lines=('name: runtime' 'runs:' '  using: ${{ inputs.runtime }}')
			;;
		quoted)
			lines=('name: runtime' 'runs:' '  using: "composite"' '  steps:' '    - shell: bash' '      run: echo local')
			;;
		missing)
			lines=('name: runtime' 'runs:' '  steps:' '    - shell: bash' '      run: echo local')
			;;
		duplicate)
			lines=('name: runtime' 'runs:' '  using: composite' '  using: composite' '  steps:' '    - shell: bash' '      run: echo local')
			;;
	esac
	write_repository_file "$fixture" .github/actions/runtime/action.yml "${lines[@]}"
	expect_fail "local Action $mode runs using contract" "$fixture"
done

fixture="$(make_fixture direct-docker-action "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/docker.yml \
	'name: docker' \
	'jobs:' \
	'  docker:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - uses: docker://alpine:latest'
expect_fail "direct docker Action runtime is unsupported" "$fixture"

for primitive in source dot bash bash-option sh; do
	fixture="$(make_fixture "unknown-extension-$primitive" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" scripts/bootstrap.inc \
		"base=https://github.com/$TINYGO_RELEASE_REPOSITORY" \
		"env $CURL_COMMAND_NAME \"\$base/releases/download/v0.42.0/file\""
	case "$primitive" in
		source) command='source scripts/bootstrap.inc' ;;
		dot) command='. scripts/bootstrap.inc' ;;
		bash) command='bash scripts/bootstrap.inc' ;;
		bash-option) command='bash -e scripts/bootstrap.inc' ;;
		sh) command='sh scripts/bootstrap.inc' ;;
	esac
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "$primitive force-scans an unknown-extension shell target" "$fixture"
done

for wrapper in command env env-assignment exec; do
	fixture="$(make_fixture "wrapped-helper-$wrapper" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" scripts/bootstrap.inc \
		"base=https://github.com/$TINYGO_RELEASE_REPOSITORY" \
		"env $CURL_COMMAND_NAME \"\$base/releases/download/v0.42.0/file\""
	case "$wrapper" in
		command) command='command bash scripts/bootstrap.inc' ;;
		env) command='env bash scripts/bootstrap.inc' ;;
		env-assignment) command='env FOO=bar bash scripts/bootstrap.inc' ;;
		exec) command='exec bash scripts/bootstrap.inc' ;;
	esac
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "$wrapper wrapper preserves helper discovery" "$fixture"
done

for command in \
	'sudo bash scripts/bootstrap.inc' \
	'timeout 30 bash scripts/bootstrap.inc' \
	'nice bash scripts/bootstrap.inc' \
	'nohup bash scripts/bootstrap.inc' \
	'stdbuf -oL bash scripts/bootstrap.inc' \
	'xargs bash scripts/bootstrap.inc' \
	'arbitrary-wrapper bash scripts/bootstrap.inc'; do
	fixture="$(make_fixture "unsupported-helper-indirection-$tests_run" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" scripts/bootstrap.inc \
		"base=https://github.com/$TINYGO_RELEASE_REPOSITORY" \
		"env $CURL_COMMAND_NAME \"\$base/releases/download/v0.42.0/file\""
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "unsupported repository helper indirection: $command" "$fixture"
done

for command in \
	'sudo python3 tools/bootstrap.resource' \
	'arbitrary-wrapper node tools/bootstrap.resource'; do
	fixture="$(make_fixture "unsupported-interpreter-indirection-$tests_run" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" tools/bootstrap.resource \
		"provenance=https://github.com/$TINYGO_RELEASE_REPOSITORY"
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "unsupported non-shell helper indirection: $command" "$fixture"
done

fixture="$(make_fixture nested-unsupported-helper-indirection "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/outer.sh \
	'#!/usr/bin/env bash' \
	'arbitrary-wrapper bash scripts/bootstrap.inc'
write_repository_file "$fixture" scripts/bootstrap.inc \
	"provenance=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'bash scripts/outer.sh'
expect_fail "unsupported helper indirection inside a scanned shell source" "$fixture"

for command in \
	'sudo ./scripts/executable-helper' \
	'timeout 10 scripts/executable-helper'; do
	fixture="$(make_fixture "wrapped-executable-helper-$tests_run" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" scripts/executable-helper 'printf "benign\\n"'
	chmod +x "$fixture/scripts/executable-helper"
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "wrapped executable repository helper: $command" "$fixture"
done

fixture="$(make_fixture wrapped-working-directory-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
write_repository_file "$fixture" .github/workflows/wrapped-working-directory.yml \
	'name: wrapped working directory' \
	'jobs:' \
	'  helper:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - working-directory: tools' \
	'        run: sudo bash bootstrap.inc'
expect_fail "working-directory resolves a helper behind unsupported indirection" "$fixture"

fixture="$(make_fixture wrapped-symlink-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" real-tools/helper.inc 'printf "benign\\n"'
ln -s real-tools "$fixture/tools"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'sudo bash tools/helper.inc'
expect_fail "wrapped helper target with a symlink path component is rejected" "$fixture"

fixture="$(make_fixture wrapped-traversed-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/helper.inc 'printf "benign\\n"'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'sudo bash tools/../tools/helper.inc'
expect_fail "wrapped helper target traversal is rejected" "$fixture"

fixture="$(make_fixture command-substitution-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'result="$(bash scripts/bootstrap.inc)"'
expect_fail "command substitution cannot hide a repository helper" "$fixture"

fixture="$(make_fixture backtick-substitution-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'result=`bash scripts/bootstrap.inc`'
expect_fail "backtick substitution cannot hide a repository helper" "$fixture"

fixture="$(make_fixture literal-substitution-data "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/bootstrap.inc 'printf "benign\\n"'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	"echo '\$(bash scripts/bootstrap.inc)'"
expect_pass "single-quoted substitution-shaped text remains ordinary data" "$fixture"

fixture="$(make_fixture subshell-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'( bash scripts/bootstrap.inc )'
expect_fail "subshell preserves repository helper discovery" "$fixture"

fixture="$(make_fixture repository-data-arguments "$FULL_ACTION_REF")"
write_repository_file "$fixture" docs/file.txt 'harmless data'
write_repository_file "$fixture" scripts/data.txt 'pattern'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'cat docs/file.txt'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'grep pattern scripts/data.txt'
expect_pass "non-executable repository data arguments remain supported" "$fixture"

fixture="$(make_fixture env-chdir-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'env -C tools bash bootstrap.inc'
expect_fail "env chdir option cannot hide a repository helper" "$fixture"

fixture="$(make_fixture exec-argv0-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'exec -a bootstrap bash scripts/bootstrap.inc'
expect_fail "exec argv0 option cannot hide a repository helper" "$fixture"

fixture="$(make_fixture node-preload-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/preload.resource \
	"provenance=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'node --require ./tools/preload.resource -e "console.log(1)"'
expect_fail "Node preload force-scans a repository helper" "$fixture"

for primitive in python python3 node pwsh powershell; do
	fixture="$(make_fixture "interpreted-helper-$primitive" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" tools/bootstrap.resource \
		"provenance=https://github.com/$TINYGO_RELEASE_REPOSITORY"
	case "$primitive" in
		pwsh) command='pwsh -File tools/bootstrap.resource' ;;
		powershell) command='powershell -File tools/bootstrap.resource' ;;
		*) command="$primitive tools/bootstrap.resource" ;;
	esac
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "$primitive force-scans an unknown-extension target" "$fixture"
done

fixture="$(make_fixture direct-unknown-extension "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/runner.data \
	"provenance=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'./tools/runner.data'
expect_fail "direct repository path force-scans an unknown-extension target" "$fixture"

fixture="$(make_fixture direct-unknown-extension-without-dot "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/runner.data \
	"provenance=https://github.com/$TINYGO_RELEASE_REPOSITORY"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'tools/runner.data'
expect_fail "direct repository path without dot prefix is force-scanned" "$fixture"

fixture="$(make_fixture quoted-helper-path "$FULL_ACTION_REF")"
write_repository_file "$fixture" 'tools/path with spaces.inc' \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY" \
	"env $CURL_COMMAND_NAME \"\$base/releases/download/v0.42.0/file\""
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'bash "tools/path with spaces.inc"'
expect_fail "quoted repository helper path with spaces is resolved" "$fixture"

for scope in workflow job step; do
	fixture="$(make_fixture "working-directory-$scope" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" tools/bootstrap.inc \
		"base=https://github.com/$TINYGO_RELEASE_REPOSITORY" \
		"env $CURL_COMMAND_NAME \"\$base/releases/download/v0.42.0/file\""
	case "$scope" in
		workflow)
			write_repository_file "$fixture" .github/workflows/working-directory.yml \
				'name: working directory' \
				'jobs:' \
				'  helper:' \
				'    runs-on: ubuntu-latest' \
				'    steps:' \
				'      - run: bash bootstrap.inc' \
				'defaults:' \
				'  run:' \
				'    working-directory: tools'
			;;
		job)
			write_repository_file "$fixture" .github/workflows/working-directory.yml \
				'name: working directory' \
				'jobs:' \
				'  helper:' \
				'    runs-on: ubuntu-latest' \
				'    steps:' \
				'      - run: bash bootstrap.inc' \
				'    defaults:' \
				'      run:' \
				'        working-directory: tools'
			;;
		step)
			write_repository_file "$fixture" .github/workflows/working-directory.yml \
				'name: working directory' \
				'jobs:' \
				'  helper:' \
				'    runs-on: ubuntu-latest' \
				'    steps:' \
				'      - run: bash bootstrap.inc' \
				'        working-directory: tools'
			;;
	esac
	expect_fail "$scope working-directory resolves the executed helper" "$fixture"
done

fixture="$(make_fixture working-directory-precedence "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/malicious/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
write_repository_file "$fixture" tools/benign/bootstrap.inc 'printf "benign\\n"'
write_repository_file "$fixture" .github/workflows/working-directory.yml \
	'name: working directory precedence' \
	'defaults:' \
	'  run:' \
	'    working-directory: tools/malicious' \
	'jobs:' \
	'  helper:' \
	'    runs-on: ubuntu-latest' \
	'    defaults:' \
	'      run:' \
	'        working-directory: tools/benign' \
	'    steps:' \
	'      - run: bash bootstrap.inc'
expect_pass "job working-directory overrides workflow default" "$fixture"

fixture="$(make_fixture step-working-directory-precedence "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/malicious/bootstrap.inc \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
write_repository_file "$fixture" tools/benign/bootstrap.inc 'printf "benign\\n"'
write_repository_file "$fixture" .github/workflows/working-directory.yml \
	'name: working directory precedence' \
	'defaults:' \
	'  run:' \
	'    working-directory: tools/malicious' \
	'jobs:' \
	'  helper:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - run: bash bootstrap.inc' \
	'        working-directory: tools/benign'
expect_pass "step working-directory overrides inherited defaults" "$fixture"

fixture="$(make_fixture dynamic-working-directory-no-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/working-directory.yml \
	'name: dynamic working directory' \
	'jobs:' \
	'  harmless:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - working-directory: ${{ matrix.directory }}' \
	'        run: echo harmless'
expect_pass "dynamic working-directory without local helper remains supported" "$fixture"

fixture="$(make_fixture dynamic-step-shell "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/dynamic-shell.yml \
	'name: dynamic shell' \
	'jobs:' \
	'  dynamic:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - shell: ${{ matrix.shell }}' \
	'        run: echo harmless'
expect_fail "dynamic executable step shell fails closed" "$fixture"

for mode in traversal backslash expression; do
	fixture="$(make_fixture "unsafe-working-directory-$mode" "$FULL_ACTION_REF")"
	write_repository_file "$fixture" tools/bootstrap.inc 'printf "benign\\n"'
	case "$mode" in
		traversal) directory='tools/../tools' ;;
		backslash) directory='tools\subdir' ;;
		expression) directory='${{ matrix.directory }}' ;;
	esac
	write_repository_file "$fixture" .github/workflows/working-directory.yml \
		'name: unsafe working directory' \
		'jobs:' \
		'  helper:' \
		'    runs-on: ubuntu-latest' \
		'    steps:' \
		"      - working-directory: $directory" \
		'        run: bash bootstrap.inc'
	expect_fail "$mode working-directory fails closed for helper resolution" "$fixture"
done

fixture="$(make_fixture traversed-helper-target "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/bootstrap.inc 'printf "benign\\n"'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'bash tools/../tools/bootstrap.inc'
expect_fail "helper target traversal is rejected" "$fixture"

fixture="$(make_fixture backslash-helper-target "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/bootstrap.inc 'printf "benign\\n"'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'bash tools\bootstrap.inc'
expect_fail "helper target backslash spelling is rejected" "$fixture"

outside_helper="$TMP_ROOT/outside-execution-target"
printf '%s\n' 'printf "outside\\n"' > "$outside_helper"
for mode in direct bash; do
	fixture="$(make_fixture "symlink-helper-$mode" "$FULL_ACTION_REF")"
	mkdir -p "$fixture/scripts"
	ln -s "$outside_helper" "$fixture/scripts/helper"
	if [[ "$mode" == direct ]]; then
		command='scripts/helper'
	else
		command='bash scripts/helper'
	fi
	add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" "$command"
	expect_fail "$mode helper symlink escape is rejected" "$fixture"
done

fixture="$(make_fixture symlink-helper-component "$FULL_ACTION_REF")"
mkdir -p "$fixture/real-tools"
write_repository_file "$fixture" real-tools/helper.inc 'printf "benign\\n"'
ln -s real-tools "$fixture/tools"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'bash tools/helper.inc'
expect_fail "helper target with a symlink path component is rejected" "$fixture"

fixture="$(make_fixture workflow-standalone-mutable-step "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/standalone.yml \
	'jobs:' \
	'  standalone:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      -' \
	'        uses: actions/checkout@v7'
expect_fail "workflow standalone step indicator with mutable Action" "$fixture"

fixture="$(make_fixture workflow-standalone-pinned-step "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/standalone.yml \
	'jobs:' \
	'  standalone:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      -' \
	"        uses: actions/checkout@$FULL_ACTION_SHA # v7.0.1"
expect_fail "workflow standalone step indicator with pinned Action" "$fixture"

fixture="$(make_fixture workflow-commented-standalone-step "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/standalone.yml \
	'jobs:' \
	'  standalone:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - # comment' \
	'        uses: actions/checkout@v7'
expect_fail "workflow commented standalone step indicator" "$fixture"

fixture="$(make_fixture workflow-standalone-run-step "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/standalone.yml \
	'jobs:' \
	'  standalone:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      -' \
	'        run: echo unsupported'
expect_fail "workflow standalone run step indicator" "$fixture"

fixture="$(make_fixture composite-standalone-mutable-step "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/actions/standalone/action.yml \
	'name: standalone' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    -' \
	'      uses: actions/checkout@v7'
expect_fail "composite standalone step indicator with mutable Action" "$fixture"

fixture="$(make_fixture composite-standalone-pinned-step "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/actions/standalone/action.yml \
	'name: standalone' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    -' \
	"      uses: actions/checkout@$FULL_ACTION_SHA # v7.0.1"
expect_fail "composite standalone step indicator with pinned Action" "$fixture"

fixture="$(make_fixture ordinary-standalone-sequence "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/data-sequence.yml \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    strategy:' \
	'      matrix:' \
	'        include:' \
	'          -' \
	'            value: ordinary' \
	'    steps:' \
	'      - run: echo ordinary'
expect_pass "standalone sequence data outside steps" "$fixture"

fixture="$(make_fixture protected-job-if-before "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before 'if: false'
expect_fail "protected TinyGo job if false before ordinary properties" "$fixture"

fixture="$(make_fixture protected-job-if-after "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" after 'if: false'
expect_fail "protected TinyGo job if false after steps" "$fixture"

fixture="$(make_fixture protected-job-if-true "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before 'if: true'
expect_fail "protected TinyGo job if true" "$fixture"

fixture="$(make_fixture protected-job-if-expression "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before 'if: ${{ always() }}'
expect_fail "protected TinyGo job if expression" "$fixture"

fixture="$(make_fixture protected-job-continue-true "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'continue-on-error: true'
expect_fail "protected TinyGo job continue-on-error true" "$fixture"

fixture="$(make_fixture protected-job-continue-false "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" after \
	'continue-on-error: false'
expect_fail "protected TinyGo job continue-on-error false" "$fixture"

fixture="$(make_fixture protected-job-continue-expression "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'continue-on-error: ${{ matrix.experimental }}'
expect_fail "protected TinyGo job continue-on-error expression" "$fixture"

fixture="$(make_fixture protected-job-needs-scalar "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'needs: prerequisite'
expect_fail "protected TinyGo job scalar needs" "$fixture"

fixture="$(make_fixture protected-job-needs-list "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" after 'needs:' \
	'- prerequisite'
expect_fail "protected TinyGo job sequence needs" "$fixture"

fixture="$(make_fixture protected-job-self-hosted "$FULL_ACTION_REF")"
replace_fixture_runs_on "$fixture/.github/workflows/ci-core.yml" self-hosted
expect_fail "protected TinyGo job self-hosted runner" "$fixture"

fixture="$(make_fixture protected-job-runner-expression "$FULL_ACTION_REF")"
replace_fixture_runs_on "$fixture/.github/workflows/ci-core.yml" \
	'${{ matrix.runner }}'
expect_fail "protected TinyGo job runner expression" "$fixture"

fixture="$(make_fixture protected-job-quoted-runner "$FULL_ACTION_REF")"
replace_fixture_runs_on "$fixture/.github/workflows/ci-core.yml" \
	"'ubuntu-latest'"
expect_fail "protected TinyGo job quoted runner" "$fixture"

fixture="$(make_fixture protected-job-runner-flow-sequence "$FULL_ACTION_REF")"
replace_fixture_runs_on "$fixture/.github/workflows/ci-core.yml" \
	'[ubuntu-latest]'
expect_fail "protected TinyGo job flow-sequence runner" "$fixture"

fixture="$(make_fixture protected-job-runner-block-sequence "$FULL_ACTION_REF")"
replace_fixture_runs_on "$fixture/.github/workflows/ci-core.yml" '' \
	'- ubuntu-latest'
expect_fail "protected TinyGo job block-sequence runner" "$fixture"

fixture="$(make_fixture protected-job-missing-runner "$FULL_ACTION_REF")"
remove_fixture_runs_on "$fixture/.github/workflows/ci-core.yml"
expect_fail "protected TinyGo job missing runs-on" "$fixture"

fixture="$(make_fixture protected-job-duplicate-runner "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'runs-on: ubuntu-latest'
expect_fail "protected TinyGo job duplicate runs-on" "$fixture"

fixture="$(make_fixture protected-job-container-scalar "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'container: attacker/tinygo-tools:latest'
expect_fail "protected TinyGo job container scalar" "$fixture"

fixture="$(make_fixture protected-job-container-mapping "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before 'container:' \
	'image: attacker/tinygo-tools:latest'
expect_fail "protected TinyGo job container mapping" "$fixture"

fixture="$(make_fixture protected-job-container-expression "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'container: ${{ matrix.container }}'
expect_fail "protected TinyGo job container expression" "$fixture"

fixture="$(make_fixture protected-job-container-null "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" before \
	'container: null'
expect_fail "protected TinyGo job container null" "$fixture"

fixture="$(make_fixture protected-job-container-after-steps "$FULL_ACTION_REF")"
add_job_property "$fixture/.github/workflows/ci-core.yml" after \
	'container: attacker/tinygo-tools:latest'
expect_fail "protected TinyGo job container after steps" "$fixture"

fixture="$(make_fixture cross-job-tinygo-sequence "$FULL_ACTION_REF")"
split_verify_into_new_job "$fixture/.github/workflows/ci-core.yml"
expect_fail "Install and Verify TinyGo must share one containing job" "$fixture"

fixture="$(make_fixture unrelated-job-controls "$FULL_ACTION_REF")"
add_unrelated_control_jobs "$fixture/.github/workflows/ci-core.yml"
expect_pass "unrelated jobs may use if continue-on-error and needs" "$fixture"

fixture="$(make_fixture unrelated-job-runner-container "$FULL_ACTION_REF")"
add_unrelated_runner_jobs "$fixture/.github/workflows/ci-core.yml"
expect_pass "unrelated jobs may use self-hosted runners and containers" "$fixture"

fixture="$(make_fixture protected-job-matrix "$FULL_ACTION_REF")"
add_job_strategy_matrix "$fixture/.github/workflows/ci-core.yml"
expect_pass "protected TinyGo job preserves strategy and matrix" "$fixture"

fixture="$(make_fixture workflow-run-default "$FULL_ACTION_REF")"
add_workflow_run_default "$fixture/.github/workflows/ci-core.yml"
expect_pass "workflow run shell default does not invalidate protected steps" "$fixture"

fixture="$(make_fixture job-run-default "$FULL_ACTION_REF")"
add_job_run_default "$fixture/.github/workflows/ci-core.yml"
expect_pass "job run shell default does not invalidate protected steps" "$fixture"

fixture="$(make_fixture python-helper-tinygo-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.py \
	'import subprocess' \
	"base = \"https://github.com/$TINYGO_RELEASE_REPOSITORY\"" \
	"subprocess.run([\"$CURL_COMMAND_NAME\", base + \"/releases/download/v0.42.0/tinygo_0.42.0_amd64.deb\"], check=True)"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'python3 scripts/install-tinygo.py'
expect_fail "invoked Python helper with TinyGo repository provenance" "$fixture"

fixture="$(make_fixture python-helper-direct-tinygo-url "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.py \
	'import subprocess' \
	"subprocess.run([\"$CURL_COMMAND_NAME\", \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v0.42.0/tinygo_0.42.0_amd64.deb\"], check=True)"
expect_fail "Python helper with direct TinyGo release URL" "$fixture"

fixture="$(make_fixture python-helper-case-variant-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.py \
	'import subprocess' \
	"base = \"https://github.com/$TINYGO_RELEASE_REPOSITORY_CASE_VARIANT\"" \
	"subprocess.run([\"$CURL_COMMAND_NAME\", base + \"/releases/download/artifact\"], check=True)"
expect_fail "Python helper with case-variant TinyGo repository provenance" "$fixture"

fixture="$(make_fixture javascript-helper-tinygo-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.mjs \
	'import { spawnSync } from "node:child_process";' \
	"const base = \"https://github.com/$TINYGO_RELEASE_REPOSITORY\";" \
	"spawnSync(\"$CURL_COMMAND_NAME\", [base + \"/releases/download/v0.42.0/tinygo_0.42.0_amd64.deb\"]);"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'node scripts/install-tinygo.mjs'
expect_fail "invoked JavaScript helper with TinyGo repository provenance" "$fixture"

fixture="$(make_fixture workflow-provided-tinygo-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.py \
	'import os' \
	'import subprocess' \
	"subprocess.run([\"$CURL_COMMAND_NAME\", os.environ[\"TINYGO_BASE\"] + \"/releases/download/artifact\"]);"
printf '%s\n' '' '  delegated-python:' '    runs-on: ubuntu-latest' \
	'    env:' \
	"      TINYGO_BASE: https://github.com/$TINYGO_RELEASE_REPOSITORY" \
	'    steps:' \
	'      - run: python3 scripts/install-tinygo.py' \
	>> "$fixture/.github/workflows/ci-core.yml"
expect_fail "workflow cannot supply TinyGo provenance to a non-shell helper" "$fixture"

fixture="$(make_fixture benign-python-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/benign.py \
	'print("benign helper")'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'python3 scripts/benign.py'
expect_pass "invoked Python helper without TinyGo provenance" "$fixture"

fixture="$(make_fixture loopback-javascript-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/loopback.mjs \
	'const target = "http://127.0.0.1:8080/";' \
	'console.log(target);'
expect_pass "JavaScript helper with loopback-only data" "$fixture"

fixture="$(make_fixture documentation-tinygo-link "$FULL_ACTION_REF")"
write_repository_file "$fixture" docs/tinygo-reference.md \
	"Upstream: https://github.com/$TINYGO_RELEASE_REPOSITORY/releases/tag/v0.42.0"
expect_pass "documentation may reference the TinyGo repository" "$fixture"

fixture="$(make_fixture uninvoked-python-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/uninvoked.py \
	"base = \"https://github.com/$TINYGO_RELEASE_REPOSITORY\""
expect_fail "uninvoked executable-source helper with TinyGo provenance" "$fixture"

fixture="$(make_fixture outside-helper-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/install-tinygo.py \
	"base = \"https://github.com/$TINYGO_RELEASE_REPOSITORY\""
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'python3 tools/install-tinygo.py'
expect_fail "invoked helper outside standard CI source roots" "$fixture"

fixture="$(make_fixture outside-go-helper-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/install-tinygo.go \
	'package main' \
	'import "fmt"' \
	"func main() { fmt.Println(\"https://github.com/$TINYGO_RELEASE_REPOSITORY\") }"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'go run tools/install-tinygo.go'
expect_fail "directly referenced Go helper with TinyGo provenance" "$fixture"

fixture="$(make_fixture traversed-go-helper-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" outside/install-tinygo.go \
	'package main' \
	'import "fmt"' \
	"func main() { fmt.Println(\"https://github.com/$TINYGO_RELEASE_REPOSITORY\") }"
parent_path_component=..
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	"go run tools/$parent_path_component/outside/install-tinygo.go"
expect_fail "traversed Go helper path cannot evade TinyGo provenance" "$fixture"

fixture="$(make_fixture extensionless-helper-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo \
	'#!/usr/bin/env python3' \
	"base = \"https://github.com/$TINYGO_RELEASE_REPOSITORY\""
chmod +x "$fixture/scripts/install-tinygo"
expect_fail "extensionless executable helper with TinyGo provenance" "$fixture"

fixture="$(make_fixture checker-short-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/ci-supply-chain-check.sh \
	'#!/usr/bin/env bash' \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
expect_fail "checker source is not exempt from TinyGo provenance policy" "$fixture"

fixture="$(make_fixture harness-short-provenance "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/scripts/ci-supply-chain-check.tests.sh \
	'#!/usr/bin/env bash' \
	"base=https://github.com/$TINYGO_RELEASE_REPOSITORY"
expect_fail "test harness is not exempt from TinyGo provenance policy" "$fixture"

fixture="$(make_fixture checker-active-downloader "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/ci-supply-chain-check.sh \
	'#!/usr/bin/env bash' \
	"$CURL_COMMAND_NAME https://example.invalid/file"
expect_fail "checker source is not exempt from downloader policy" "$fixture"

fixture="$(make_fixture harness-active-downloader "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/scripts/ci-supply-chain-check.tests.sh \
	'#!/usr/bin/env bash' \
	"$CURL_COMMAND_NAME https://example.invalid/file"
expect_fail "test harness is not exempt from downloader policy" "$fixture"

expect_pass "protected install followed by separate TinyGo version smoke" \
	"$(make_fixture split-version-smoke "$FULL_ACTION_REF")"

fixture="$(make_fixture missing-verify-shell "$FULL_ACTION_REF")"
remove_verify_shell "$fixture/.github/workflows/ci-core.yml"
expect_fail "Verify TinyGo without explicit shell" "$fixture"

fixture="$(make_fixture ambient-verify-shell "$FULL_ACTION_REF")"
replace_verify_shell "$fixture/.github/workflows/ci-core.yml" bash
expect_fail "Verify TinyGo with PATH-resolved Bash shell" "$fixture"

fixture="$(make_fixture incomplete-verify-shell "$FULL_ACTION_REF")"
replace_verify_shell "$fixture/.github/workflows/ci-core.yml" \
	'/bin/bash -e {0}'
expect_fail "Verify TinyGo with incomplete Bash shell" "$fixture"

fixture="$(make_fixture sanitized-verify-shell "$FULL_ACTION_REF")"
replace_verify_shell "$fixture/.github/workflows/ci-core.yml" "$CLEAN_SHELL"
expect_fail "Verify TinyGo with install boundary shell" "$fixture"

expect_fail "ambient PATH TinyGo version smoke" \
	"$(make_fixture ambient-version-smoke "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted protected ambient)"
fixture="$(make_fixture path-poisoned-version-smoke "$FULL_ACTION_REF")"
add_tinygo_path_poisoning "$fixture/.github/workflows/ci-core.yml"
expect_pass "PATH poisoning cannot redirect package-owned TinyGo smoke" "$fixture"
test_verify_shell_boundary
expect_fail "TinyGo version smoke inside sanitized install" \
	"$(make_fixture in-boundary-version-smoke "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted protected inside)"
expect_fail "protected install without TinyGo version smoke" \
	"$(make_fixture missing-version-smoke "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted protected missing)"
expect_fail "TinyGo version smoke before protected install" \
	"$(make_fixture early-version-smoke "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted protected before-install)"
expect_fail "malformed TinyGo version smoke" \
	"$(make_fixture malformed-version-smoke "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted protected malformed)"
expect_fail "sanitized TinyGo version smoke" \
	"$(make_fixture sanitized-version-smoke "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted protected custom-shell)"

for position in before after; do
	fixture="$(make_fixture "verify-continue-on-error-$position" "$FULL_ACTION_REF")"
	add_verify_property "$fixture/.github/workflows/ci-core.yml" "$position" \
		'continue-on-error: true'
	expect_fail "Verify TinyGo continue-on-error $position run" "$fixture"

	fixture="$(make_fixture "verify-if-$position" "$FULL_ACTION_REF")"
	add_verify_property "$fixture/.github/workflows/ci-core.yml" "$position" 'if: false'
	expect_fail "Verify TinyGo if false $position run" "$fixture"

	fixture="$(make_fixture "verify-env-$position" "$FULL_ACTION_REF")"
	add_verify_property "$fixture/.github/workflows/ci-core.yml" "$position" \
		'env:' 'TINYGO_FAKE: attacker'
	expect_fail "Verify TinyGo step env $position run" "$fixture"

	fixture="$(make_fixture "install-continue-on-error-$position" "$FULL_ACTION_REF")"
	add_install_property "$fixture/.github/workflows/ci-core.yml" "$position" \
		'continue-on-error: true'
	expect_fail "Install TinyGo continue-on-error $position run" "$fixture"

	fixture="$(make_fixture "install-if-$position" "$FULL_ACTION_REF")"
	add_install_property "$fixture/.github/workflows/ci-core.yml" "$position" 'if: false'
	expect_fail "Install TinyGo if false $position run" "$fixture"

	fixture="$(make_fixture "install-env-$position" "$FULL_ACTION_REF")"
	add_install_property "$fixture/.github/workflows/ci-core.yml" "$position" \
		'env:' 'TINYGO_FAKE: attacker'
	expect_fail "Install TinyGo step env $position run" "$fixture"
done

fixture="$(make_fixture install-unexpected-property "$FULL_ACTION_REF")"
add_install_property "$fixture/.github/workflows/ci-core.yml" after \
	'timeout-minutes: 1'
expect_fail "Install TinyGo unexpected step property" "$fixture"

fixture="$(make_fixture verify-duplicate-run "$FULL_ACTION_REF")"
add_verify_property "$fixture/.github/workflows/ci-core.yml" after \
	'run: /usr/local/bin/tinygo version'
expect_fail "Verify TinyGo duplicate run property" "$fixture"

fixture="$(make_fixture install-duplicate-shell "$FULL_ACTION_REF")"
add_install_property "$fixture/.github/workflows/ci-core.yml" after \
	"shell: $CLEAN_SHELL"
expect_fail "Install TinyGo duplicate shell property" "$fixture"

fixture="$(make_fixture reordered-verify-properties "$FULL_ACTION_REF")"
reorder_verify_properties "$fixture/.github/workflows/ci-core.yml"
expect_pass "Verify TinyGo valid properties in reordered mapping" "$fixture"

fixture="$(make_fixture reordered-install-properties "$FULL_ACTION_REF")"
move_install_shell_after_run "$fixture/.github/workflows/ci-core.yml"
expect_pass "Install TinyGo valid properties in reordered mapping" "$fixture"

expect_pass "full remote Action SHA with version annotation" \
	"$(make_fixture full-sha "$FULL_ACTION_REF")"
expect_fail "mutable major Action tag" \
	"$(make_fixture mutable-tag 'actions/checkout@v7')"
expect_fail "mutable branch Action ref" \
	"$(make_fixture mutable-branch 'actions/checkout@main')"
expect_fail "short Action SHA" \
	"$(make_fixture short-sha 'actions/checkout@3d3c42e5aac5')"
expect_pass "external reusable workflow with annotated full SHA" \
	"$(make_fixture reusable-workflow "$FULL_REUSABLE_REF")"
expect_fail "full remote Action SHA without version annotation" \
	"$(make_fixture missing-version-annotation "actions/checkout@$FULL_ACTION_SHA")"
expect_fail "remote Action SHA with prose annotation" \
	"$(make_fixture prose-version-annotation "actions/checkout@$FULL_ACTION_SHA # pinned")"
expect_fail "remote Action SHA with major-only annotation" \
	"$(make_fixture major-version-annotation "actions/checkout@$FULL_ACTION_SHA # v1")"
expect_fail "remote Action SHA with two-component annotation" \
	"$(make_fixture minor-version-annotation "actions/checkout@$FULL_ACTION_SHA # v1.2")"

fixture="$(make_fixture job-env-uses-data "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/data.yml \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    env:' \
	'      uses: harmless-data' \
	'    steps:' \
	'      - run: echo ok'
expect_pass "job env key named uses is ordinary data" "$fixture"

fixture="$(make_fixture step-env-uses-data "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/data.yml \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	'      - run: echo ok' \
	'        env:' \
	'          uses: harmless-data'
expect_pass "step env key named uses is ordinary data" "$fixture"

fixture="$(make_fixture with-uses-data "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/data.yml \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    steps:' \
	"      - uses: actions/checkout@$FULL_ACTION_SHA # v7.0.1" \
	'        with:' \
	'          uses: harmless-data'
expect_pass "with key named uses is ordinary data" "$fixture"

fixture="$(make_fixture flow-env-uses-data "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/data.yml \
	'jobs:' \
	'  data:' \
	'    runs-on: ubuntu-latest' \
	'    env: { uses: harmless-data }' \
	'    steps:' \
	'      - run: echo ok'
expect_pass "flow env data named uses is not an Action reference" "$fixture"

fixture="$(make_fixture action-input-output-uses-data "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/actions/data/action.yml \
	'name: data' \
	'inputs:' \
	'  uses:' \
	'    default: harmless-data' \
	'outputs:' \
	'  uses:' \
	'    value: harmless-data' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    - run: echo ok' \
	'      shell: bash'
expect_pass "Action input and output keys named uses are ordinary data" "$fixture"

fixture="$(make_fixture job-reusable-pinned "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/reusable-job.yml \
	'jobs:' \
	'  call:' \
	"    uses: $FULL_REUSABLE_REF"
expect_pass "job-level reusable workflow uses is validated" "$fixture"

fixture="$(make_fixture job-reusable-mutable "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/workflows/reusable-job.yml \
	'jobs:' \
	'  call:' \
	'    uses: owner/repo/.github/workflows/build.yml@main'
expect_fail "mutable job-level reusable workflow uses" "$fixture"

fixture="$(make_fixture composite-action-pinned "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/actions/direct/action.yml \
	'name: direct' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	"    - uses: owner/remote@$FULL_ACTION_SHA # v1.2.3"
expect_pass "composite Action step uses is validated" "$fixture"

fixture="$(make_fixture composite-action-mutable "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/actions/direct/action.yml \
	'name: direct' \
	'runs:' \
	'  using: composite' \
	'  steps:' \
	'    - uses: owner/remote@main'
expect_fail "mutable composite Action step uses" "$fixture"

expect_fail "old unqualified TinyGo checksum verifier" \
	"$(make_fixture old-unqualified-verifier "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal unqualified)"

fixture="$(make_fixture path-poisoned-unqualified-verifier "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal unqualified)"
add_path_poisoning "$fixture/.github/workflows/ci-core.yml"
expect_fail "PATH-poisoned workflow with unqualified checksum verifier" "$fixture"

fixture="$(make_fixture path-poisoned-trusted-verifier "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted)"
add_path_poisoning "$fixture/.github/workflows/ci-core.yml"
expect_pass "PATH-poisoned workflow with trusted checksum verifier" "$fixture"

fixture="$(make_fixture bash-env-trusted-verifier "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted)"
add_bash_env_shadowing "$fixture/.github/workflows/ci-core.yml"
expect_pass "BASH_ENV-shadowed workflow with trusted checksum verifier" "$fixture"

dynamic_base="https://github.com/${TINYGO_RELEASE_REPOSITORY}"
fixture="$(make_fixture dynamic-workflow-download "$FULL_ACTION_REF")"
printf '%s\n' '' '  dynamic-download:' '    steps:' '      - run: |' \
	"          base=$dynamic_base" \
	"          $CURL_COMMAND_NAME -fsSL -o /tmp/extra.deb \"\$base/releases/download/v0.42.0/tinygo_0.42.0_amd64.deb\"" \
	>> "$fixture/.github/workflows/ci-core.yml"
expect_fail "runtime-composed TinyGo download in workflow" "$fixture"

fixture="$(make_fixture dynamic-helper-download "$FULL_ACTION_REF")"
dynamic_helper="$(printf '%s\n' "base=$dynamic_base" \
	"$CURL_COMMAND_NAME -fsSL -o /tmp/extra.deb \"\$base/releases/download/v0.42.0/tinygo_0.42.0_amd64.deb\"")"
write_repository_file "$fixture" scripts/install-tinygo.sh "$dynamic_helper"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'scripts/install-tinygo.sh'
expect_fail "runtime-composed TinyGo download in invoked helper" "$fixture"

fixture="$(make_fixture continued-dynamic-download "$FULL_ACTION_REF")"
printf '%s\n' '' '  continued-download:' '    steps:' '      - run: |' \
	"          base=$dynamic_base" \
	'          url="$base/releases/download/v0.42.0/tinygo_0.42.0_amd64.deb"' \
	"          $CURL_COMMAND_NAME \\" '            "$url"' \
	>> "$fixture/.github/workflows/ci-core.yml"
expect_fail "continued runtime-composed TinyGo download" "$fixture"

fixture="$(make_fixture dynamic-url-download "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "$CURL_COMMAND_NAME \"\$url\""
expect_fail 'downloader with runtime URL' "$fixture"

fixture="$(make_fixture absolute-dynamic-url-download "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "/usr/bin/$CURL_COMMAND_NAME \"\$url\""
expect_fail 'absolute downloader with runtime URL' "$fixture"

fixture="$(make_fixture alternate-absolute-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "/usr/local/bin/$CURL_COMMAND_NAME \"\$url\""
expect_fail 'alternate absolute downloader path with runtime URL' "$fixture"

fixture="$(make_fixture relative-path-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "./$CURL_COMMAND_NAME \"\$url\""
expect_fail 'relative downloader path with runtime URL' "$fixture"

fixture="$(make_fixture command-dynamic-url-download "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "command $CURL_COMMAND_NAME \"\$url\""
expect_fail 'command-prefixed downloader with runtime URL' "$fixture"

fixture="$(make_fixture conditional-dynamic-url-download "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "if $CURL_COMMAND_NAME \"\$url\"; then printf success; fi"
expect_fail 'conditional downloader with runtime URL' "$fixture"

fixture="$(make_fixture substitution-dynamic-url-download "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/dynamic-download.sh \
	'url=https://example.com/file' "result=\"\$($CURL_COMMAND_NAME \"\$url\")\""
expect_fail 'command-substitution downloader with runtime URL' "$fixture"

fixture="$(make_fixture literal-external-download "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/external-download.sh \
	"$CURL_COMMAND_NAME https://example.com/file"
expect_fail 'literal external downloader' "$fixture"

fixture="$(make_fixture external-alternate-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/external-download.sh \
	"$WGET_COMMAND_NAME https://example.com/file"
expect_fail 'literal external alternate downloader' "$fixture"

fixture="$(make_fixture env-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	'url=https://example.com/file' "env $CURL_COMMAND_NAME \"\$url\""
expect_fail 'env-wrapped downloader' "$fixture"

fixture="$(make_fixture absolute-env-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	'url=https://example.com/file' "/usr/bin/env $CURL_COMMAND_NAME \"\$url\""
expect_fail 'absolute env-wrapped downloader' "$fixture"

fixture="$(make_fixture clean-env-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	'url=https://example.com/file' "env -i $CURL_COMMAND_NAME \"\$url\""
expect_fail 'clean-env-wrapped downloader' "$fixture"

fixture="$(make_fixture assigned-env-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	'url=https://example.com/file' "env FOO=bar $CURL_COMMAND_NAME \"\$url\""
expect_fail 'assigned-env-wrapped downloader' "$fixture"

for wrapper in exec sudo nohup 'timeout 10' 'nice -n 5' 'stdbuf -oL' xargs; do
	fixture="$(make_fixture "${wrapper%% *}-wrapped-downloader" "$FULL_ACTION_REF")"
	write_shell_source "$fixture" scripts/wrapped-download.sh \
		'url=https://example.com/file' "$wrapper $CURL_COMMAND_NAME \"\$url\""
	expect_fail "$wrapper wrapped downloader" "$fixture"
done

fixture="$(make_fixture bash-nested-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	"bash -c '$CURL_COMMAND_NAME \"\$url\"'"
expect_fail 'Bash-nested downloader' "$fixture"

fixture="$(make_fixture sh-nested-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	"sh -c '$WGET_COMMAND_NAME \"\$url\"'"
expect_fail 'POSIX-shell-nested downloader' "$fixture"

fixture="$(make_fixture variable-selected-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	"downloader=$CURL_COMMAND_NAME" \
	'url=https://example.com/file' \
	'"$downloader" "$url"'
expect_fail 'variable-selected downloader identity' "$fixture"

fixture="$(make_fixture evaluated-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	"cmd='$CURL_COMMAND_NAME https://example.com/file'" \
	'eval "$cmd"'
expect_fail 'evaluated downloader string' "$fixture"

fixture="$(make_fixture aliased-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	"alias fetch='$CURL_COMMAND_NAME'" \
	'fetch https://example.com/file'
expect_fail 'aliased downloader identity' "$fixture"

fixture="$(make_fixture function-downloader "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/wrapped-download.sh \
	"fetch() { $CURL_COMMAND_NAME \"\$url\"; }" \
	'fetch'
expect_fail 'function-contained downloader identity' "$fixture"

fixture="$(make_fixture simple-loopback-probe "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/loopback-probe.sh \
	"$CURL_COMMAND_NAME -fsS \"http://127.0.0.1:\$port/\" >/dev/null 2>&1"
expect_pass 'literal-rooted loopback port probe' "$fixture"

fixture="$(make_fixture path-loopback-probe "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/loopback-probe.sh \
	"$CURL_COMMAND_NAME -fsS \"http://127.0.0.1:\$port/\$wasm_path\" >/dev/null 2>&1"
expect_pass 'literal-rooted loopback path probe' "$fixture"

fixture="$(make_fixture substitution-loopback-probe "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/loopback-probe.sh \
	"headers=\"\$($CURL_COMMAND_NAME -fsSI \"http://127.0.0.1:\$port/\$wasm_path?smoke=\$(date +%s%N)\" || true)\""
expect_pass 'literal-rooted loopback command-substitution probe' "$fixture"

fixture="$(make_fixture loopback-tool-requirement "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/browser-smoke.sh \
	"require_command $CURL_COMMAND_NAME \"Install $CURL_COMMAND_NAME for smoke server readiness checks.\""
expect_pass 'exact loopback tool requirement' "$fixture"

fixture="$(make_fixture loopback-tool-package "$FULL_ACTION_REF")"
printf '      - run: sudo apt-get install -y brotli zstd %s\n' \
	"$CURL_COMMAND_NAME" >> "$fixture/.github/workflows/ci-browser-smoke.yml"
expect_pass 'exact loopback tool package install' "$fixture"

fixture="$(make_fixture nested-external-loopback-download "$FULL_ACTION_REF")"
nested_loopback_command="$CURL_COMMAND_NAME -fsS \"http://127.0.0.1:\$port/\$($CURL_COMMAND_NAME https://example.com/file)\" >/dev/null 2>&1"
write_shell_source "$fixture" scripts/loopback-probe.sh "$nested_loopback_command"
expect_fail 'external downloader nested in loopback probe' "$fixture"

fixture="$(make_fixture multiple-downloaders "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/external-download.sh \
	"$CURL_COMMAND_NAME https://example.com/file || $WGET_COMMAND_NAME https://example.com/fallback"
expect_fail 'multiple external downloaders' "$fixture"

fixture="$(make_fixture benign-downloader-string "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/benign.sh \
	"message='$CURL_COMMAND_NAME \"\$url\" is not executed'" \
	'printf "%s\\n" "$message"'
expect_fail 'downloader-shaped inert string is fail-closed' "$fixture"

fixture="$(make_fixture benign-url-variable "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/benign.sh \
	'url=https://example.com/file' 'printf "%s\\n" "$url"'
expect_pass 'benign URL assignment without downloader' "$fixture"

fixture="$(make_fixture downloader-comments "$FULL_ACTION_REF")"
write_shell_source "$fixture" scripts/benign.sh \
	"# $CURL_COMMAND_NAME and $WGET_COMMAND_NAME are comments only" \
	"printf 'inline comment ignored\\n' # $CURL_COMMAND_NAME and $WGET_COMMAND_NAME" \
	'printf "comments ignored\\n"'
expect_pass 'comments may mention downloader tools' "$fixture"

expect_fail "old TinyGo sequence without sanitized shell" \
	"$(make_fixture old-install-boundary "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted legacy)"
expect_fail "sanitized shell without protected staging" \
	"$(make_fixture no-protected-staging "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted no-staging)"
expect_fail "protected staging with checksum of original candidate" \
	"$(make_fixture checksum-original-candidate "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted checksum-original)"
expect_fail "protected checksum with install of original candidate" \
	"$(make_fixture install-original-candidate "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted install-original)"
expect_fail "protected boundary without cleanup" \
	"$(make_fixture missing-protected-cleanup "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted missing-cleanup)"
expect_fail "mutable TinyGo custom shell" \
	"$(make_fixture mutable-install-shell "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted mutable-shell)"

fixture="$(make_fixture bash-env-sudo-interposition-old "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes literal trusted legacy)"
add_bash_env_sudo_shadowing "$fixture/.github/workflows/ci-core.yml"
expect_fail "BASH_ENV sudo override with old install boundary" "$fixture"

fixture="$(make_fixture bash-env-sudo-interposition-protected "$FULL_ACTION_REF")"
add_bash_env_sudo_shadowing "$fixture/.github/workflows/ci-core.yml"
expect_pass "BASH_ENV sudo override cannot enter protected install boundary" "$fixture"

test_ambient_sudo_interposition
test_clean_shell_boundary

for action_form in \
	'"uses": actions/checkout@v7' \
	"'uses': actions/checkout@v7" \
	'{ uses: actions/checkout@v7 }' \
	"\"uses\": actions/checkout@$FULL_ACTION_SHA # v7.0.1" \
	"{ uses: actions/checkout@$FULL_ACTION_SHA } # v7.0.1" \
	'"u\u0073es": actions/checkout@v7' \
	'? uses'; do
	fixture="$(make_fixture alternate-action "$FULL_ACTION_REF")"
	printf 'jobs:\n  alternate:\n    steps:\n      - %s\n' "$action_form" \
		> "$fixture/.github/workflows/alternate.yml"
	expect_fail "unsupported Action key: $action_form" "$fixture"
done

fixture="$(make_fixture commented-action "$FULL_ACTION_REF")"
printf '# - "uses": actions/checkout@v7\n# - { uses: actions/checkout@v7 }\n' \
	> "$fixture/.github/workflows/comments.yml"
expect_pass "commented alternate Action keys" "$fixture"

fixture="$(make_fixture scalar-content "$FULL_ACTION_REF")"
printf '%s\n' 'env:' '  VERSION: value' 'jobs:' '  quoted:' '    strategy:' '      matrix:' '        go:' "          - '1.26.6'" \
	'    steps:' '      - run: |' '          echo "uses: actions/example@v7"' \
	> "$fixture/.github/workflows/scalars.yml"
expect_pass "quoted scalar list and literal script content" "$fixture"

for scalar in '|' '|-' '|+' '>' '>-' '>+' '|2' '|2-' '|-2' '>2+' '>+2'; do
	fixture="$(make_fixture "block-scalar-$tests_run" "$FULL_ACTION_REF")"
	write_block_scalar_workflow "$fixture/.github/workflows/scalars.yml" "$scalar"
	expect_pass "block scalar $scalar with mapping-like body content" "$fixture"
done

for scalar in '|0' '|22' '|+-' '>-+'; do
	fixture="$(make_fixture "invalid-block-scalar-$tests_run" "$FULL_ACTION_REF")"
	write_block_scalar_workflow "$fixture/.github/workflows/scalars.yml" "$scalar"
	expect_fail "invalid block scalar indicator $scalar" "$fixture"
done

printf '%s\n' 'jobs:' '  named:' '    steps:' '      - name: |' '          multiline name' \
	'        uses: actions/checkout@v7' > "$fixture/.github/workflows/scalars.yml"
expect_fail "Action following a block-scalar step name" "$fixture"

printf '%s\n' 'jobs:' '  named:' '    steps:' '      - name: |2' '          multiline name' \
	'        uses: actions/checkout@v7' > "$fixture/.github/workflows/scalars.yml"
expect_fail "Action following an explicit-indent block-scalar step name" "$fixture"

printf '%s\n' 'jobs:' '  scalar:' '    steps:' '      - run: >+2' '          scalar body' \
	'        uses: actions/checkout@v7' > "$fixture/.github/workflows/scalars.yml"
expect_fail "Action following a folded explicit-indent block scalar" "$fixture"

printf '%s\n' 'jobs:' '  named:' '    steps:' '      -   name: |' '            multiline name' \
	'          uses: actions/checkout@v7' > "$fixture/.github/workflows/scalars.yml"
expect_fail "Action following a spaced block-scalar step name" "$fixture"

fixture="$(make_fixture 'space in repository path' "$FULL_ACTION_REF")"
expect_pass "repository path containing spaces" "$fixture"
printf '%s\n' 'jobs:' '  newline:' '    steps:' \
	'      - uses: actions/checkout@v7' \
	> "$fixture/.github/workflows/"$'line\nbreak.yml'
expect_fail "workflow filename containing a newline" "$fixture"

fixture="$(make_fixture local-action-yml './.github/actions/local')"
write_local_action "$fixture" './.github/actions/local' action.yml
expect_pass "local Action with action.yml" "$fixture"

fixture="$(make_fixture local-action-yaml './.github/actions/local')"
write_local_action "$fixture" './.github/actions/local' action.yaml
expect_pass "local Action with action.yaml" "$fixture"

fixture="$(make_fixture duplicate-local-metadata './.github/actions/local')"
write_local_action "$fixture" './.github/actions/local' action.yml
write_local_action "$fixture" './.github/actions/local' action.yaml
expect_fail "local Action with duplicate metadata entrypoints" "$fixture"

fixture="$(make_fixture missing-local-metadata './.github/actions/local')"
mkdir -p "$fixture/.github/actions/local"
expect_fail "local Action missing metadata" "$fixture"

fixture="$(make_fixture symlinked-local-action './.github/actions/local')"
write_local_action "$fixture" './outside-action' action.yml 'owner/remote@v1'
mkdir -p "$fixture/.github/actions"
ln -s ../../outside-action "$fixture/.github/actions/local"
expect_fail "symlinked local Action outside scanned tree" "$fixture"

fixture="$(make_fixture symlinked-local-metadata './.github/actions/local')"
write_local_action "$fixture" './outside-action' action.yml 'owner/remote@v1'
mkdir -p "$fixture/.github/actions/local"
ln -s ../../../outside-action/action.yml "$fixture/.github/actions/local/action.yml"
expect_fail "symlinked local Action metadata outside scanned tree" "$fixture"

fixture="$(make_fixture nested-mutable-action './.github/actions/local')"
write_local_action "$fixture" './.github/actions/local' action.yml 'owner/remote@v1'
expect_fail "local Action with mutable nested remote Action" "$fixture"

fixture="$(make_fixture nested-pinned-action './.github/actions/local')"
write_local_action "$fixture" './.github/actions/local' action.yml \
	"owner/remote@$FULL_ACTION_SHA # v1.2.3"
expect_pass "local Action with pinned annotated nested Action" "$fixture"

fixture="$(make_fixture local-reusable './.github/workflows/reusable.yml')"
write_local_workflow "$fixture" './.github/workflows/reusable.yml'
expect_pass "local reusable workflow" "$fixture"

fixture="$(make_fixture local-reusable-mutable './.github/workflows/reusable.yml')"
write_local_workflow "$fixture" './.github/workflows/reusable.yml' 'owner/remote@main'
expect_fail "local reusable workflow with mutable nested Action" "$fixture"

fixture="$(make_fixture local-reusable-pinned './.github/workflows/reusable.yml')"
write_local_workflow "$fixture" './.github/workflows/reusable.yml' \
	"owner/remote@$FULL_ACTION_SHA # v1.2.3"
expect_pass "local reusable workflow with pinned annotated nested Action" "$fixture"

fixture="$(make_fixture outside-local-action './custom-action')"
write_local_action "$fixture" './custom-action' action.yml 'owner/remote@v1'
expect_fail "local Action outside scanned roots" "$fixture"

expect_fail "local Action path traversal" \
	"$(make_fixture local-action-traversal './.github/actions/../../custom-action')"
expect_fail "backslash local Action path" \
	"$(make_fixture local-action-backslash '.\.github\actions\local')"

extra_download="$CURL_COMMAND_NAME -fsSL -o /tmp/extra.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v0.42.0/tinygo_0.42.0_amd64.deb\""
for placement in before after same-line canonical hash-prefix unexpected; do
	fixture="$(make_fixture extra-download "$FULL_ACTION_REF")"
	file="$fixture/.github/workflows/ci-core.yml"
	case "$placement" in
		before)
			awk -v command="$extra_download" '
				/      - name: Install TinyGo/ { print "      - run: " command }
				{ print }
			' "$file" > "$TMP_ROOT/extra.yml"
			mv "$TMP_ROOT/extra.yml" "$file"
			;;
		after) printf '      - run: %s\n' "$extra_download" >> "$file" ;;
		same-line) printf '      - run: %s; %s\n' "$extra_download" "$extra_download" >> "$file" ;;
		canonical)
			command="$(sed -n "/\\/usr\\/bin\\/$CURL_COMMAND_NAME -fsSL/p" "$file")"
			printf '%s\n' "$command" >> "$file"
			;;
		hash-prefix) printf '      - run: printf "# data"; %s\n' "$extra_download" >> "$file" ;;
		unexpected)
			printf 'jobs:\n  extra:\n    steps:\n      - run: %s\n' "$extra_download" \
				> "$fixture/.github/workflows/extra.yml"
			;;
	esac
	expect_fail "additional TinyGo download: $placement" "$fixture"
done

fixture="$(make_fixture delegated-tinygo-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.sh "$extra_download"
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'scripts/install-tinygo.sh'
expect_fail "invoked repository helper with direct TinyGo download" "$fixture"

fixture="$(make_fixture uninvoked-tinygo-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/install-tinygo.sh "$extra_download"
expect_fail "uninvoked repository helper with direct TinyGo download" "$fixture"

fixture="$(make_fixture non-shell-tinygo-source "$FULL_ACTION_REF")"
write_repository_file "$fixture" tools/install.py "$extra_download"
expect_fail "non-shell repository source with direct TinyGo download" "$fixture"

fixture="$(make_fixture benign-invoked-helper "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/helper.sh 'printf "benign helper\\n"'
add_workflow_script_invocation "$fixture/.github/workflows/ci-core.yml" \
	'scripts/helper.sh'
expect_pass "invoked benign repository helper" "$fixture"

fixture="$(make_fixture executable-checker-direct-download "$FULL_ACTION_REF")"
write_repository_file "$fixture" scripts/ci-supply-chain-check.sh "$extra_download"
expect_fail "executable checker script with direct TinyGo download" "$fixture"

fixture="$(make_fixture executable-test-harness-direct-download "$FULL_ACTION_REF")"
write_repository_file "$fixture" .github/scripts/ci-supply-chain-check.tests.sh \
	"$extra_download"
expect_fail "executable test harness with direct TinyGo download" "$fixture"

if grep -Fq -- "$TINYGO_RELEASE_NAMESPACE" "$CHECKER"; then
	printf 'not ok - checker source contains raw TinyGo release namespace\n' >&2
	exit 1
fi
printf 'ok - checker source contains no raw TinyGo release namespace\n'

if grep -Fq -- "$TINYGO_RELEASE_NAMESPACE" "$BASH_SOURCE"; then
	printf 'not ok - test harness source contains raw TinyGo release namespace\n' >&2
	exit 1
fi
printf 'ok - test harness source contains no raw TinyGo release namespace\n'

if grep -Fq -- "$TINYGO_RELEASE_REPOSITORY" "$CHECKER"; then
	printf 'not ok - checker source contains raw TinyGo repository identity\n' >&2
	exit 1
fi
printf 'ok - checker source contains no raw TinyGo repository identity\n'

if grep -Fq -- "$TINYGO_RELEASE_REPOSITORY" "$BASH_SOURCE"; then
	printf 'not ok - test harness source contains raw TinyGo repository identity\n' >&2
	exit 1
fi
printf 'ok - test harness source contains no raw TinyGo repository identity\n'

runtime_namespace="$TINYGO_RELEASE_NAMESPACE"
expected_runtime_namespace="${TINYGO_RELEASE_REPOSITORY}/releases/download/"
if [[ "$runtime_namespace" != "$expected_runtime_namespace" ]]; then
	printf 'not ok - runtime TinyGo release namespace construction\n' >&2
	exit 1
fi
printf 'ok - runtime TinyGo release namespace construction\n'

expect_fail "missing TinyGo checksum" \
	"$(make_fixture missing-checksum "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install no)"
expect_fail "wrong TinyGo checksum" \
	"$(make_fixture wrong-checksum "$FULL_ACTION_REF" "$(printf '0%.0s' {1..64})")"
expect_pass "verified TinyGo install contract" \
	"$(make_fixture verified-tinygo "$FULL_ACTION_REF")"
expect_fail "runtime-variable TinyGo download URL" \
	"$(make_fixture variable-tinygo-url "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes variable-url)"
expect_fail "runtime-variable TinyGo checksum input" \
	"$(make_fixture variable-tinygo-digest "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes variable-digest)"

fixture="$(make_fixture runtime-trust-root-override "$FULL_ACTION_REF" "$EXPECTED_SHA" before-install yes variable-both)"
add_runtime_override "$fixture/.github/workflows/ci-core.yml"
expect_fail "runtime-variable TinyGo trust roots after GITHUB_ENV override" "$fixture"

fixture="$(make_fixture literal-trust-root-override "$FULL_ACTION_REF")"
add_runtime_override "$fixture/.github/workflows/ci-core.yml"
expect_pass "GITHUB_ENV override cannot alter literal TinyGo trust roots" "$fixture"

expect_fail "TinyGo install before verification" \
	"$(make_fixture install-first "$FULL_ACTION_REF" "$EXPECTED_SHA" after-install)"
expect_fail "commented TinyGo verification" \
	"$(make_fixture commented-verification "$FULL_ACTION_REF" "$EXPECTED_SHA" commented)"
expect_fail "TinyGo verification with ignored failure" \
	"$(make_fixture ignored-verification "$FULL_ACTION_REF" "$EXPECTED_SHA" ignored)"
expect_fail "TinyGo verification with warning-only failure" \
	"$(make_fixture warned-verification "$FULL_ACTION_REF" "$EXPECTED_SHA" warned)"
expect_fail "TinyGo verification relying on implicit errexit" \
	"$(make_fixture implicit-verification "$FULL_ACTION_REF" "$EXPECTED_SHA" implicit)"
expect_fail "TinyGo verification behind false condition" \
	"$(make_fixture skipped-verification "$FULL_ACTION_REF" "$EXPECTED_SHA" skipped)"
expect_fail "TinyGo artifact replaced after verification" \
	"$(make_fixture replaced-artifact "$FULL_ACTION_REF" "$EXPECTED_SHA" replaced)"

synthetic_artifact="$TMP_ROOT/synthetic-tinygo.deb"
printf 'synthetic TinyGo archive\n' > "$synthetic_artifact"
sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1"
	else
		shasum -a 256 "$1"
	fi | awk '{print $1}'
}

verify_synthetic_checksum() {
	local digest="$1"
	local artifact="$2"
	if command -v sha256sum >/dev/null 2>&1; then
		printf '%s  %s\n' "$digest" "$artifact" | sha256sum --check --strict -
	else
		printf '%s  %s\n' "$digest" "$artifact" | shasum -a 256 --check -
	fi
}

synthetic_sha="$(sha256_file "$synthetic_artifact")"
tests_run=$((tests_run + 1))
if ! verify_synthetic_checksum "$synthetic_sha" "$synthetic_artifact" >/dev/null; then
	printf 'not ok %d - correct synthetic artifact checksum\n' "$tests_run" >&2
	exit 1
fi
printf 'ok %d - correct synthetic artifact checksum\n' "$tests_run"

tests_run=$((tests_run + 1))
if verify_synthetic_checksum "$(printf '%064d' 0)" "$synthetic_artifact" >/dev/null 2>&1; then
	printf 'not ok %d - incorrect synthetic artifact checksum unexpectedly passed\n' "$tests_run" >&2
	exit 1
fi
printf 'ok %d - incorrect synthetic artifact checksum\n' "$tests_run"

if [[ -x /usr/bin/env && -x /usr/bin/sha256sum ]]; then
	# Execute the fixture's actual verification line without inherited errexit.
	for digest in "$(printf '%064d' 0)" "$synthetic_sha"; do
		if [[ "$digest" == "$synthetic_sha" ]]; then
			case_name="correct checksum continues without inherited errexit"
		else
			case_name="wrong checksum stops without inherited errexit"
		fi
		fixture="$(make_fixture plain-bash "$FULL_ACTION_REF" "$digest")"
		verification="$(sed -n '/sha256sum --check --strict/p' "$fixture/.github/workflows/ci-core.yml")"
		verification="${verification//\/tmp\/goframe-tinygo-0.42.0-verified.deb/$synthetic_artifact}"
		printf 'set +e\n%s\nprintf continued > "$continuation"\n' "$verification" > "$TMP_ROOT/verify.sh"
		rm -f "$TMP_ROOT/continued"
		tests_run=$((tests_run + 1))
		if synthetic_artifact="$synthetic_artifact" continuation="$TMP_ROOT/continued" \
			"$BASH" --noprofile --norc "$TMP_ROOT/verify.sh" > "$TMP_ROOT/checksum.log" 2>&1; then
			if [[ "$digest" != "$synthetic_sha" || ! -f "$TMP_ROOT/continued" ]]; then
				printf 'not ok %d - checksum continuation without errexit\n' "$tests_run" >&2
				exit 1
			fi
		else
			if [[ "$digest" == "$synthetic_sha" || -f "$TMP_ROOT/continued" ]]; then
				printf 'not ok %d - checksum failure without errexit\n' "$tests_run" >&2
				exit 1
			fi
		fi
		printf 'ok %d - %s\n' "$tests_run" "$case_name"
	done

	fake_bin="$TMP_ROOT/fake-bin"
	fake_marker="$TMP_ROOT/fake-sha256sum-ran"
	mkdir -p "$fake_bin"
	{
		printf '#!/usr/bin/env bash\n'
		printf ': > "$FAKE_SHA256SUM_MARKER"\n'
		printf 'exit 0\n'
	} > "$fake_bin/sha256sum"
	chmod +x "$fake_bin/sha256sum"

	for digest in "$(printf '%064d' 0)" "$synthetic_sha"; do
		if [[ "$digest" == "$synthetic_sha" ]]; then
			case_name="correct checksum passes under poisoned PATH"
		else
			case_name="wrong checksum fails under poisoned PATH"
		fi
		fixture="$(make_fixture poisoned-path-execution "$FULL_ACTION_REF" "$digest")"
		verification="$(sed -n '/sha256sum --check --strict/p' "$fixture/.github/workflows/ci-core.yml")"
		verification="${verification//\/tmp\/goframe-tinygo-0.42.0-verified.deb/$synthetic_artifact}"
		printf 'set +e\n%s\n' "$verification" > "$TMP_ROOT/path-verify.sh"
		rm -f "$fake_marker"
		tests_run=$((tests_run + 1))
		if FAKE_SHA256SUM_MARKER="$fake_marker" PATH="$fake_bin:$PATH" \
			"$BASH" --noprofile --norc "$TMP_ROOT/path-verify.sh" > "$TMP_ROOT/path-checksum.log" 2>&1; then
			if [[ "$digest" != "$synthetic_sha" ]]; then
				printf 'not ok %d - wrong checksum passed under poisoned PATH\n' "$tests_run" >&2
				exit 1
			fi
		else
			if [[ "$digest" == "$synthetic_sha" ]]; then
				printf 'not ok %d - correct checksum failed under poisoned PATH\n' "$tests_run" >&2
				exit 1
			fi
		fi
		if [[ -e "$fake_marker" ]]; then
			printf 'not ok %d - fake PATH checksum verifier executed\n' "$tests_run" >&2
			exit 1
		fi
		printf 'ok %d - %s\n' "$tests_run" "$case_name"
	done

	checksum_function_marker="$TMP_ROOT/checksum-function-ran"
	printf_function_marker="$TMP_ROOT/printf-function-ran"
	{
		printf 'sha256sum() {\n'
		printf '  : > "$FAKE_SHA256SUM_FUNCTION_MARKER"\n'
		printf '  return 0\n'
		printf '}\n'
		printf 'printf() {\n'
		printf '  : > "$FAKE_PRINTF_FUNCTION_MARKER"\n'
		printf '  builtin printf "attacker\\n"\n'
		printf '}\n'
	} > "$TMP_ROOT/bash-env"

	for digest in "$(printf '%064d' 0)" "$synthetic_sha"; do
		if [[ "$digest" == "$synthetic_sha" ]]; then
			case_name="correct checksum passes with BASH_ENV function overrides"
		else
			case_name="wrong checksum fails with BASH_ENV function overrides"
		fi
		fixture="$(make_fixture bash-env-execution "$FULL_ACTION_REF" "$digest")"
		verification="$(sed -n '/sha256sum --check --strict/p' "$fixture/.github/workflows/ci-core.yml")"
		verification="${verification//\/tmp\/goframe-tinygo-0.42.0-verified.deb/$synthetic_artifact}"
		printf 'set +e\n%s\n' "$verification" > "$TMP_ROOT/bash-env-verify.sh"
		rm -f "$checksum_function_marker" "$printf_function_marker"
		tests_run=$((tests_run + 1))
		if BASH_ENV="$TMP_ROOT/bash-env" \
			FAKE_SHA256SUM_FUNCTION_MARKER="$checksum_function_marker" \
			FAKE_PRINTF_FUNCTION_MARKER="$printf_function_marker" \
			"$BASH" --noprofile --norc "$TMP_ROOT/bash-env-verify.sh" > "$TMP_ROOT/bash-env-checksum.log" 2>&1; then
			if [[ "$digest" != "$synthetic_sha" ]]; then
				printf 'not ok %d - wrong checksum passed with BASH_ENV function overrides\n' "$tests_run" >&2
				exit 1
			fi
		else
			if [[ "$digest" == "$synthetic_sha" ]]; then
				printf 'not ok %d - correct checksum failed with BASH_ENV function overrides\n' "$tests_run" >&2
				exit 1
			fi
		fi
		if [[ -e "$checksum_function_marker" || -e "$printf_function_marker" ]]; then
			printf 'not ok %d - BASH_ENV checksum or printf override executed\n' "$tests_run" >&2
			exit 1
		fi
		printf 'ok %d - %s\n' "$tests_run" "$case_name"
	done
else
	printf 'skip - exact production verifier (/usr/bin/env -i /usr/bin/sha256sum) unavailable; portable SHA-256 controls passed\n'
fi

printf 'ci supply-chain check tests: ok (%d cases)\n' "$tests_run"
