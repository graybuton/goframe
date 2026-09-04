#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "$ROOT_DIR/scripts/security-analysis.sh"

TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/goframe-security-runner-tests.XXXXXX")"
trap 'rm -rf "$TEST_DIR"' EXIT

write_fake_go() {
	local path="$1"
	local mode="$2"
	local expected_pattern="$3"
	local expected_goos="$4"
	local expected_goarch="$5"
	local expected_cgo_enabled="$6"
	cat >"$path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${1:-}" != list || "\${!#}" != '$expected_pattern' ]]; then
  exit 97
fi
if [[ "\${GOOS:-}" != '$expected_goos' || "\${GOARCH:-}" != '$expected_goarch' || "\${CGO_ENABLED:-}" != '$expected_cgo_enabled' ]]; then
  exit 96
fi
case '$mode' in
  failure)
    exit 1
    ;;
  empty)
    exit 0
    ;;
  host)
    printf 'example.test/project/one\\t%s\\n' '$TEST_DIR/repository/one'
    printf 'example.test/project/two\\t%s\\n' '$TEST_DIR/repository/two'
    ;;
  browser)
    printf 'example.test/project/browser\\t%s\\n' '$TEST_DIR/repository/browser'
    ;;
esac
EOF
	chmod +x "$path"
}

write_fake_gosec() {
	local path="$1"
	local mode="$2"
	local expected_goos="$3"
	local expected_goarch="$4"
	local expected_cgo_enabled="$5"
	cat >"$path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${GOOS:-}" != '$expected_goos' || "\${GOARCH:-}" != '$expected_goarch' || "\${CGO_ENABLED:-}" != '$expected_cgo_enabled' ]]; then
  exit 96
fi
if [[ '$mode' == failure ]]; then
  exit 1
fi
if [[ '$mode' == no-report ]]; then
  exit 0
fi
report=''
for argument in "\$@"; do
  case "\$argument" in
    -out=*) report="\${argument#-out=}" ;;
  esac
done
if [[ -z "\$report" ]]; then
  exit 95
fi
printf '%s\\n' '{"Golang errors":{},"Issues":[],"Stats":{"files":1,"lines":1,"nosec":0,"found":0},"GosecVersion":"dev"}' >"\$report"
EOF
	chmod +x "$path"
}

mkdir -p \
	"$TEST_DIR/repository/one" \
	"$TEST_DIR/repository/two" \
	"$TEST_DIR/repository/browser"

write_fake_go "$TEST_DIR/go-host" host ./... linux amd64 1
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/host.txt" \
	"$TEST_DIR/go-host" \
	./...
if ((${#GOSEC_PACKAGE_DIRS[@]} != 2)); then
	echo 'runner test: host package coverage count mismatch' >&2
	exit 1
fi
host_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")

write_fake_go "$TEST_DIR/go-browser" browser ./pkg/goframe js wasm 0
GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/browser.txt" \
	"$TEST_DIR/go-browser" \
	./pkg/goframe
if ((${#GOSEC_PACKAGE_DIRS[@]} != 1)) || [[ "${GOSEC_PACKAGE_IMPORTS[0]}" != example.test/project/browser ]]; then
	echo 'runner test: browser package scope was not isolated' >&2
	exit 1
fi
browser_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")
if ((${#host_package_dirs[@]} != 2)) || [[ "${host_package_dirs[0]}" == "${GOSEC_PACKAGE_DIRS[0]}" ]]; then
	echo 'runner test: host package coverage was contaminated by browser coverage' >&2
	exit 1
fi

write_fake_go "$TEST_DIR/go-browser-failure" failure ./pkg/goframe js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/browser-failure.txt" \
	"$TEST_DIR/go-browser-failure" \
	./pkg/goframe; then
	echo 'runner test: failed browser package enumeration was accepted' >&2
	exit 1
fi

write_fake_go "$TEST_DIR/go-browser-empty" empty ./pkg/goframe js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/browser-empty.txt" \
	"$TEST_DIR/go-browser-empty" \
	./pkg/goframe; then
	echo 'runner test: empty browser package enumeration was accepted' >&2
	exit 1
fi

write_fake_gosec "$TEST_DIR/gosec-host" success linux amd64 1
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 run_gosec_analysis \
	"$TEST_DIR/gosec-host.json" \
	"$TEST_DIR/gosec-host" \
	"${host_package_dirs[@]}"

write_fake_gosec "$TEST_DIR/gosec-browser" success js wasm 0
GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
	"$TEST_DIR/gosec-browser.json" \
	"$TEST_DIR/gosec-browser" \
	"${browser_package_dirs[@]}"

write_fake_gosec "$TEST_DIR/gosec-browser-failure" failure js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
	"$TEST_DIR/gosec-browser-failure.json" \
	"$TEST_DIR/gosec-browser-failure" \
	"${browser_package_dirs[@]}"; then
	echo 'runner test: failed browser gosec execution was accepted' >&2
	exit 1
fi

write_fake_gosec "$TEST_DIR/gosec-browser-no-report" no-report js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
	"$TEST_DIR/gosec-browser-no-report.json" \
	"$TEST_DIR/gosec-browser-no-report" \
	"${browser_package_dirs[@]}"; then
	echo 'runner test: missing browser gosec report was accepted' >&2
	exit 1
fi

echo 'security analysis runner tests: ok'
