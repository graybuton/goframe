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
			printf "          # printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict -\n"
		elif [[ "$verification_order" == "ignored" ]]; then
			printf "          printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict - || true\n"
		elif [[ "$verification_order" == "skipped" ]]; then
			printf "          false && printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict -\n"
		else
			printf "          printf '%%s  %%s\\\\n' \"\$TINYGO_SHA256\" /tmp/tinygo.deb | sha256sum --check --strict -\n"
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
	if ! output="$($CHECKER "$fixture" 2>&1)"; then
		printf 'not ok %d - %s\n%s\n' "$tests_run" "$name" "$output" >&2
		exit 1
	fi
	printf 'ok %d - %s\n' "$tests_run" "$name"
}

expect_fail() {
	local name="$1"
	local fixture="$2"
	tests_run=$((tests_run + 1))
	if output="$($CHECKER "$fixture" 2>&1)"; then
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
expect_fail "TinyGo verification behind false condition" \
	"$(make_fixture skipped-verification "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" skipped)"
expect_fail "TinyGo artifact replaced after verification" \
	"$(make_fixture replaced-artifact "actions/checkout@$FULL_ACTION_SHA" "$EXPECTED_SHA" replaced)"

synthetic_artifact="$TMP_ROOT/synthetic-tinygo.deb"
printf 'synthetic TinyGo archive\n' > "$synthetic_artifact"
synthetic_sha="$(sha256sum "$synthetic_artifact" | awk '{print $1}')"
tests_run=$((tests_run + 1))
if ! printf '%s  %s\n' "$synthetic_sha" "$synthetic_artifact" | sha256sum --check --strict - >/dev/null; then
	printf 'not ok %d - correct synthetic artifact checksum\n' "$tests_run" >&2
	exit 1
fi
printf 'ok %d - correct synthetic artifact checksum\n' "$tests_run"

tests_run=$((tests_run + 1))
if printf '%064d  %s\n' 0 "$synthetic_artifact" | sha256sum --check --strict - >/dev/null 2>&1; then
	printf 'not ok %d - incorrect synthetic artifact checksum unexpectedly passed\n' "$tests_run" >&2
	exit 1
fi
printf 'ok %d - incorrect synthetic artifact checksum\n' "$tests_run"

printf 'ci supply-chain check tests: ok (%d cases)\n' "$tests_run"
