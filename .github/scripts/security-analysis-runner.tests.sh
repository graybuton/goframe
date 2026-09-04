#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "$ROOT_DIR/scripts/security-analysis.sh"

TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/goframe-security-runner-tests.XXXXXX")"
trap 'rm -rf "$TEST_DIR"' EXIT

write_fake_go() {
	local path="$1"
	local mode="$2"
	cat >"$path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${1:-}" != list ]]; then
  exit 97
fi
case '$mode' in
  failure)
    exit 1
    ;;
  empty)
    exit 0
    ;;
  valid)
    printf 'example.test/project/one\\t%s\\n' '$TEST_DIR/repository/one'
    printf 'example.test/project/two\\t%s\\n' '$TEST_DIR/repository/two'
    ;;
esac
EOF
	chmod +x "$path"
}

mkdir -p "$TEST_DIR/repository/one" "$TEST_DIR/repository/two"

write_fake_go "$TEST_DIR/go-valid" valid
enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/valid.txt" \
	"$TEST_DIR/go-valid"
if ((${#GOSEC_PACKAGE_DIRS[@]} != 2)); then
	echo 'runner test: valid package coverage count mismatch' >&2
	exit 1
fi

write_fake_go "$TEST_DIR/go-failure" failure
if enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/failure.txt" \
	"$TEST_DIR/go-failure"; then
	echo 'runner test: failed package enumeration was accepted' >&2
	exit 1
fi

write_fake_go "$TEST_DIR/go-empty" empty
if enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/empty.txt" \
	"$TEST_DIR/go-empty"; then
	echo 'runner test: empty package enumeration was accepted' >&2
	exit 1
fi

echo 'security analysis runner tests: ok'
