#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHECKER="$ROOT_DIR/scripts/ci-supply-chain-check.sh"
TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT

EXPECTED_SHA="2082c4762fea6d5cc4cd1f4a243eaacf07b12f576717d4c6b74828bd163cb563"
FULL_ACTION_SHA="3d3c42e5aac5ba805825da76410c181273ba90b1"
tests_run=0

write_tinygo_workflow() {
	local path="$1"
	local action_ref="$2"
	local checksum="$3"
	local verification_order="$4"
	local include_checksum="$5"

	mkdir -p "$(dirname "$path")"
	{
		printf 'name: fixture\n'
		printf 'env:\n'
		printf '  TINYGO_VERSION: 0.42.0\n'
		if [[ "$include_checksum" == "yes" ]]; then
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
		printf '          curl -fsSL -o /tmp/tinygo.deb "https://github.com/tinygo-org/tinygo/releases/download/v${TINYGO_VERSION}/tinygo_${TINYGO_VERSION}_amd64.deb"\n'
		if [[ "$verification_order" == "after-install" ]]; then
			printf '          sudo apt-get install -y /tmp/tinygo.deb\n'
		fi
		if [[ "$verification_order" == "commented" ]]; then
			printf "          # printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || exit 1\n"
		elif [[ "$verification_order" == "ignored" ]]; then
			printf "          printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || true\n"
		elif [[ "$verification_order" == "warned" ]]; then
			printf "          printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || echo warning\n"
		elif [[ "$verification_order" == "skipped" ]]; then
			printf "          false && printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || exit 1\n"
		elif [[ "$verification_order" == "implicit" ]]; then
			printf "          printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict -\n"
		else
			printf "          printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || exit 1\n"
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
	local fixture="$TMP_ROOT/$name"

	write_tinygo_workflow "$fixture/.github/workflows/ci-core.yml" \
		"$action_ref" "$checksum" "$verification_order" "$include_checksum"
	write_tinygo_workflow "$fixture/.github/workflows/ci-browser-smoke.yml" \
		"" "$checksum" "before-install" "$include_checksum"
	write_tinygo_workflow "$fixture/.github/workflows/ci-wasm-size.yml" \
		"" "$checksum" "before-install" "$include_checksum"
	printf '%s' "$fixture"
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

expect_pass "full 40-character remote Action SHA" \
	"$(make_fixture full-sha "actions/checkout@$FULL_ACTION_SHA")"
expect_pass "local Action reference" \
	"$(make_fixture local-action './.github/actions/local')"
expect_fail "mutable major Action tag" \
	"$(make_fixture mutable-tag 'actions/checkout@v7')"
expect_fail "mutable branch Action ref" \
	"$(make_fixture mutable-branch 'actions/checkout@main')"
expect_fail "short Action SHA" \
	"$(make_fixture short-sha 'actions/checkout@3d3c42e5aac5')"

expect_pass "external reusable workflow with full SHA" \
	"$(make_fixture reusable-workflow "owner/repo/.github/workflows/build.yml@$FULL_ACTION_SHA")"

for action_form in \
	'"uses": actions/checkout@v7' \
	"'uses': actions/checkout@v7" \
	'{ uses: actions/checkout@v7 }' \
	"\"uses\": actions/checkout@$FULL_ACTION_SHA" \
	"{ uses: actions/checkout@$FULL_ACTION_SHA }" \
	'"u\u0073es": actions/checkout@v7' \
	'? uses'; do
	fixture="$(make_fixture alternate-action "actions/checkout@$FULL_ACTION_SHA")"
	printf 'jobs:\n  alternate:\n    steps:\n      - %s\n' "$action_form" \
		> "$fixture/.github/workflows/alternate.yml"
	expect_fail "unsupported Action key: $action_form" "$fixture"
done

fixture="$(make_fixture commented-action "actions/checkout@$FULL_ACTION_SHA")"
printf '# - "uses": actions/checkout@v7\n# - { uses: actions/checkout@v7 }\n' \
	> "$fixture/.github/workflows/comments.yml"
expect_pass "commented alternate Action keys" "$fixture"

fixture="$(make_fixture scalar-content "actions/checkout@$FULL_ACTION_SHA")"
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

fixture="$(make_fixture 'space in repository path' "actions/checkout@$FULL_ACTION_SHA")"
expect_pass "repository path containing spaces" "$fixture"
printf 'uses: actions/checkout@v7\n' > "$fixture/.github/workflows/"$'line\nbreak.yml'
expect_fail "workflow filename containing a newline" "$fixture"

extra_download='curl -fsSL -o /tmp/extra.deb "https://github.com/tinygo-org/tinygo/releases/download/v0.42.0/tinygo_0.42.0_amd64.deb"'
for placement in before after same-line canonical hash-prefix unexpected; do
	fixture="$(make_fixture extra-download "actions/checkout@$FULL_ACTION_SHA")"
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

expect_fail "missing TinyGo checksum" \
	"$(make_fixture missing-checksum "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" before-install no)"
expect_fail "wrong TinyGo checksum" \
	"$(make_fixture wrong-checksum "actions/checkout@$FULL_ACTION_SHA" "$(printf '0%.0s' {1..64})")"
expect_pass "verified TinyGo install contract" \
	"$(make_fixture verified-tinygo "actions/checkout@$FULL_ACTION_SHA")"
expect_fail "TinyGo install before verification" \
	"$(make_fixture install-first "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" after-install)"
expect_fail "commented TinyGo verification" \
	"$(make_fixture commented-verification "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" commented)"
expect_fail "TinyGo verification with ignored failure" \
	"$(make_fixture ignored-verification "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" ignored)"
expect_fail "TinyGo verification with warning-only failure" \
	"$(make_fixture warned-verification "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" warned)"
expect_fail "TinyGo verification relying on implicit errexit" \
	"$(make_fixture implicit-verification "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" implicit)"
expect_fail "TinyGo verification behind false condition" \
	"$(make_fixture skipped-verification "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" skipped)"
expect_fail "TinyGo artifact replaced after verification" \
	"$(make_fixture replaced-artifact "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" replaced)"

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
	fixture="$(make_fixture plain-bash "actions/checkout@$FULL_ACTION_SHA")"
	verification="$(sed -n '/sha256sum --check --strict/p' "$fixture/.github/workflows/ci-core.yml")"
	verification="${verification/\/tmp\/tinygo.deb/\"\$synthetic_artifact\"}"
	printf 'set +e\n%s\nprintf continued > "$continuation"\n' "$verification" > "$TMP_ROOT/verify.sh"
	for digest in "$(printf '%064d' 0)" "$synthetic_sha"; do
		rm -f "$TMP_ROOT/continued"
		tests_run=$((tests_run + 1))
		if TINYGO_SHA256="$digest" synthetic_artifact="$synthetic_artifact" continuation="$TMP_ROOT/continued" \
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
