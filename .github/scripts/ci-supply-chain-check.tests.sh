#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHECKER="$ROOT_DIR/scripts/ci-supply-chain-check.sh"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

EXPECTED_SHA="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
TINYGO_RELEASE_REPOSITORY="tinygo-org/tinygo"
TINYGO_RELEASE_NAMESPACE="${TINYGO_RELEASE_REPOSITORY}/releases/download/"
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
	local download_command verify_input

	mkdir -p "$(dirname "$path")"
	case "$trust_root_mode" in
		literal|variable-digest)
			download_command="curl -fsSL -o /tmp/tinygo.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v0.42.0/tinygo_0.42.0_amd64.deb\""
			;;
		variable-url|variable-both)
			download_command="curl -fsSL -o /tmp/tinygo.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v\${TINYGO_VERSION}/tinygo_\${TINYGO_VERSION}_amd64.deb\""
			;;
		*)
			printf 'unsupported trust-root mode: %s\n' "$trust_root_mode" >&2
			exit 1
			;;
	esac
	case "$trust_root_mode" in
		literal|variable-url) verify_input="'$checksum'" ;;
		variable-digest|variable-both) verify_input='"$TINYGO_SHA256"' ;;
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
		printf '    steps:\n'
		if [[ -n "$action_ref" ]]; then
			printf '      - uses: %s\n' "$action_ref"
		fi
		printf '      # uses: actions/commented-out@v1\n'
		printf '      - run: |\n'
		printf '          %s\n' "$download_command"
		if [[ "$verification_order" == "after-install" ]]; then
			printf '          sudo apt-get install -y /tmp/tinygo.deb\n'
		fi
		if [[ "$include_checksum" == "yes" ]]; then
			if [[ "$verification_order" == "commented" ]]; then
				printf "          # printf '%%s  %%s\\\\n' %s /tmp/tinygo.deb | sha256sum --check --strict - || exit 1\n" "$verify_input"
			elif [[ "$verification_order" == "ignored" ]]; then
				printf "          printf '%%s  %%s\\\\n' %s /tmp/tinygo.deb | sha256sum --check --strict - || true\n" "$verify_input"
			elif [[ "$verification_order" == "warned" ]]; then
				printf "          printf '%%s  %%s\\\\n' %s /tmp/tinygo.deb | sha256sum --check --strict - || echo warning\n" "$verify_input"
			elif [[ "$verification_order" == "skipped" ]]; then
				printf "          false && printf '%%s  %%s\\\\n' %s /tmp/tinygo.deb | sha256sum --check --strict - || exit 1\n" "$verify_input"
			elif [[ "$verification_order" == "implicit" ]]; then
				printf "          printf '%%s  %%s\\\\n' %s /tmp/tinygo.deb | sha256sum --check --strict -\n" "$verify_input"
			else
				printf "          printf '%%s  %%s\\\\n' %s /tmp/tinygo.deb | sha256sum --check --strict - || exit 1\n" "$verify_input"
			fi
		fi
		if [[ "$verification_order" == "replaced" ]]; then
			printf '          cp /tmp/replacement.deb /tmp/tinygo.deb\n'
		fi
		if [[ "$verification_order" != "after-install" ]]; then
			printf '          sudo apt-get install -y /tmp/tinygo.deb\n'
		fi
		printf '          tinygo version\n'
	} > "$path"
}

make_fixture() {
	local name="$1"
	local action_ref="$2"
	local checksum="${3:-$EXPECTED_SHA}"
	local verification_order="${4:-before-install}"
	local include_checksum="${5:-yes}"
	local trust_root_mode="${6:-literal}"
	local fixture="$TMP_ROOT/$name"

	write_tinygo_workflow "$fixture/.github/workflows/ci-core.yml" \
		"$action_ref" "$checksum" "$verification_order" "$include_checksum" "$trust_root_mode"
	write_tinygo_workflow "$fixture/.github/workflows/ci-browser-smoke.yml" \
		"" "$checksum" "before-install" "$include_checksum" "$trust_root_mode"
	write_tinygo_workflow "$fixture/.github/workflows/ci-wasm-size.yml" \
		"" "$checksum" "before-install" "$include_checksum" "$trust_root_mode"
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
		/      - run: \|/ && !inserted {
			print "      - run: |"
			print "          echo '\''TINYGO_VERSION=attacker-value'\'' >> \"$GITHUB_ENV\""
			print "          echo '\''TINYGO_SHA256=attacker-digest'\'' >> \"$GITHUB_ENV\""
			inserted = 1
		}
		{ print }
	' "$file" > "$TMP_ROOT/runtime-override.yml"
	mv "$TMP_ROOT/runtime-override.yml" "$file"
}

write_repository_file() {
	local fixture="$1"
	local path="$2"
	local content="$3"
	local target="$fixture/$path"

	mkdir -p "$(dirname "$target")"
	printf '%s\n' "$content" > "$target"
}

add_workflow_script_invocation() {
	local file="$1"
	local command="$2"

	printf '\n  delegated-script:\n    steps:\n      - run: %s\n' "$command" >> "$file"
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

printf '%s\n' 'jobs:' '  named:' '    steps:' '      - name: |' '          multiline name' \
	'        uses: actions/checkout@v7' > "$fixture/.github/workflows/scalars.yml"
expect_fail "Action following a block-scalar step name" "$fixture"

printf '%s\n' 'jobs:' '  named:' '    steps:' '      -   name: |' '            multiline name' \
	'          uses: actions/checkout@v7' > "$fixture/.github/workflows/scalars.yml"
expect_fail "Action following a spaced block-scalar step name" "$fixture"

fixture="$(make_fixture 'space in repository path' "$FULL_ACTION_REF")"
expect_pass "repository path containing spaces" "$fixture"
printf 'uses: actions/checkout@v7\n' > "$fixture/.github/workflows/"$'line\nbreak.yml'
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

extra_download="curl -fsSL -o /tmp/extra.deb \"https://github.com/${TINYGO_RELEASE_NAMESPACE}v0.42.0/tinygo_0.42.0_amd64.deb\""
for placement in before after same-line canonical hash-prefix unexpected; do
	fixture="$(make_fixture extra-download "$FULL_ACTION_REF")"
	file="$fixture/.github/workflows/ci-core.yml"
	case "$placement" in
		before)
			awk -v command="$extra_download" '
				/      - run: \|/ { print "      - run: " command }
				{ print }
			' "$file" > "$TMP_ROOT/extra.yml"
			mv "$TMP_ROOT/extra.yml" "$file"
			;;
		after) printf '      - run: %s\n' "$extra_download" >> "$file" ;;
		same-line) printf '      - run: %s; %s\n' "$extra_download" "$extra_download" >> "$file" ;;
		canonical)
			command="$(sed -n '/          curl -fsSL/p' "$file")"
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

if command -v sha256sum >/dev/null 2>&1; then
	# Execute the fixture's actual verification line without inherited errexit.
	for digest in "$(printf '%064d' 0)" "$synthetic_sha"; do
		fixture="$(make_fixture plain-bash "$FULL_ACTION_REF" "$digest")"
		verification="$(sed -n '/sha256sum --check --strict/p' "$fixture/.github/workflows/ci-core.yml")"
		verification="${verification/\/tmp\/tinygo.deb/\"\$synthetic_artifact\"}"
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
		printf 'ok %d - explicit checksum status and continuation without errexit\n' "$tests_run"
	done
else
	printf 'skip - GNU workflow checksum execution; portable SHA-256 controls passed\n'
fi

printf 'ci supply-chain check tests: ok (%d cases)\n' "$tests_run"
